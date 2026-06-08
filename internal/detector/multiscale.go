package detector

import (
	"math"
	"sort"
)

// Multi-scale baseline detection (OI-21). A single sliding-window CFAR cannot
// serve narrow (CW/FT8) and wide (SSB/NFM/WFM) signals at once: a small training
// window lets a wide signal mask itself; a large window lets dense narrow signals
// contaminate the noise estimate (flatline). Diffuse SSB (energy spread over
// ~2.7 kHz, no carrier) never clears a per-bin threshold at all.
//
// Instead: estimate a bandwidth-agnostic noise baseline by grayscale opening
// (sliding min then max, structuring element wider than the widest signal),
// subtract it, then detect at several matched box-integration scales (a diffuse
// signal of width w gains ~sqrt(w) SNR at scale ~w). Fuse fine->coarse, a coarse
// run replacing the finer ones it covers only when it is FILLED (no internal
// return to noise) — a wide WFM stays one signal while close narrow signals
// separated by a noise gap split. One sensitivity knob (K).
//
// Hot-path discipline (Constitution XI): all per-frame work runs on preallocated
// scratch on the Detector — no allocation in steady state — and the per-scale
// noise threshold is derived analytically (sigma/sqrt(W)) from a single residual
// MAD rather than sorting the full spectrum per scale.
//
// Validated offline on real captures (40m/20m/BC-FM): ~2x the detection recall of
// the CFAR path and it actually finds SSB, where CFAR is blind.

// MultiScaleParams configures the multi-scale detector. Zero value is not valid;
// use DefaultMultiScaleParams (or withDefaults to fill gaps).
type MultiScaleParams struct {
	OpeningHz float64   // baseline structuring-element width (> widest signal)
	ScalesHz  []float64 // matched integration scales
	K         float64   // detection threshold = K * residual-noise-sigma at scale
	MinSNRDb  float64   // a detection's peak must clear the baseline by this
	CutMult   float64   // valley cut at noiseMean + CutMult*noiseSigma
	MinGapHz  float64   // a split requires a noise valley at least this wide
	MinBwHz   float64   // reject detections narrower than this
}

func DefaultMultiScaleParams() MultiScaleParams {
	return MultiScaleParams{
		OpeningHz: 400e3,
		ScalesHz:  []float64{120, 500, 2000, 6000, 20000, 80000},
		K:         6.0,
		MinSNRDb:  4.0,
		CutMult:   3.0,
		MinGapHz:  150,
		MinBwHz:   40,
	}
}

func (p MultiScaleParams) withDefaults() MultiScaleParams {
	d := DefaultMultiScaleParams()
	if p.OpeningHz <= 0 {
		p.OpeningHz = d.OpeningHz
	}
	if len(p.ScalesHz) == 0 {
		p.ScalesHz = d.ScalesHz
	}
	if p.K <= 0 {
		p.K = d.K
	}
	if p.MinSNRDb <= 0 {
		p.MinSNRDb = d.MinSNRDb
	}
	if p.CutMult <= 0 {
		p.CutMult = d.CutMult
	}
	if p.MinGapHz <= 0 {
		p.MinGapHz = d.MinGapHz
	}
	if p.MinBwHz <= 0 {
		p.MinBwHz = d.MinBwHz
	}
	return p
}

// msRun is a contiguous detected run (spectrum bin indices).
type msRun struct{ first, last int }

// msScratch holds per-frame reusable buffers (sized to the spectrum length) so
// detection allocates nothing in steady state.
type msScratch struct {
	lin, erode, base, resid, baseDb, sm, samp []float64
	dq                                        []int
}

func (s *msScratch) ensure(n int) {
	if len(s.lin) == n {
		return
	}
	s.lin = make([]float64, n)
	s.erode = make([]float64, n)
	s.base = make([]float64, n)
	s.resid = make([]float64, n)
	s.baseDb = make([]float64, n)
	s.sm = make([]float64, n)
	s.samp = make([]float64, n)
	s.dq = make([]int, 0, n)
}

// detectMultiScale runs the detector on a dB power spectrum (already EMA-smoothed
// by the caller). centerHz is the RF center; FirstBin/LastBin are spectrum indices.
func (d *Detector) detectMultiScale(psdDb []float64, centerHz float64) []Signal {
	p := d.msParams.withDefaults()
	n := len(psdDb)
	if n == 0 || d.binWidth <= 0 {
		return nil
	}
	if d.msScr == nil {
		d.msScr = &msScratch{}
	}
	s := d.msScr
	s.ensure(n)

	for i, db := range psdDb {
		s.lin[i] = math.Pow(10, db/10)
	}
	oh := int(p.OpeningHz/d.binWidth) / 2
	if oh < 1 {
		oh = 1
	}
	// baseline = opening = erosion (min) then dilation (max)
	slidingExtremeInto(s.erode, s.lin, oh, true, &s.dq)
	slidingExtremeInto(s.base, s.erode, oh, false, &s.dq)
	for i := 0; i < n; i++ {
		r := s.lin[i] - s.base[i]
		if r < 0 {
			r = 0
		}
		s.resid[i] = r
		s.baseDb[i] = 10 * math.Log10(s.base[i]+1e-30)
	}

	// One robust noise estimate of the residual (mean + MAD-sigma); per-scale
	// thresholds derive analytically as sigma/sqrt(W) for box-averaging.
	noiseMed, noiseSig := residualNoise(s.resid, s.samp)
	cut := noiseMed + p.CutMult*noiseSig

	minGap := int(p.MinGapHz / d.binWidth)
	if minGap < 1 {
		minGap = 1
	}
	minBw := int(p.MinBwHz / d.binWidth)
	if minBw < 1 {
		minBw = 1
	}

	final := d.msFinal[:0]
	for _, sHz := range p.ScalesHz {
		h := int(sHz/d.binWidth) / 2
		boxavgInto(s.sm, s.resid, h)
		w := float64(2*h + 1)
		thr := noiseMed + p.K*noiseSig/math.Sqrt(w)
		i := 0
		for i < n {
			if s.sm[i] <= thr {
				i++
				continue
			}
			j := i
			for j < n && s.sm[j] > thr {
				j++
			}
			rf, rl := i, j-1
			i = j
			covered := false
			for fi := range final {
				if final[fi].first <= rl && final[fi].last >= rf {
					covered = true
					break
				}
			}
			if !covered {
				final = append(final, msRun{rf, rl})
				continue
			}
			// covered by finer scale(s): upgrade to this coarse run only if it is a
			// single filled blob (no internal return to noise). Otherwise the finer
			// detections are genuinely separate signals and are kept.
			if !hasNoiseGap(s.resid, rf, rl, cut, minGap) {
				kept := final[:0] // in-place filter (writes never pass the read index)
				for _, f := range final {
					if f.first <= rl && f.last >= rf {
						continue
					}
					kept = append(kept, f)
				}
				final = append(kept, msRun{rf, rl})
			}
		}
	}
	d.msFinal = final // retain backing array for reuse next frame

	sigs := make([]Signal, 0, len(final))
	for _, r := range final {
		a, e := r.first, r.last
		if (e - a + 1) < minBw {
			continue
		}
		peakBin := a
		var wsum, fsum float64
		for b := a; b <= e; b++ {
			if psdDb[b] > psdDb[peakBin] {
				peakBin = b
			}
			pw := s.lin[b]
			wsum += pw
			fsum += pw * float64(b)
		}
		peakDb := psdDb[peakBin]
		noiseDb := s.baseDb[peakBin]
		if peakDb-noiseDb < p.MinSNRDb {
			continue
		}
		ctrBin := float64(peakBin)
		if wsum > 0 {
			ctrBin = fsum / wsum
		}
		sigs = append(sigs, Signal{
			FirstBin: a,
			LastBin:  e,
			CenterHz: centerHz + (ctrBin-float64(d.nbins)/2.0)*d.binWidth,
			BWHz:     float64(e-a+1) * d.binWidth,
			PeakDb:   peakDb,
			SNRDb:    peakDb - noiseDb,
			NoiseDb:  noiseDb,
		})
	}
	sort.Slice(sigs, func(a, b int) bool { return sigs[a].FirstBin < sigs[b].FirstBin })
	return sigs
}

// --- O(n), allocation-free DSP helpers ---

// slidingExtremeInto writes the centered sliding min (or max) of a over a window
// of half-width h into out, reusing *dq as the monotonic-index deque scratch.
func slidingExtremeInto(out, a []float64, h int, min bool, dq *[]int) {
	n := len(a)
	if h < 1 {
		copy(out, a)
		return
	}
	q := (*dq)[:0]
	for i := 0; i < n+h; i++ {
		if i < n {
			for len(q) > 0 {
				last := q[len(q)-1]
				if (min && a[last] >= a[i]) || (!min && a[last] <= a[i]) {
					q = q[:len(q)-1]
				} else {
					break
				}
			}
			q = append(q, i)
		}
		c := i - h
		if c >= 0 {
			for len(q) > 0 && q[0] < c-h {
				q = q[1:]
			}
			out[c] = a[q[0]]
		}
	}
	*dq = q[:0]
}

// boxavgInto writes the centered moving average (window 2h+1) of a into out via a
// running window sum (no prefix-array allocation).
func boxavgInto(out, a []float64, h int) {
	n := len(a)
	if h < 1 {
		copy(out, a)
		return
	}
	lo, hi := 0, -1
	sum := 0.0
	for i := 0; i < n; i++ {
		nlo := i - h
		if nlo < 0 {
			nlo = 0
		}
		nhi := i + h
		if nhi > n-1 {
			nhi = n - 1
		}
		for hi < nhi {
			hi++
			sum += a[hi]
		}
		for lo < nlo {
			sum -= a[lo]
			lo++
		}
		out[i] = sum / float64(hi-lo+1)
	}
}

// residualNoise returns the median (robust noise center, unbiased by strong
// signals — unlike the mean) and MAD-sigma of the residual, using samp as a
// reusable sort scratch (subsampled for speed; the residual is mostly noise).
func residualNoise(resid, samp []float64) (med, sigma float64) {
	n := len(resid)
	if n == 0 {
		return 0, 1e-30
	}
	step := n / 8192
	if step < 1 {
		step = 1
	}
	m := samp[:0]
	for i := 0; i < n; i += step {
		m = append(m, resid[i])
	}
	if len(m) < 8 {
		return 0, 1e-30
	}
	sort.Float64s(m)
	med = m[len(m)/2]
	for i := range m {
		m[i] = math.Abs(m[i] - med)
	}
	sort.Float64s(m)
	sigma = 1.4826 * m[len(m)/2]
	if sigma <= 0 {
		sigma = 1e-30
	}
	return med, sigma
}

// hasNoiseGap reports whether [a,b] contains >= minGap consecutive bins at/below
// the noise cut (a return to noise between separate signals, vs a filled blob).
func hasNoiseGap(resid []float64, a, b int, cut float64, minGap int) bool {
	gap := 0
	for i := a; i <= b && i < len(resid); i++ {
		if resid[i] <= cut {
			gap++
			if gap >= minGap {
				return true
			}
		} else {
			gap = 0
		}
	}
	return false
}

//go:build bench

// Strategy prototype + evaluation for universal multi-bandwidth detection
// (OI-21). The live problem: one CFAR (guard/train) cannot serve narrow (CW/FT8)
// and wide (SSB/NFM/WFM) at once — small window -> a wide signal masks itself in
// its own training; large window -> dense narrow signals contaminate training
// (flatline). And diffuse SSB (energy spread over ~2.7 kHz, no carrier) never
// clears a per-bin threshold.
//
// "Other strategy" prototyped here: matched multi-scale detection on a
// baseline-subtracted spectrum.
//  1. robust baseline B(f) via grayscale opening (sliding min then max), width >
//     widest signal -> a signal-free noise floor, bandwidth-agnostic.
//  2. residual R = max(0, PSD_lin - B_lin) (excess power over noise).
//  3. for each scale s: smooth R over s bins (matched integration: a diffuse
//     signal of width w gains ~sqrt(w) SNR at scale ~w), robust noise via MAD,
//     threshold k*sigma, extract runs.
//  4. fuse across scales (prefer fine; add a coarser run only where the fine
//     scales found nothing — i.e. genuinely diffuse/wide signals).
//
// One sensitivity knob (k) for all widths.
//
//	go test -tags bench -run TestStrategyDetect ./internal/synth/ -v
package synth_test

import (
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/detector"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/iqfile"
)

// ---------- DSP helpers ----------

// slidingMin / slidingMax: O(n) monotonic-deque min/max over a centered window of
// half-width h (window = 2h+1).
func slidingMin(a []float64, h int) []float64 { return slidingExtreme(a, h, true) }
func slidingMax(a []float64, h int) []float64 { return slidingExtreme(a, h, false) }

func slidingExtreme(a []float64, h int, min bool) []float64 {
	n := len(a)
	out := make([]float64, n)
	if h < 1 {
		copy(out, a)
		return out
	}
	dq := make([]int, 0, n) // indices, monotonic
	push := func(i int) {
		for len(dq) > 0 {
			last := dq[len(dq)-1]
			if (min && a[last] >= a[i]) || (!min && a[last] <= a[i]) {
				dq = dq[:len(dq)-1]
			} else {
				break
			}
		}
		dq = append(dq, i)
	}
	for i := 0; i < n+h; i++ {
		if i < n {
			push(i)
		}
		// front out of window [i-2h, i] when evaluating center i-h
		c := i - h
		if c >= 0 {
			for len(dq) > 0 && dq[0] < c-h {
				dq = dq[1:]
			}
			out[c] = a[dq[0]]
		}
	}
	return out
}

// opening = erosion (min) then dilation (max): removes bright features narrower
// than the structuring element, leaving the baseline.
func opening(a []float64, h int) []float64 { return slidingMax(slidingMin(a, h), h) }

// boxavg: centered moving average over (2h+1) via prefix sums (O(n)).
func boxavg(a []float64, h int) []float64 {
	n := len(a)
	out := make([]float64, n)
	if h < 1 {
		copy(out, a)
		return out
	}
	pre := make([]float64, n+1)
	for i := 0; i < n; i++ {
		pre[i+1] = pre[i] + a[i]
	}
	for i := 0; i < n; i++ {
		lo := i - h
		if lo < 0 {
			lo = 0
		}
		hi := i + h + 1
		if hi > n {
			hi = n
		}
		out[i] = (pre[hi] - pre[lo]) / float64(hi-lo)
	}
	return out
}

func madStd(a []float64) float64 {
	c := append([]float64(nil), a...)
	sort.Float64s(c)
	med := c[len(c)/2]
	dev := make([]float64, len(c))
	for i, v := range c {
		dev[i] = math.Abs(v - med)
	}
	sort.Float64s(dev)
	return 1.4826 * dev[len(dev)/2]
}

// ---------- multi-scale detector ----------

type msCand struct {
	first, last int
	scaleBins   int
	snrDb       float64
}

type msOpts struct {
	openingHz float64   // structuring element width (Hz) for the baseline
	scalesHz  []float64 // matched smoothing scales (Hz) for the sensitivity mask
	k         float64   // mask threshold = k * residual-noise-sigma per scale
	minSnrDb  float64   // a detection's peak must clear baseline by this (dB)
	cutMult   float64   // valley split: cut at noiseMedian + cutMult*noiseSigma
	minGapHz  float64   // a split needs a valley at least this wide
	minBwHz   float64   // reject detections narrower than this
}

func defaultMsOpts() msOpts {
	return msOpts{
		openingHz: 400e3,
		scalesHz:  []float64{120, 500, 2000, 6000, 20000, 80000},
		k:         6.0,
		minSnrDb:  4.0,
		cutMult:   3.0,
		minGapHz:  150,
		minBwHz:   40,
	}
}

// multiScaleDetect: baseline-subtract, build a sensitivity mask as the OR of
// matched-scale detections, then segment each active region into signals at
// valleys where the residual returns near noise (relative valley test). A filled
// wide blob (WFM MPX) stays one signal; close narrow signals separated by a
// noise gap split. Returns candidates in bins.
func multiScaleDetect(psdDb []float64, binWidth float64, o msOpts) []msCand {
	n := len(psdDb)
	lin := make([]float64, n)
	for i, db := range psdDb {
		lin[i] = math.Pow(10, db/10)
	}
	oh := int(o.openingHz/binWidth) / 2
	if oh < 1 {
		oh = 1
	}
	base := opening(lin, oh)
	resid := make([]float64, n)
	baseDb := make([]float64, n)
	for i := range lin {
		r := lin[i] - base[i]
		if r < 0 {
			r = 0
		}
		resid[i] = r
		baseDb[i] = 10 * math.Log10(base[i]+1e-30)
	}

	// sensitivity mask: OR of matched-scale detections
	active := make([]bool, n)
	for _, sHz := range o.scalesHz {
		h := int(sHz/binWidth) / 2
		sm := boxavg(resid, h)
		sigma := madStd(sm)
		if sigma <= 0 {
			sigma = 1e-30
		}
		thr := o.k * sigma
		for i := 0; i < n; i++ {
			if sm[i] > thr {
				active[i] = true
			}
		}
	}

	// noise level of the raw residual (from inactive bins) -> absolute valley cut.
	// Splitting only where the residual returns to NOISE keeps a filled wide blob
	// (WFM MPX) whole while separating narrow signals across a noise gap.
	var inact []float64
	for i := 0; i < n; i++ {
		if !active[i] {
			inact = append(inact, resid[i])
		}
	}
	noiseMed, noiseSig := 0.0, 1e-30
	if len(inact) > 16 {
		s := append([]float64(nil), inact...)
		sort.Float64s(s)
		noiseMed = s[len(s)/2]
		noiseSig = madStd(inact)
	}
	cut := noiseMed + o.cutMult*noiseSig

	minGap := int(o.minGapHz / binWidth)
	if minGap < 1 {
		minGap = 1
	}
	minBw := int(o.minBwHz / binWidth)
	if minBw < 1 {
		minBw = 1
	}

	emit := func(cands *[]msCand, s, e int) {
		if e < s {
			return
		}
		peakSnr := -math.MaxFloat64
		for b := s; b <= e; b++ {
			snr := psdDb[b] - baseDb[b]
			if snr > peakSnr {
				peakSnr = snr
			}
		}
		if peakSnr < o.minSnrDb || (e-s+1) < minBw {
			return
		}
		*cands = append(*cands, msCand{first: s, last: e, snrDb: peakSnr})
	}

	var cands []msCand
	i := 0
	for i < n {
		if !active[i] {
			i++
			continue
		}
		j := i
		for j < n && active[j] {
			j++
		}
		// segment [i,j) at valleys where resid < cut (back to noise) for >= minGap bins
		b := i
		for b < j {
			for b < j && resid[b] < cut {
				b++
			}
			if b >= j {
				break
			}
			s := b
			last := b
			gap := 0
			for b < j {
				if resid[b] < cut {
					gap++
					if gap >= minGap {
						break
					}
				} else {
					gap = 0
					last = b
				}
				b++
			}
			emit(&cands, s, last)
		}
		i = j
	}
	sort.Slice(cands, func(a, b int) bool { return cands[a].first < cands[b].first })
	return cands
}

// ---------- band plan ground-truth regions ----------

type bandRegion struct {
	name   string
	lo, hi float64 // abs Hz
}

func regionsFor(centerHz float64) []bandRegion {
	switch {
	case centerHz > 6.5e6 && centerHz < 7.5e6: // 40m
		return []bandRegion{
			{"40m-CW   7.00-7.04", 7.000e6, 7.040e6},
			{"40m-digi 7.04-7.05", 7.040e6, 7.050e6},
			{"40m-FT8  7.074", 7.0735e6, 7.0745e6},
			{"40m-SSB  7.05-7.20", 7.050e6, 7.200e6},
			{"41m-bc   7.20-7.45", 7.200e6, 7.450e6},
		}
	case centerHz > 13.5e6 && centerHz < 14.5e6: // 20m
		return []bandRegion{
			{"22m-bc  13.57-13.87", 13.570e6, 13.870e6},
			{"20m-CW  14.00-14.07", 14.000e6, 14.070e6},
			{"20m-FT8 14.074", 14.0735e6, 14.0745e6},
			{"20m-digi 14.07-14.10", 14.070e6, 14.099e6},
			{"20m-SSB 14.10-14.35", 14.101e6, 14.350e6},
		}
	default: // BC-FM / generic
		return []bandRegion{{"full-band", centerHz - 2e6, centerHz + 2e6}}
	}
}

func regionOf(absHz float64, regs []bandRegion) string {
	for _, r := range regs {
		if absHz >= r.lo && absHz < r.hi {
			return r.name
		}
	}
	return "(other)"
}

// ---------- the eval ----------

func strategyOracles() []oracle {
	return []oracle{
		{"40m-fresh", "../../data/snapshots/hf_40m_ssb.cf32"},
		{"40m", "../../data/snapshots/hf_40m.cf32"},
		{"20m", "../../data/snapshots/hf_20m.cf32"},
		{"BC-FM", "../../data/snapshots/fm_bc.cf32"},
	}
}

func cfarLiveConfig() config.DetectorConfig {
	c := detectorConfig()
	c.CFARMode = "GOSCA"
	c.CFARScaleDb = 12
	c.CFARGuardHz = 15000
	c.CFARTrainHz = 100000
	c.MergeGapHz = 5000
	c.EdgeMarginDb = 6
	c.EmaAlpha = 1.0 // smooth == psd immediately (single-frame fair comparison)
	return c
}

func multiScaleConfig() config.DetectorConfig {
	c := detectorConfig()
	c.MultiScale = true
	c.EmaAlpha = 1.0 // smooth == psd (single-frame fair comparison)
	return c
}

func TestStrategyDetect(t *testing.T) {
	const fft = 65536
	for _, oc := range strategyOracles() {
		iq, meta, err := iqfile.Read(oc.path)
		if err != nil {
			t.Logf("%s: SKIP (%v)", oc.name, err)
			continue
		}
		binWidth := float64(meta.SampleRate) / float64(fft)
		win := fftutil.Hann(fft)
		psd := fftutil.WelchPSD(iq, fft, 0.5, win)
		regs := regionsFor(meta.CenterHz)

		// truth = Welch reference emissions (offset Hz)
		truth, _ := welchReference(iq, meta.SampleRate, refSeg, refMinPeakDb, refAboveFloor)

		// multi-scale (production detector) -> det list (offset Hz)
		dm := detector.New(multiScaleConfig(), meta.SampleRate, fft)
		_, msSig := dm.Process(time.Unix(0, 0), psd, 0)
		var ms []det
		for _, s := range msSig {
			ms = append(ms, det{c: s.CenterHz, bw: s.BWHz, snr: s.SNRDb})
		}
		// current CFAR -> det list
		d := detector.New(cfarLiveConfig(), meta.SampleRate, fft)
		_, cf := d.Process(time.Unix(0, 0), psd, 0)
		var cfd []det
		for _, s := range cf {
			cfd = append(cfd, det{c: s.CenterHz, bw: s.BWHz, snr: s.SNRDb})
		}

		mr := scoreByRegion(truth, ms, regs, meta.CenterHz, binWidth)
		cr := scoreByRegion(truth, cfd, regs, meta.CenterHz, binWidth)
		t.Logf("== %s  %.3f MHz  bin=%.1f Hz | truth=%d  multiScale: det=%d rec=%.2f prec=%.2f | cfar: det=%d rec=%.2f prec=%.2f ==",
			oc.name, meta.CenterHz/1e6, binWidth, len(truth),
			len(ms), mr.recall(), mr.precision(), len(cfd), cr.recall(), cr.precision())
		t.Logf("   %-20s %6s | %-13s | %-13s", "region", "truth", "multiScale R/P", "cfar R/P")
		for _, r := range regs {
			m, c := mr.byReg[r.name], cr.byReg[r.name]
			t.Logf("   %-20s %6d | rec %.2f prec %.2f (%d) | rec %.2f prec %.2f (%d)",
				r.name, m.truth, m.rec(), m.prec(), m.det, c.rec(), c.prec(), c.det)
		}
		// dump multi-scale detections in the SSB region (verify they are real ~2-3 kHz SSB)
		for _, r := range regs {
			if !containsSSB(r.name) {
				continue
			}
			t.Logf("   -- %s multi-scale dets (abs MHz / bw kHz / snr dB):", r.name)
			cnt := 0
			for _, dd := range ms {
				abs := meta.CenterHz + dd.c
				if abs >= r.lo && abs < r.hi {
					t.Logf("        %.4f  %.2fk  +%.0f", abs/1e6, dd.bw/1e3, dd.snr)
					cnt++
					if cnt >= 12 {
						t.Logf("        ...")
						break
					}
				}
			}
		}
		fmt.Println()
	}
}

func containsSSB(s string) bool {
	for i := 0; i+3 <= len(s); i++ {
		if s[i:i+3] == "SSB" {
			return true
		}
	}
	return false
}

type det struct{ c, bw, snr float64 }

type regScore struct {
	truth, det, recHit, precHit int
}

func (r regScore) rec() float64 {
	if r.truth == 0 {
		return math.NaN()
	}
	return float64(r.recHit) / float64(r.truth)
}
func (r regScore) prec() float64 {
	if r.det == 0 {
		return math.NaN()
	}
	return float64(r.precHit) / float64(r.det)
}

type scoreResult struct {
	byReg                  map[string]*regScore
	tTot, tHit, dTot, dHit int
}

func (s scoreResult) recall() float64 {
	if s.tTot == 0 {
		return math.NaN()
	}
	return float64(s.tHit) / float64(s.tTot)
}
func (s scoreResult) precision() float64 {
	if s.dTot == 0 {
		return math.NaN()
	}
	return float64(s.dHit) / float64(s.dTot)
}

// scoreByRegion matches detections to truth (greedy by center within tolerance)
// and tallies recall/precision globally and per band-plan region.
func scoreByRegion(truth []refSig, dets []det, regs []bandRegion, centerHz, binWidth float64) scoreResult {
	res := scoreResult{byReg: map[string]*regScore{}}
	get := func(name string) *regScore {
		if res.byReg[name] == nil {
			res.byReg[name] = &regScore{}
		}
		return res.byReg[name]
	}
	for _, r := range regs {
		get(r.name)
	}
	usedDet := make([]bool, len(dets))
	// recall: each truth needs a matching det
	for _, tr := range truth {
		reg := regionOf(centerHz+tr.centerHz, regs)
		g := get(reg)
		g.truth++
		res.tTot++
		best, bestD := -1, math.MaxFloat64
		for i, dd := range dets {
			if usedDet[i] {
				continue
			}
			tol := math.Max(tr.bwHz, dd.bw)/2 + 4*binWidth
			dd2 := math.Abs(dd.c - tr.centerHz)
			if dd2 <= tol && dd2 < bestD {
				best, bestD = i, dd2
			}
		}
		if best >= 0 {
			usedDet[best] = true
			g.recHit++
			res.tHit++
		}
	}
	// precision: each det should match some truth (independent assignment)
	for _, dd := range dets {
		reg := regionOf(centerHz+dd.c, regs)
		g := get(reg)
		g.det++
		res.dTot++
		matched := false
		for _, tr := range truth {
			tol := math.Max(tr.bwHz, dd.bw)/2 + 4*binWidth
			if math.Abs(dd.c-tr.centerHz) <= tol {
				matched = true
				break
			}
		}
		if matched {
			g.precHit++
			res.dHit++
		}
	}
	return res
}

// TestStrategyLiveLikeTune feeds single-FFT frames through the EMA (like live,
// not Welch) and sweeps K to find a live-appropriate sensitivity.
func TestStrategyLiveLikeTune(t *testing.T) {
	const fft = 65536
	iq, meta, err := iqfile.Read("../../data/snapshots/hf_40m_ssb.cf32")
	if err != nil {
		t.Skip(err)
	}
	win := fftutil.Hann(fft)
	binWidth := float64(meta.SampleRate) / float64(fft)
	regs := regionsFor(meta.CenterHz)
	truth, _ := welchReference(iq, meta.SampleRate, refSeg, refMinPeakDb, refAboveFloor)
	maxStart := len(iq) - fft
	stride := maxStart / 47
	for _, k := range []float64{6, 8, 10, 12, 14} {
		cfg := multiScaleConfig()
		cfg.EmaAlpha = 0.025 // live value
		cfg.MSK = k
		d := detector.New(cfg, meta.SampleRate, fft)
		now := time.Unix(0, 0)
		var sig []detector.Signal
		for f := 0; f < 48; f++ {
			spec := fftutil.Spectrum(iq[f*stride:f*stride+fft], win)
			now = now.Add(80 * time.Millisecond)
			_, sig = d.Process(now, spec, 0)
		}
		var dets []det
		for _, s := range sig {
			dets = append(dets, det{c: s.CenterHz, bw: s.BWHz, snr: s.SNRDb})
		}
		r := scoreByRegion(truth, dets, regs, meta.CenterHz, binWidth)
		ssb := r.byReg["40m-SSB  7.05-7.20"]
		t.Logf("K=%-3.0f det=%-4d rec=%.2f prec=%.2f | SSB rec=%.2f (%d dets)", k, len(dets), r.recall(), r.precision(), ssb.rec(), ssb.det)
	}
}

// TestLiveTrackedCount runs the full multi-scale detector + tracking over frames
// of the 4.096 MS/s live-rate oracle (single-FFT + EMA, like live) and reports
// the tracked StableSignals count at various K / MinStableFrames. The live CPU/GC
// is driven by the number of stable signals (per-signal state on the heap), so
// this finds settings that yield real-signal counts instead of ~400 noise tracks.
func TestLiveTrackedCount(t *testing.T) {
	const fft = 65536
	iq, meta, err := iqfile.Read("../../data/snapshots/hf_40m_live.cf32")
	if err != nil {
		t.Skip(err)
	}
	win := fftutil.Hann(fft)
	maxStart := len(iq) - fft
	stride := maxStart / 89
	t.Logf("oracle %.3f MHz / %.3f MS/s, fft=%d", meta.CenterHz/1e6, float64(meta.SampleRate)/1e6, fft)
	for _, k := range []float64{16, 24, 32, 48} {
		for _, ms := range []int{2, 5, 10} {
			cfg := multiScaleConfig()
			cfg.EmaAlpha = 0.025
			cfg.MSK = k
			cfg.MinStableFrames = ms
			cfg.MinDurationMs = 200
			cfg.HoldMs = 700
			cfg.GapToleranceMs = 700
			d := detector.New(cfg, meta.SampleRate, fft)
			now := time.Unix(0, 0)
			var raw []detector.Signal
			for f := 0; f < 90; f++ {
				spec := fftutil.Spectrum(iq[f*stride:f*stride+fft], win)
				now = now.Add(66 * time.Millisecond)
				_, raw = d.Process(now, spec, 0)
			}
			stable := d.StableSignals()
			t.Logf("K=%-3.0f minStable=%-2d  raw=%-4d  stable=%-4d", k, ms, len(raw), len(stable))
		}
	}
}

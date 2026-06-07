//go:build bench

// Real-capture detector parameter evaluation (Phase R / L1, OI-21 #53, part of #5).
//
// The #51 synth sweep (param_eval_test.go) found single-resolution @ cfar_scale≈12
// reaches P/R~1.0 on every synth scene — suggesting the scale-space L1 may be
// unnecessary. But synth under-represents the real big-antenna regime, and we have
// NO known-good CFAR/scale params validated on real RF (for BC-FM or the HF mix).
// So this is the real twin of that sweep: same detector, same levers, but the
// "scene" is a time window of a captured oracle, and the reference is the fine
// Welch peak set ("what is on the air") instead of synthetic ground truth.
//
// The reference is not a precise P/R label set; it is a CONSISTENT reference that
// scores every parameter config identically, so config-vs-config comparison is
// valid even though absolute P/R is approximate. The decisive question is
// structural: does any single-resolution operating point resolve the narrow end
// (FT8/CW) AND the wide end (broadcast) at once? If not, that is the evidence for
// the scale-space L1 / per-band decimation.
//
//	go test -tags bench -run TestRealOracleReference ./internal/synth/ -v
//
// Oracles (gitignored, under data/snapshots/):
//   - fm_bc.cf32  101.5 MHz / 4.096 MS/s  — BC-FM (two ~30 dB stations)
//   - hf_40m.cf32  7.100 MHz / 2.000 MS/s — 40m ham (FT8/CW) + 41m broadcast
//   - hf_20m.cf32 14.100 MHz / 2.000 MS/s — 20m ham + a +51 dB 22m broadcast cluster
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

type oracle struct {
	name string
	path string
}

func oracles() []oracle {
	return []oracle{
		{"BC-FM   ", "../../data/snapshots/fm_bc.cf32"},
		{"40m     ", "../../data/snapshots/hf_40m.cf32"},
		{"20m     ", "../../data/snapshots/hf_20m.cf32"},
	}
}

// refSig is one reference emission in baseband-offset Hz (relative to the capture
// center): centerHz is the power-weighted centroid, bwHz the width of the region
// above floor+aboveFloorDb, peakDb the strongest bin over the floor.
type refSig struct {
	centerHz float64
	bwHz     float64
	peakDb   float64
}

// welchReference derives the reference emission set from a high-resolution Welch
// PSD: peak-pick bins >= minPeakDb over the median floor, expand each to the
// contiguous region above floor+aboveFloorDb, and merge peaks whose regions
// overlap into one emission. Returns emissions in baseband-offset Hz.
func welchReference(iq []complex64, fs, seg int, minPeakDb, aboveFloorDb float64) ([]refSig, float64) {
	win := fftutil.Hann(seg)
	psd := fftutil.WelchPSD(iq, seg, 0.5, win) // dB, fftshifted (DC at seg/2)
	floor := medianf(psd)
	binHz := float64(fs) / float64(seg)
	thr := floor + aboveFloorDb

	// Mark every bin that is above the occupancy threshold.
	above := make([]bool, len(psd))
	for i, db := range psd {
		above[i] = db > thr
	}
	// Walk contiguous above-threshold runs; a run is an emission only if it
	// contains at least one bin >= floor+minPeakDb. Centroid is power-weighted.
	var refs []refSig
	i := 0
	for i < len(psd) {
		if !above[i] {
			i++
			continue
		}
		j := i
		peak := -math.MaxFloat64
		var wSum, wfSum float64
		for j < len(psd) && above[j] {
			over := psd[j] - floor
			if over > peak {
				peak = over
			}
			lin := math.Pow(10, psd[j]/10)
			off := (float64(j) - float64(seg)/2) * binHz
			wSum += lin
			wfSum += lin * off
			j++
		}
		if peak >= minPeakDb {
			refs = append(refs, refSig{
				centerHz: wfSum / wSum,
				bwHz:     float64(j-i) * binHz,
				peakDb:   peak,
			})
		}
		i = j
	}
	return refs, floor
}

// TestRealOracleReference derives and prints the Welch reference landmarks for
// each oracle, split narrow (<5 kHz: CW/FT8/SSB) vs wide (>=5 kHz: NFM/AM/
// broadcast), for operator sanity-check before the parameter sweep trusts them.
func TestRealOracleReference(t *testing.T) {
	for _, o := range oracles() {
		iq, meta, err := iqfile.Read(o.path)
		if err != nil {
			t.Logf("%s: SKIP (%v)", o.name, err)
			continue
		}
		refs, floor := welchReference(iq, meta.SampleRate, refSeg, refMinPeakDb, refAboveFloor)
		binHz := float64(meta.SampleRate) / float64(refSeg)
		sort.Slice(refs, func(a, b int) bool { return refs[a].peakDb > refs[b].peakDb })

		// Two tiers x narrow/wide (operator-chosen reporting).
		var sN, sW, wN, wW int
		var widest float64
		for _, r := range refs {
			strong := r.peakDb >= refStrongDb
			narrow := r.bwHz < narrowMaxBwHz
			switch {
			case strong && narrow:
				sN++
			case strong && !narrow:
				sW++
			case !strong && narrow:
				wN++
			default:
				wW++
			}
			if r.bwHz > widest {
				widest = r.bwHz
			}
		}
		t.Logf("== %s  %.3f MHz / %.3f MS/s  bin=%.0f Hz floor=%.1f dB ==",
			o.name, meta.CenterHz/1e6, float64(meta.SampleRate)/1e6, binHz, floor)
		t.Logf("   %d emissions | STRONG(>=%.0fdB): %d narrow + %d wide | WEAK(%.0f-%.0fdB): %d narrow + %d wide | widest=%.1f kHz",
			len(refs), refStrongDb, sN, sW, refMinPeakDb, refStrongDb, wN, wW, widest/1e3)
		t.Logf("   %-12s %-10s %-9s %s", "abs-MHz", "off-kHz", "bw-kHz", "peak-dB")
		for k, r := range refs {
			if k >= 25 {
				t.Logf("   ... (%d more)", len(refs)-25)
				break
			}
			abs := meta.CenterHz + r.centerHz
			t.Logf("   %-12.4f %-10.1f %-9.2f +%.1f",
				abs/1e6, r.centerHz/1e3, r.bwHz/1e3, r.peakDb)
		}
		fmt.Println()
	}
}

// --- parameter sweep against the real oracles ---

const (
	refMinPeakDb   = 10.0  // an emission must clear noise by this to be in the reference
	refStrongDb    = 14.0  // strong tier: >= this over floor; weak tier: [refMinPeakDb, this)
	refAboveFloor  = 6.0   // occupied-width threshold over the floor
	refSeg         = 32768 // Welch resolution for the reference
	narrowMaxBwHz  = 5000.0
	sweepFrames    = 24 // frames spanning the capture
	sweepWarm      = 6  // EMA settle frames (ignored in the union)
	sweepFrameMs   = 100
	clusterMinHits = 2 // a detected cluster must appear in >= this many frames (drop 1-frame noise)
)

// detCluster is a detection aggregated across frames (union over the capture, so
// bursty signals like FT8 that are not present every frame still count).
type detCluster struct {
	center, bw float64
	hits       int
}

// realScene slides sweepFrames windows of fftSize across the capture, feeds each
// frame's periodogram to the detector (advancing time so the EMA/hold logic runs),
// and returns the union of post-warmup raw detections clustered by center.
func realScene(iq []complex64, fs, fftSize int, cfg config.DetectorConfig) []detCluster {
	return realSceneFrames(iq, fs, fftSize, cfg, sweepFrames, sweepWarm)
}

// realSceneFrames is realScene with an explicit frame count + warmup, so the
// diagnosis can test recall sensitivity to the number of frames (non-stationarity).
func realSceneFrames(iq []complex64, fs, fftSize int, cfg config.DetectorConfig, frames, warm int) []detCluster {
	d := detector.New(cfg, fs, fftSize)
	win := fftutil.Hann(fftSize)
	binWidth := float64(fs) / float64(fftSize)
	maxStart := len(iq) - fftSize
	if maxStart < 0 {
		return nil
	}
	stride := 0
	if frames > 1 {
		stride = maxStart / (frames - 1)
	}
	now := time.Unix(0, 0)
	type det struct{ c, b float64 }
	var all []det
	for f := 0; f < frames; f++ {
		start := f * stride
		spec := fftutil.Spectrum(iq[start:start+fftSize], win)
		now = now.Add(sweepFrameMs * time.Millisecond)
		_, raw := d.Process(now, spec, 0)
		if f < warm {
			continue
		}
		for _, g := range raw {
			all = append(all, det{g.CenterHz, g.BWHz})
		}
	}
	// Greedy-cluster by center: merge detections whose centers fall within a
	// tolerance (half the wider bandwidth + 2 bins).
	sort.Slice(all, func(a, b int) bool { return all[a].c < all[b].c })
	var clusters []detCluster
	for _, g := range all {
		merged := false
		for i := range clusters {
			tol := math.Max(clusters[i].bw, g.b)/2 + 2*binWidth
			if math.Abs(clusters[i].center-g.c) <= tol {
				// running mean center, max bw, +1 hit
				n := float64(clusters[i].hits)
				clusters[i].center = (clusters[i].center*n + g.c) / (n + 1)
				if g.b > clusters[i].bw {
					clusters[i].bw = g.b
				}
				clusters[i].hits++
				merged = true
				break
			}
		}
		if !merged {
			clusters = append(clusters, detCluster{center: g.c, bw: g.b, hits: 1})
		}
	}
	return clusters
}

type realMetrics struct {
	recStrongNarrow, recStrongWide, recWeak float64
	precision                               float64
	maxBwK                                  float64
	overWide                                int // detected clusters wider than 1.5x widest ref
	fragmented                              int // wide refs matched by >= 2 clusters
}

// scoreReal matches detected clusters (hits >= clusterMinHits) to the reference,
// reporting tiered recall (strong narrow / strong wide / weak), precision, and the
// bridging (overWide) and fragmentation indicators.
func scoreReal(refs []refSig, floor float64, clusters []detCluster, binWidth float64) realMetrics {
	// keep only clusters seen in enough frames (drops single-frame noise spikes)
	var det []detCluster
	maxRefBw := 0.0
	for _, r := range refs {
		if r.bwHz > maxRefBw {
			maxRefBw = r.bwHz
		}
	}
	for _, c := range clusters {
		if c.hits >= clusterMinHits {
			det = append(det, c)
		}
	}
	used := make([]bool, len(det))
	var m realMetrics
	var sN, sNhit, sW, sWhit, wk, wkHit int
	for _, r := range refs {
		strong := r.peakDb >= refStrongDb
		narrow := r.bwHz < narrowMaxBwHz
		switch {
		case strong && narrow:
			sN++
		case strong && !narrow:
			sW++
		default:
			wk++
		}
		// match: nearest unused cluster within tolerance
		best, bestD, matches := -1, math.MaxFloat64, 0
		for i, c := range det {
			tol := math.Max(r.bwHz, c.bw)/2 + 2*binWidth
			d := math.Abs(c.center - r.centerHz)
			if d <= tol {
				matches++
				if !used[i] && d < bestD {
					best, bestD = i, d
				}
			}
		}
		if best >= 0 {
			used[best] = true
			switch {
			case strong && narrow:
				sNhit++
			case strong && !narrow:
				sWhit++
			default:
				wkHit++
			}
			if !narrow && matches >= 2 {
				m.fragmented++
			}
		}
	}
	matchedClusters := 0
	for i, c := range det {
		if used[i] {
			matchedClusters++
		}
		if c.bw/1e3 > m.maxBwK {
			m.maxBwK = c.bw / 1e3
		}
		if maxRefBw > 0 && c.bw > 1.5*maxRefBw {
			m.overWide++
		}
	}
	m.recStrongNarrow = safeDiv(sNhit, sN)
	m.recStrongWide = safeDiv(sWhit, sW)
	m.recWeak = safeDiv(wkHit, wk)
	m.precision = safeDiv(matchedClusters, len(det))
	return m
}

// realCache holds an oracle's IQ + meta + reference (computed once, reused across
// configs).
type realCache struct {
	name  string
	iq    []complex64
	fs    int
	refs  []refSig
	floor float64
}

func loadOracles(t *testing.T) []realCache {
	var out []realCache
	for _, o := range oracles() {
		iq, meta, err := iqfile.Read(o.path)
		if err != nil {
			t.Logf("%s: SKIP (%v)", o.name, err)
			continue
		}
		refs, floor := welchReference(iq, meta.SampleRate, refSeg, refMinPeakDb, refAboveFloor)
		out = append(out, realCache{name: o.name, iq: iq, fs: meta.SampleRate, refs: refs, floor: floor})
	}
	return out
}

func runRealRow(t *testing.T, label string, caches []realCache, fftSize int, cfg config.DetectorConfig) {
	for _, c := range caches {
		binWidth := float64(c.fs) / float64(fftSize)
		clusters := realScene(c.iq, c.fs, fftSize, cfg)
		m := scoreReal(c.refs, c.floor, clusters, binWidth)
		t.Logf("  %-14s %s Rs_n=%.2f Rs_w=%.2f Rw=%.2f P=%.2f maxBw=%6.0fk oWide=%2d frag=%d",
			label, c.name, m.recStrongNarrow, m.recStrongWide, m.recWeak, m.precision, m.maxBwK, m.overWide, m.fragmented)
	}
}

// TestRealParamSweep mirrors the #51 synth sweep on the real oracles. Columns:
// Rs_n = strong-tier NARROW recall (CW/FT8/SSB, the resolution-critical end),
// Rs_w = strong-tier WIDE recall (broadcast), Rw = weak-tier recall, P = precision
// (matched clusters / detected), maxBw = widest detected (bridging magnitude),
// oWide = #clusters wider than 1.5x the widest reference (bridging), frag = wide
// refs split across >=2 clusters. The decisive read: does ANY single row reach
// high Rs_n AND high Rs_w with low oWide/frag on ALL oracles? If narrow-good rows
// always bridge the wide (high oWide) and wide-good rows miss the narrow (low
// Rs_n), that tradeoff is the evidence for the scale-space L1.
func TestRealParamSweep(t *testing.T) {
	caches := loadOracles(t)
	if len(caches) == 0 {
		t.Skip("no oracles available")
	}
	base := detectorConfig()
	const baseFFT = 65536
	t.Logf("Real-capture sweep (baseline: guard=%.0fk train=%.0fk scale=%.0f edge=%.0f merge=%.0fk maxBw=%.0fk, fft=%d, %d frames union)",
		base.CFARGuardHz/1e3, base.CFARTrainHz/1e3, base.CFARScaleDb, base.EdgeMarginDb, base.MergeGapHz/1e3, base.MaxSignalBwHz/1e3, baseFFT, sweepFrames)

	t.Logf("== baseline ==")
	runRealRow(t, "baseline", caches, baseFFT, base)

	t.Logf("== FFT size (resolution lever) ==")
	for _, fft := range []int{16384, 32768, 65536, 131072} {
		runRealRow(t, fmt.Sprintf("fft=%d", fft), caches, fft, base)
	}

	t.Logf("== CFARScaleDb (precision lever) ==")
	for _, v := range []float64{7, 10, 12, 16} {
		c := base
		c.CFARScaleDb = v
		runRealRow(t, fmt.Sprintf("scale=%.0f", v), caches, baseFFT, c)
	}

	t.Logf("== CFARGuardHz ==")
	for _, v := range []float64{15e3, 50e3, 250e3} {
		c := base
		c.CFARGuardHz = v
		runRealRow(t, fmt.Sprintf("guard=%.0fk", v/1e3), caches, baseFFT, c)
	}

	t.Logf("== MergeGapHz ==")
	for _, v := range []float64{0, 5e3, 20e3, 50e3} {
		c := base
		c.MergeGapHz = v
		runRealRow(t, fmt.Sprintf("merge=%.0fk", v/1e3), caches, baseFFT, c)
	}

	t.Logf("== MaxSignalBwHz ==")
	for _, v := range []float64{130e3, 260e3, 520e3} {
		c := base
		c.MaxSignalBwHz = v
		runRealRow(t, fmt.Sprintf("maxBw=%.0fk", v/1e3), caches, baseFFT, c)
	}

	t.Logf("== best-guess combos (scale x fft) ==")
	for _, fft := range []int{32768, 65536} {
		for _, sc := range []float64{10, 12} {
			c := base
			c.CFARScaleDb = sc
			c.CFARGuardHz = 15e3
			runRealRow(t, fmt.Sprintf("fft=%d/sc=%.0f", fft, sc), caches, fft, c)
		}
	}

	// Tradeoff-breaker: the one-at-a-time sweep showed MergeGap=0 nearly doubles
	// narrow recall (merge eats dense narrow signals) while scale=12 fixes
	// precision. Their interaction is where a single working operating point would
	// live, so test it directly.
	t.Logf("== tradeoff-breaker: low merge x high scale (fft=32768, guard=15k) ==")
	for _, mg := range []float64{0, 5e3} {
		for _, sc := range []float64{10, 12, 14} {
			c := base
			c.CFARScaleDb = sc
			c.CFARGuardHz = 15e3
			c.MergeGapHz = mg
			runRealRow(t, fmt.Sprintf("merge=%.0fk/sc=%.0f", mg/1e3, sc), caches, 32768, c)
		}
	}
}

// liveBaseConfig mirrors the operator's config.autosave.yaml detector block (the
// config that actually runs live) so we can validate the candidate change on the
// EXACT live base, not on the GOSCA test baseline.
func liveBaseConfig() config.DetectorConfig {
	c := detectorConfig()
	c.CFARMode = "CASO"
	c.EdgeMarginDb = 6
	c.CFARTrainHz = 100000
	c.HysteresisDb = 10
	c.MinStableFrames = 4
	// ema/hold/gap are temporal/tracking; left near defaults (do not affect raw
	// per-frame detection materially in this harness).
	return c
}

// TestRealLiveConfigCheck validates the candidate known-good change on the live
// base config: does cfar_scale 7->12 + merge 20k->5k improve precision/over-widening
// on the live (CASO) base the same way it did on the GOSCA baseline? Relative
// comparison on the same base, so harness/ema limitations cancel.
func TestRealLiveConfigCheck(t *testing.T) {
	caches := loadOracles(t)
	if len(caches) == 0 {
		t.Skip("no oracles available")
	}
	const fft = 65536 // live autosave fft_size
	type variant struct {
		name         string
		mode         string
		scale, merge float64
	}
	variants := []variant{
		{"live-now  CASO sc7/mg20k", "CASO", 7, 20e3},
		{"cand-CASO  sc12/mg5k", "CASO", 12, 5e3},
		{"cand-GOSCA sc12/mg5k", "GOSCA", 12, 5e3},
	}
	for _, v := range variants {
		cfg := liveBaseConfig()
		cfg.CFARMode = v.mode
		cfg.CFARScaleDb = v.scale
		cfg.MergeGapHz = v.merge
		t.Logf("== %s (CASO base, fft=%d) ==", v.name, fft)
		for _, c := range caches {
			binWidth := float64(c.fs) / float64(fft)
			clusters := realSceneFrames(c.iq, c.fs, fft, cfg, 48, 12)
			m := scoreReal(c.refs, c.floor, clusters, binWidth)
			t.Logf("  %s Rs_n=%.2f Rs_w=%.2f Rw=%.2f P=%.2f maxBw=%6.0fk oWide=%d frag=%d",
				c.name, m.recStrongNarrow, m.recStrongWide, m.recWeak, m.precision, m.maxBwK, m.overWide, m.fragmented)
		}
	}
}

// matchCluster returns the nearest cluster to centerHz within tolerance, or nil.
func matchCluster(centerHz, bwHz float64, clusters []detCluster, binWidth float64) *detCluster {
	best, bestD := -1, math.MaxFloat64
	for i, c := range clusters {
		tol := math.Max(bwHz, c.bw)/2 + 2*binWidth
		d := math.Abs(c.center - centerHz)
		if d <= tol && d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 {
		return nil
	}
	return &clusters[best]
}

// TestRealRecallDiagnosis (#55 step 1) separates measurement artifacts from a real
// recall failure before any detector change:
//
//	A. reference robustness — does each reference emission reappear when the Welch
//	   is computed on the first vs second half of the capture? A real signal does;
//	   a noise/spur spike does not. Splits the narrow reference into confirmed vs
//	   likely-noise, per tier.
//	B. frame-count sensitivity — does narrow recall climb with more frames (a
//	   duty-cycle / non-stationarity artifact) or plateau (a genuine miss)?
//	C. per-signal duty cycle — at 96 frames, how many frames is each strong-narrow
//	   reference actually detected in? hits=0 is a real miss; low hits is bursty.
func TestRealRecallDiagnosis(t *testing.T) {
	caches := loadOracles(t)
	if len(caches) == 0 {
		t.Skip("no oracles available")
	}
	best := detectorConfig()
	best.CFARScaleDb = 12
	best.CFARGuardHz = 15e3
	best.MergeGapHz = 0
	const fft = 32768

	t.Logf("=== A. reference robustness (split-half: confirmed in BOTH halves = real) ===")
	for _, c := range caches {
		half := len(c.iq) / 2
		r1, _ := welchReference(c.iq[:half], c.fs, refSeg, refMinPeakDb, refAboveFloor)
		r2, _ := welchReference(c.iq[half:], c.fs, refSeg, refMinPeakDb, refAboveFloor)
		binHz := float64(c.fs) / float64(refSeg)
		tol := func() float64 { return 3 * binHz }
		in := func(center float64, rs []refSig) bool {
			for _, r := range rs {
				if math.Abs(r.centerHz-center) <= math.Max(tol(), r.bwHz/2) {
					return true
				}
			}
			return false
		}
		var sBoth, sNarrowBoth, sNarrow, wBoth, w int
		for _, r := range c.refs {
			strong := r.peakDb >= refStrongDb
			narrow := r.bwHz < narrowMaxBwHz
			both := in(r.centerHz, r1) && in(r.centerHz, r2)
			if strong {
				if narrow {
					sNarrow++
					if both {
						sNarrowBoth++
					}
				}
				if both {
					sBoth++
				}
			} else {
				w++
				if both {
					wBoth++
				}
			}
		}
		t.Logf("  %s strong: %d confirmed-both | strong-narrow: %d/%d confirmed | weak: %d/%d confirmed",
			c.name, sBoth, sNarrowBoth, sNarrow, wBoth, w)
	}

	t.Logf("=== B. frame-count sensitivity (best cfg merge0/sc12/fft32768) ===")
	for _, c := range caches {
		binWidth := float64(c.fs) / float64(fft)
		for _, fr := range []int{24, 48, 96} {
			clusters := realSceneFrames(c.iq, c.fs, fft, best, fr, fr/4)
			m := scoreReal(c.refs, c.floor, clusters, binWidth)
			t.Logf("  %s frames=%-3d Rs_n=%.2f Rs_w=%.2f Rw=%.2f P=%.2f",
				c.name, fr, m.recStrongNarrow, m.recStrongWide, m.recWeak, m.precision)
		}
	}

	t.Logf("=== C. per-signal duty cycle @ 96 frames (strong-narrow refs) ===")
	for _, c := range caches {
		binWidth := float64(c.fs) / float64(fft)
		clusters := realSceneFrames(c.iq, c.fs, fft, best, 96, 24)
		// sort strong-narrow refs by peak for readability
		var sn []refSig
		for _, r := range c.refs {
			if r.peakDb >= refStrongDb && r.bwHz < narrowMaxBwHz {
				sn = append(sn, r)
			}
		}
		sort.Slice(sn, func(a, b int) bool { return sn[a].peakDb > sn[b].peakDb })
		miss, hitN := 0, 0
		for _, r := range sn {
			cl := matchCluster(r.centerHz, r.bwHz, clusters, binWidth)
			if cl == nil {
				miss++
			} else {
				hitN++
			}
		}
		t.Logf("  %s strong-narrow: %d detected / %d total (%d missed)", c.name, hitN, len(sn), miss)
		// detail for the first few (peak, bw, detected hits/72)
		for i, r := range sn {
			if i >= 8 {
				break
			}
			cl := matchCluster(r.centerHz, r.bwHz, clusters, binWidth)
			if cl == nil {
				t.Logf("    off=%+8.1fk bw=%.2fk peak=+%.1f  -> MISSED", r.centerHz/1e3, r.bwHz/1e3, r.peakDb)
			} else {
				t.Logf("    off=%+8.1fk bw=%.2fk peak=+%.1f  -> hits=%d detBw=%.2fk", r.centerHz/1e3, r.bwHz/1e3, r.peakDb, cl.hits, cl.bw/1e3)
			}
		}
	}

	t.Logf("=== D. blending stage: MaxSignalBwHz caps EXPANSION (small cap splits cluster => expansion is the culprit) ===")
	for _, c := range caches {
		binWidth := float64(c.fs) / float64(fft)
		var sn []refSig
		for _, r := range c.refs {
			if r.peakDb >= refStrongDb && r.bwHz < narrowMaxBwHz {
				sn = append(sn, r)
			}
		}
		for _, mx := range []float64{2e3, 5e3, 20e3, 260e3} {
			cfg := best
			cfg.MaxSignalBwHz = mx
			clusters := realSceneFrames(c.iq, c.fs, fft, cfg, 96, 24)
			// median detBw of matched strong-narrow clusters + how many resolved
			var detBws []float64
			hit := 0
			for _, r := range sn {
				if cl := matchCluster(r.centerHz, r.bwHz, clusters, binWidth); cl != nil {
					hit++
					detBws = append(detBws, cl.bw)
				}
			}
			m := scoreReal(c.refs, c.floor, clusters, binWidth)
			t.Logf("  %s maxBw=%6.0fk Rs_n=%.2f Rs_w=%.2f narrowHit=%2d/%2d medDetBw=%.2fk",
				c.name, mx/1e3, m.recStrongNarrow, m.recStrongWide, hit, len(sn), medianf(detBws)/1e3)
		}
	}
}

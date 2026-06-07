//go:build bench

// Detector parameter sensitivity evaluation (Phase R / L1, OI-21 #5).
//
// Why this exists: the L1-B "sharp CFAR guard" mechanism failed live because the
// synth probe (2.5 MS/s, FFT 16384) did not match the real regime, and because
// over-widening is produced by the detector's edge-expansion + (uncapped) merge
// stages, not the CFAR guard. Rather than guess the next single lever, this sweeps
// the real levers one-at-a-time around a baseline, on ground-truth scenes at the
// REAL operating params, and reports detection P/R, occupied-bw error, and an
// over-widening / bridging indicator. The data picks the lever and tells us
// whether any single-resolution config can serve narrow+wide simultaneously
// (if not, that is the evidence for the scale-space L1).
//
//	go test -tags bench -run TestDetectorParamSensitivity ./internal/synth/ -v
//
// Scenes model the big-antenna live regime (operator note 2026-06-07): strong
// ~50 dB signals, dense band, interference + weak signals — NOT the modest replay
// snapshot (two ~30 dB stations). They are multi-modulation, since Argus targets
// far more than BC-WFM.
package synth_test

import (
	"fmt"
	"testing"
	"time"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/detector"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/synth"
)

const realFs = 4_096_000 // big-antenna live sample rate

// evalScene runs one scene through the detector at the given FFT size + config
// over several breathing realizations and returns aggregate quality metrics:
// precision, recall, median occupied-bw error %, the widest detected bandwidth
// (kHz), and the per-frame count of "over-wide" detections (bw > 1.5x the widest
// truth signal) — the bridging / skirt-runaway indicator.
func evalScene(scene synth.Scene, fftSize int, cfg config.DetectorConfig) (prec, rec, bwMed, maxBwK, overWide float64) {
	d := detector.New(cfg, scene.SampleRate, fftSize)
	win := fftutil.Hann(fftSize)
	binWidth := float64(scene.SampleRate) / float64(fftSize)
	truthMaxBw := 0.0
	for _, s := range scene.Signals {
		if s.BandwidthHz > truthMaxBw {
			truthMaxBw = s.BandwidthHz
		}
	}
	now := time.Unix(0, 0)
	var tp, fp, fn, overCnt, frames int
	var bwErrs []float64
	var maxBw float64
	const F, warm = 24, 8
	for f := 0; f < F; f++ {
		s := scene
		s.Seed = scene.Seed + int64(f)
		spec := fftutil.Spectrum(s.Generate(fftSize), win)
		now = now.Add(83 * time.Millisecond)
		_, raw := d.Process(now, spec, 0)
		if f < warm {
			continue
		}
		frames++
		m := score(scene.Signals, raw, binWidth)
		tp += m.tp
		fp += m.fp
		fn += m.fn
		bwErrs = append(bwErrs, m.bwErrPct...)
		for _, g := range raw {
			if g.BWHz > maxBw {
				maxBw = g.BWHz
			}
			if truthMaxBw > 0 && g.BWHz > 1.5*truthMaxBw {
				overCnt++
			}
		}
	}
	prec = safeDiv(tp, tp+fp)
	rec = safeDiv(tp, tp+fn)
	bwMed = medianf(bwErrs)
	if frames == 0 {
		frames = 1
	}
	return prec, rec, bwMed, maxBw / 1e3, float64(overCnt) / float64(frames)
}

// --- ground-truth scenes (big-antenna regime, multi-modulation) ---

func evalSceneLoneWFM(seed int64) synth.Scene {
	return synth.Scene{SampleRate: realFs, Seed: seed, NoiseStd: 1.0, Signals: []synth.SignalSpec{
		{Kind: synth.KindWFM, CenterHz: 0, BandwidthHz: 180e3, SNRdB: 50, Dynamic: true},
	}}
}

func evalScenePair(seed int64, spacing float64) synth.Scene {
	return synth.Scene{SampleRate: realFs, Seed: seed, NoiseStd: 1.0, Signals: []synth.SignalSpec{
		{Kind: synth.KindWFM, CenterHz: -spacing / 2, BandwidthHz: 180e3, SNRdB: 52, Dynamic: true},
		{Kind: synth.KindWFM, CenterHz: spacing / 2, BandwidthHz: 180e3, SNRdB: 50, Dynamic: true},
	}}
}

// evalSceneMixed: many modulations at high SNR — the one-size / universality test.
func evalSceneMixed(seed int64) synth.Scene {
	return synth.Scene{SampleRate: realFs, Seed: seed, NoiseStd: 1.0, Signals: []synth.SignalSpec{
		{Kind: synth.KindWFM, CenterHz: -1.5e6, BandwidthHz: 180e3, SNRdB: 50, Dynamic: true},
		{Kind: synth.KindNFM, CenterHz: -0.6e6, BandwidthHz: 12e3, SNRdB: 45},
		{Kind: synth.KindAM, CenterHz: 0, BandwidthHz: 8e3, SNRdB: 40},
		{Kind: synth.KindSSB, CenterHz: 0.5e6, BandwidthHz: 3e3, SNRdB: 35},
		{Kind: synth.KindCW, CenterHz: 1.0e6, BandwidthHz: 100, SNRdB: 40},
		{Kind: synth.KindDigital, CenterHz: 1.5e6, BandwidthHz: 25e3, SNRdB: 45},
	}}
}

// evalSceneDense: strong close WFM pair + weak narrowband + a wide digital
// interferer — dynamic range + density + false-positive stress.
func evalSceneDense(seed int64) synth.Scene {
	return synth.Scene{SampleRate: realFs, Seed: seed, NoiseStd: 1.0, Signals: []synth.SignalSpec{
		{Kind: synth.KindWFM, CenterHz: -1.2e6, BandwidthHz: 180e3, SNRdB: 50, Dynamic: true},
		{Kind: synth.KindWFM, CenterHz: -0.9e6, BandwidthHz: 180e3, SNRdB: 50, Dynamic: true}, // 300k pair
		{Kind: synth.KindNFM, CenterHz: 0.2e6, BandwidthHz: 12e3, SNRdB: 12},                  // weak
		{Kind: synth.KindCW, CenterHz: 0.6e6, BandwidthHz: 100, SNRdB: 10},                    // weak
		{Kind: synth.KindDigital, CenterHz: 1.3e6, BandwidthHz: 300e3, SNRdB: 40},             // wide interferer
	}}
}

type evalScn struct {
	name  string
	scene synth.Scene
}

func evalScenes() []evalScn {
	return []evalScn{
		{"lone-wfm", evalSceneLoneWFM(1000)},
		{"pair-200k", evalScenePair(2000, 200e3)},
		{"pair-300k", evalScenePair(2100, 300e3)},
		{"mixed", evalSceneMixed(3000)},
		{"dense+interf", evalSceneDense(4000)},
	}
}

// evalBaseline mirrors the shipped detector defaults at the real FFT size.
const evalBaselineFFT = 65536

func evalBaselineCfg() config.DetectorConfig { return detectorConfig() }

func runSweepRow(t *testing.T, label string, fftSize int, cfg config.DetectorConfig) {
	for _, sc := range evalScenes() {
		p, r, bw, mx, ow := evalScene(sc.scene, fftSize, cfg)
		t.Logf("  %-18s %-13s P=%.2f R=%.2f bwMed%%=%6.1f maxBw=%6.0fk overWide=%.1f",
			label, sc.name, p, r, bw, mx, ow)
	}
}

// TestDetectorParamSensitivity sweeps each lever one-at-a-time around the shipped
// baseline at the real sample rate, logging quality per scene. It is a
// measurement (no pass/fail): read the tables to see which levers move
// over-widening (maxBw/overWide) and bridging, whether 65536 is right for WFM,
// and whether any single config serves the mixed scene.
func TestDetectorParamSensitivity(t *testing.T) {
	base := evalBaselineCfg()
	t.Logf("Detector parameter sensitivity @ %.3f MS/s (baseline: guard=%.0fk train=%.0fk scale=%.0f edge=%.0f merge=%.0fk maxBw=%.0fk, fft=%d)",
		float64(realFs)/1e6, base.CFARGuardHz/1e3, base.CFARTrainHz/1e3, base.CFARScaleDb, base.EdgeMarginDb, base.MergeGapHz/1e3, base.MaxSignalBwHz/1e3, evalBaselineFFT)

	t.Logf("== baseline ==")
	runSweepRow(t, "baseline", evalBaselineFFT, base)

	t.Logf("== FFT size ==")
	for _, fft := range []int{16384, 32768, 65536} {
		runSweepRow(t, fmt.Sprintf("fft=%d", fft), fft, base)
	}

	t.Logf("== MergeGapHz ==")
	for _, v := range []float64{0, 5e3, 20e3, 50e3} {
		c := base
		c.MergeGapHz = v
		runSweepRow(t, fmt.Sprintf("merge=%.0fk", v/1e3), evalBaselineFFT, c)
	}

	t.Logf("== EdgeMarginDb ==")
	for _, v := range []float64{2, 4, 8} {
		c := base
		c.EdgeMarginDb = v
		runSweepRow(t, fmt.Sprintf("edge=%.0f", v), evalBaselineFFT, c)
	}

	t.Logf("== MaxSignalBwHz ==")
	for _, v := range []float64{130e3, 260e3, 520e3} {
		c := base
		c.MaxSignalBwHz = v
		runSweepRow(t, fmt.Sprintf("maxBw=%.0fk", v/1e3), evalBaselineFFT, c)
	}

	t.Logf("== CFARGuardHz ==")
	for _, v := range []float64{15e3, 50e3, 250e3} {
		c := base
		c.CFARGuardHz = v
		runSweepRow(t, fmt.Sprintf("guard=%.0fk", v/1e3), evalBaselineFFT, c)
	}

	t.Logf("== CFARScaleDb ==")
	for _, v := range []float64{7, 12, 20} {
		c := base
		c.CFARScaleDb = v
		runSweepRow(t, fmt.Sprintf("scale=%.0f", v), evalBaselineFFT, c)
	}
}

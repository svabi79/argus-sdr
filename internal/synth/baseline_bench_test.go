//go:build bench

// Ground-truth detection/estimation benchmark for the current pipeline.
// Build-tagged so it does not run in the normal `go test ./...` sweep.
//
//	go test -tags bench -run TestDetectionBaseline ./internal/synth/ -v
//
// See docs/detection-rework-plan-2026-06-06.md (Phase R, steps R0.3-R0.5).
package synth_test

import (
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/estimate"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/synth"
)

const (
	benchFs     = 2_500_000
	benchN      = 8192
	benchFrames = 6
)

// detectorConfig mirrors the shipped config.yaml detector defaults.
func detectorConfig() config.DetectorConfig {
	return config.DetectorConfig{
		ThresholdDb: -60, MinDurationMs: 120, HoldMs: 1500, EmaAlpha: 0.35,
		HysteresisDb: 6, MinStableFrames: 3, GapToleranceMs: 1500,
		CFARMode: "GOSCA", CFARGuardHz: 15000, CFARTrainHz: 120000,
		CFARGuardCells: 3, CFARTrainCells: 24, CFARRank: 36, CFARScaleDb: 7,
		CFARWrapAround: true, EdgeMarginDb: 4, MaxSignalBwHz: 260000,
		MergeGapHz: 20000, ClassHistorySize: 10, ClassSwitchRatio: 0.6,
	}
}

// baseScene is the fixed signal layout; SNR is applied per run.
func baseScene(snrDb float64, seed int64) synth.Scene {
	mk := func(k synth.Kind, c, bw float64) synth.SignalSpec {
		return synth.SignalSpec{Kind: k, CenterHz: c, BandwidthHz: bw, SNRdB: snrDb}
	}
	return synth.Scene{
		SampleRate: benchFs, Seed: seed, NoiseStd: 1.0,
		Signals: []synth.SignalSpec{
			mk(synth.KindWFM, 800e3, 180e3),
			mk(synth.KindWFM, -800e3, 180e3),
			mk(synth.KindNFM, 200e3, 12e3),
			mk(synth.KindNFM, -300e3, 12e3),
			mk(synth.KindAM, 400e3, 8e3),
			mk(synth.KindSSB, -500e3, 3e3),
			mk(synth.KindCW, 50e3, 100),
			mk(synth.KindDigital, -100e3, 25e3),
		},
	}
}

// runDetector feeds benchFrames noisy realizations of the scene through the
// detector (advancing time so the EMA settles) and returns the last frame's raw
// signals.
func runDetector(d *detector.Detector, scene synth.Scene) []detector.Signal {
	win := fftutil.Hann(benchN)
	var raw []detector.Signal
	now := time.Unix(0, 0)
	for f := 0; f < benchFrames; f++ {
		s := scene
		s.Seed = scene.Seed + int64(f)
		spec := fftutil.Spectrum(s.Generate(benchN), win)
		now = now.Add(66 * time.Millisecond)
		_, raw = d.Process(now, spec, 0)
	}
	return raw
}

type metrics struct {
	tp, fp, fn int
	bwErrPct   []float64
	centerErr  []float64
}

func score(truth []synth.SignalSpec, got []detector.Signal, binWidth float64) metrics {
	used := make([]bool, len(got))
	var m metrics
	for _, t := range truth {
		bestIdx, bestDist := -1, math.MaxFloat64
		for i, g := range got {
			if used[i] {
				continue
			}
			tol := math.Max(t.BandwidthHz, g.BWHz)/2 + 2*binWidth
			d := math.Abs(g.CenterHz - t.CenterHz)
			if d <= tol && d < bestDist {
				bestIdx, bestDist = i, d
			}
		}
		if bestIdx < 0 {
			m.fn++
			continue
		}
		used[bestIdx] = true
		m.tp++
		g := got[bestIdx]
		if t.BandwidthHz > 0 {
			m.bwErrPct = append(m.bwErrPct, math.Abs(g.BWHz-t.BandwidthHz)/t.BandwidthHz*100)
		}
		m.centerErr = append(m.centerErr, math.Abs(g.CenterHz-t.CenterHz))
	}
	for i := range got {
		if !used[i] {
			m.fp++
		}
	}
	return m
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func medianf(v []float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	return c[len(c)/2]
}

func TestDetectionBaseline(t *testing.T) {
	binWidth := float64(benchFs) / float64(benchN)
	snrs := []float64{5, 10, 20, 30}

	t.Logf("Detection baseline — current pipeline (binWidth=%.0f Hz, %d truth signals/scene)", binWidth, len(baseScene(0, 0).Signals))
	t.Logf("%-6s %8s %8s %8s %10s %10s %8s", "SNRdB", "detect", "precis", "recall", "bwMAE%", "bwMed%", "ctrHz")

	for _, snr := range snrs {
		d := detector.New(detectorConfig(), benchFs, benchN)
		scene := baseScene(snr, 1000)
		got := runDetector(d, scene)
		m := score(scene.Signals, got, binWidth)

		prec := safeDiv(m.tp, m.tp+m.fp)
		rec := safeDiv(m.tp, m.tp+m.fn)
		t.Logf("%-6.0f %8d %8.2f %8.2f %10.1f %10.1f %8.0f",
			snr, len(got), prec, rec, mean(m.bwErrPct), medianf(m.bwErrPct), mean(m.centerErr))
	}

	// Per-kind bandwidth error at high SNR (where detection is easy) to expose
	// the geometric-bandwidth weakness regardless of detection misses.
	t.Logf("")
	t.Logf("Per-kind bandwidth error @ 30 dB SNR:")
	t.Logf("%-8s %10s %10s %10s", "kind", "truthBW", "detBW", "err%")
	d := detector.New(detectorConfig(), benchFs, benchN)
	scene := baseScene(30, 2000)
	got := runDetector(d, scene)
	for _, tr := range scene.Signals {
		best, bestD := -1, math.MaxFloat64
		for i, g := range got {
			dd := math.Abs(g.CenterHz - tr.CenterHz)
			if dd < bestD {
				best, bestD = i, dd
			}
		}
		if best < 0 || bestD > math.Max(tr.BandwidthHz, got[best].BWHz)/2+4*binWidth {
			t.Logf("%-8s %10.0f %10s %10s", tr.Kind, tr.BandwidthHz, "MISS", "-")
			continue
		}
		g := got[best]
		t.Logf("%-8s %10.0f %10.0f %10.1f", tr.Kind, tr.BandwidthHz, g.BWHz,
			math.Abs(g.BWHz-tr.BandwidthHz)/tr.BandwidthHz*100)
	}
}

// refTruthBW measures a signal's true occupied bandwidth from a near-noiseless,
// wide-region generation. This is the fair ground truth (the nominal Carson
// value overstates the 99%-occupied band for FM), used to score both the
// geometric and the refined estimate on equal footing.
func refTruthBW(kind synth.Kind, nominalBw float64) float64 {
	binWidth := float64(benchFs) / float64(benchN)
	scene := synth.Scene{
		SampleRate: benchFs, Seed: 7, NoiseStd: 1.0,
		Signals: []synth.SignalSpec{{Kind: kind, CenterHz: 0, BandwidthHz: nominalBw, SNRdB: 50}},
	}
	spec := fftutil.Spectrum(scene.Generate(benchN), fftutil.Hann(benchN))
	center := benchN / 2
	half := int(math.Max(4*nominalBw, 60e3) / binWidth)
	lo, hi := center-half, center+half
	if lo < 0 {
		lo = 0
	}
	if hi >= len(spec) {
		hi = len(spec) - 1
	}
	occ := estimate.OccupiedBandwidthDb(spec[lo:hi+1], binWidth, 0.99)
	return occ.BandwidthHz
}

// TestRefinedBandwidthVsGeometric proves R1: the occupied-bandwidth estimator
// recovers the true occupied bandwidth far better than the detector's geometric
// width, on the same scenes, scored against a noiseless reference truth.
func TestRefinedBandwidthVsGeometric(t *testing.T) {
	binWidth := float64(benchFs) / float64(benchN)
	win := fftutil.Hann(benchN)
	d := detector.New(detectorConfig(), benchFs, benchN)
	scene := baseScene(30, 2000)

	// Run frames; keep the final realization's spectrum for refinement.
	var raw []detector.Signal
	var spec []float64
	now := time.Unix(0, 0)
	for f := 0; f < benchFrames; f++ {
		s := scene
		s.Seed = scene.Seed + int64(f)
		spec = fftutil.Spectrum(s.Generate(benchN), win)
		now = now.Add(66 * time.Millisecond)
		_, raw = d.Process(now, spec, 0)
	}

	t.Logf("Refined vs geometric bandwidth @ 30 dB SNR (vs noiseless reference truth):")
	t.Logf("%-8s %9s %9s %9s %9s %9s", "kind", "refTruth", "geomBW", "geom%", "refBW", "ref%")
	var geomErrs, refErrs []float64
	var wfmRefErr float64
	for _, tr := range scene.Signals {
		best, bestD := -1, math.MaxFloat64
		for i, g := range raw {
			dd := math.Abs(g.CenterHz - tr.CenterHz)
			if dd < bestD {
				best, bestD = i, dd
			}
		}
		if best < 0 || bestD > math.Max(tr.BandwidthHz, raw[best].BWHz)/2+4*binWidth {
			t.Logf("%-8s %9s", tr.Kind, "MISS")
			continue
		}
		truth := refTruthBW(tr.Kind, tr.BandwidthHz)
		if truth <= 0 {
			continue
		}
		g := raw[best]
		ref := estimate.RefineFromSpectrum(spec, g.FirstBin, g.LastBin, binWidth, 0.99)
		geomErr := math.Abs(g.BWHz-truth) / truth * 100
		refErr := math.NaN()
		refBW := math.NaN()
		if ref.OK {
			refBW = ref.BandwidthHz
			refErr = math.Abs(ref.BandwidthHz-truth) / truth * 100
		}
		t.Logf("%-8s %9.0f %9.0f %9.1f %9.0f %9.1f", tr.Kind, truth, g.BWHz, geomErr, refBW, refErr)
		// Exclude the sub-bin CW signal from aggregate bandwidth stats.
		if tr.Kind != synth.KindCW {
			geomErrs = append(geomErrs, geomErr)
			if ref.OK {
				refErrs = append(refErrs, refErr)
			}
		}
		if tr.Kind == synth.KindWFM && ref.OK {
			wfmRefErr = refErr
		}
	}

	geomMed := medianf(geomErrs)
	refMed := medianf(refErrs)
	t.Logf("median bw error vs reference truth (excl. CW): geometric=%.1f%%  refined=%.1f%%", geomMed, refMed)

	if refMed > 0.7*geomMed {
		t.Errorf("refined median bw error %.1f%% should be substantially better than geometric %.1f%%", refMed, geomMed)
	}
	if refMed > 30 {
		t.Errorf("refined median bw error %.1f%% should be under 30%%", refMed)
	}
	if wfmRefErr > 35 {
		t.Errorf("refined WFM bw error %.1f%% should be under 35%% (geometric was ~48%%)", wfmRefErr)
	}
}

func safeDiv(a, b int) float64 {
	if b == 0 {
		return math.NaN()
	}
	return float64(a) / float64(b)
}

var _ = fmt.Sprintf

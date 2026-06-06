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

func safeDiv(a, b int) float64 {
	if b == 0 {
		return math.NaN()
	}
	return float64(a) / float64(b)
}

var _ = fmt.Sprintf

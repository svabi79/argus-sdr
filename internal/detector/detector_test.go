package detector

import (
	"testing"
	"time"

	"sdr-visual-suite/internal/config"
)

func TestDetectorCreatesEvent(t *testing.T) {
	d := New(config.DetectorConfig{
		ThresholdDb:     -10,
		MinDurationMs:   1,
		HoldMs:          10,
		EmaAlpha:        0.2,
		HysteresisDb:    3,
		MinStableFrames: 1,
		GapToleranceMs:  10,
		CFARMode:        "OFF",
		CFARGuardCells:  2,
		CFARTrainCells:  16,
		CFARRank:        24,
		CFARScaleDb:     6,
		CFARWrapAround:  true,
	}, 1000, 10)
	center := 0.0
	spectrum := []float64{-30, -30, -30, -5, -5, -30, -30, -30, -30, -30}
	now := time.Now()
	finished, signals := d.Process(now, spectrum, center)
	if len(finished) != 0 {
		t.Fatalf("expected no finished events yet")
	}
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].BWHz <= 0 {
		t.Fatalf("expected bandwidth > 0")
	}

	_, _ = d.Process(now.Add(5*time.Millisecond), spectrum, center)

	now2 := now.Add(30 * time.Millisecond)
	noSignal := make([]float64, len(spectrum))
	for i := range noSignal {
		noSignal[i] = -100
	}
	finished, _ = d.Process(now2, noSignal, center)
	if len(finished) != 1 {
		t.Fatalf("expected 1 finished event, got %d", len(finished))
	}
	if finished[0].Bandwidth <= 0 {
		t.Fatalf("event bandwidth not set")
	}
}

func TestSignalBandwidthExpansion(t *testing.T) {
	sampleRate := 2048000
	fftSize := 2048
	cfg := config.DetectorConfig{
		ThresholdDb:     -20,
		MinDurationMs:   1,
		HoldMs:          10,
		EmaAlpha:        1.0,
		HysteresisDb:    3,
		MinStableFrames: 1,
		GapToleranceMs:  10,
		CFARMode:        "OFF",
		CFARGuardCells:  2,
		CFARTrainCells:  8,
		CFARRank:        12,
		CFARScaleDb:     6,
		CFARWrapAround:  true,
		EdgeMarginDb:    3.0,
		MaxSignalBwHz:   150000,
	}
	d := New(cfg, sampleRate, fftSize)
	spectrum := make([]float64, fftSize)
	for i := range spectrum {
		spectrum[i] = -100
	}
	for i := 1000; i <= 1012; i++ {
		spectrum[i] = -20
	}
	for j := 1; j <= 5; j++ {
		level := -20 - float64(j)*3
		if 1000-j >= 0 {
			spectrum[1000-j] = level
		}
		if 1012+j < fftSize {
			spectrum[1012+j] = level
		}
	}
	now := time.Now()
	_, signals := d.Process(now, spectrum, 434e6)
	if len(signals) == 0 {
		t.Fatal("no signals detected")
	}
	sig := signals[0]
	expectedMinBW := 18.0 * 1000
	if sig.BWHz < expectedMinBW {
		t.Errorf("BW too narrow: got %.0f Hz, want >= %.0f Hz (FirstBin=%d LastBin=%d)", sig.BWHz, expectedMinBW, sig.FirstBin, sig.LastBin)
	}
}

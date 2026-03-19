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

	// Extend signal duration.
	_, _ = d.Process(now.Add(5*time.Millisecond), spectrum, center)

	// Advance beyond hold with no signal to finalize.
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

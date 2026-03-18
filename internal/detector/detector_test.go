package detector

import (
	"testing"
	"time"
)

func TestDetectorCreatesEvent(t *testing.T) {
	d := New(-10, 1000, 10, 1*time.Millisecond, 10*time.Millisecond, 0.2, 3, 1, 10*time.Millisecond)
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

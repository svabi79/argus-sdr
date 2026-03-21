package main

import "testing"

func TestPhaseSnapshotSetGet(t *testing.T) {
	snap := &phaseSnapshot{}
	state := phaseState{}
	state.surveillance.NoiseFloor = -91
	state.refinementInput.SampleRate = 2048000
	snap.Set(state)
	got := snap.Snapshot()
	if got.surveillance.NoiseFloor != -91 {
		t.Fatalf("unexpected noise floor: %v", got.surveillance.NoiseFloor)
	}
	if got.refinementInput.SampleRate != 2048000 {
		t.Fatalf("unexpected sample rate: %v", got.refinementInput.SampleRate)
	}
}

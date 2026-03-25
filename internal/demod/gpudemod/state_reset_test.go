package gpudemod

import "testing"

func TestResetCPUOracleStateIfConfigChanged(t *testing.T) {
	state := &CPUOracleState{
		SignalID:       1,
		ConfigHash:     111,
		NCOPhase:       1.23,
		Decim:          20,
		PhaseCount:     7,
		NumTaps:        65,
		ShiftedHistory: []complex64{1 + 1i, 2 + 2i},
	}
	ResetCPUOracleStateIfConfigChanged(state, 222)
	if state.ConfigHash != 222 {
		t.Fatalf("config hash not updated")
	}
	if state.NCOPhase != 0 {
		t.Fatalf("expected phase reset")
	}
	if state.PhaseCount != 0 {
		t.Fatalf("expected phase count reset")
	}
	if len(state.ShiftedHistory) != 0 {
		t.Fatalf("expected shifted history reset")
	}
}

func TestResetExtractStreamState(t *testing.T) {
	state := &ExtractStreamState{
		SignalID:       1,
		ConfigHash:     111,
		NCOPhase:       2.34,
		Decim:          20,
		PhaseCount:     9,
		NumTaps:        65,
		ShiftedHistory: []complex64{3 + 3i, 4 + 4i},
		Initialized:    true,
	}
	ResetExtractStreamState(state, 333)
	if state.ConfigHash != 333 {
		t.Fatalf("config hash not updated")
	}
	if state.NCOPhase != 0 {
		t.Fatalf("expected phase reset")
	}
	if state.PhaseCount != 0 {
		t.Fatalf("expected phase count reset")
	}
	if len(state.ShiftedHistory) != 0 {
		t.Fatalf("expected shifted history reset")
	}
	if state.Initialized {
		t.Fatalf("expected initialized=false after reset")
	}
}

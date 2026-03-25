package gpudemod

import "testing"

func TestCPUOracleRunnerCleansUpDisappearedSignals(t *testing.T) {
	r := NewCPUOracleRunner(4000000)
	jobs1 := []StreamingExtractJob{
		{SignalID: 1, OffsetHz: 1000, Bandwidth: 20000, OutRate: 200000, NumTaps: 65, ConfigHash: 101},
		{SignalID: 2, OffsetHz: 2000, Bandwidth: 20000, OutRate: 200000, NumTaps: 65, ConfigHash: 102},
	}
	_, err := r.StreamingExtract(makeDeterministicIQ(4096), jobs1)
	if err != nil {
		t.Fatalf("unexpected error on first extract: %v", err)
	}
	if len(r.States) != 2 {
		t.Fatalf("expected 2 states, got %d", len(r.States))
	}
	jobs2 := []StreamingExtractJob{
		{SignalID: 2, OffsetHz: 2000, Bandwidth: 20000, OutRate: 200000, NumTaps: 65, ConfigHash: 102},
	}
	_, err = r.StreamingExtract(makeDeterministicIQ(2048), jobs2)
	if err != nil {
		t.Fatalf("unexpected error on second extract: %v", err)
	}
	if len(r.States) != 1 {
		t.Fatalf("expected 1 state after cleanup, got %d", len(r.States))
	}
	if _, ok := r.States[1]; ok {
		t.Fatalf("expected signal 1 state to be cleaned up")
	}
}

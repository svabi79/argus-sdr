package gpudemod

import "testing"

func TestStreamingGPUUsesSafeProductionDefault(t *testing.T) {
	r := &BatchRunner{eng: &Engine{sampleRate: 4000000}, streamState: make(map[int64]*ExtractStreamState)}
	job := StreamingExtractJob{
		SignalID:   1,
		OffsetHz:   12500,
		Bandwidth:  20000,
		OutRate:    200000,
		NumTaps:    65,
		ConfigHash: 777,
	}
	iq := makeDeterministicIQ(1000)
	results, err := r.StreamingExtractGPU(iq, []StreamingExtractJob{job})
	if err != nil {
		t.Fatalf("expected safe production default path, got error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].NOut == 0 {
		t.Fatalf("expected non-zero output count from safe production path")
	}
}

func TestStreamingGPUHostOracleAdvancesState(t *testing.T) {
	r := &BatchRunner{eng: &Engine{sampleRate: 4000000}, streamState: make(map[int64]*ExtractStreamState)}
	job := StreamingExtractJob{
		SignalID:   1,
		OffsetHz:   12500,
		Bandwidth:  20000,
		OutRate:    200000,
		NumTaps:    65,
		ConfigHash: 777,
	}
	iq := makeDeterministicIQ(1000)
	results, err := r.StreamingExtractGPUHostOracle(iq, []StreamingExtractJob{job})
	if err != nil {
		t.Fatalf("unexpected host-oracle error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	state := r.streamState[1]
	if state == nil {
		t.Fatalf("expected state to be initialized")
	}
	if state.NCOPhase == 0 {
		t.Fatalf("expected phase to advance")
	}
	if len(state.ShiftedHistory) == 0 {
		t.Fatalf("expected shifted history to be updated")
	}
	if results[0].NOut == 0 {
		t.Fatalf("expected non-zero output count from host oracle path")
	}
}

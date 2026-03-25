package gpudemod

import "testing"

func TestCompareOracleAndGPUStub(t *testing.T) {
	oracle := StreamingExtractResult{
		SignalID:   1,
		IQ:         []complex64{1 + 1i, 2 + 2i},
		Rate:       200000,
		NOut:       2,
		PhaseCount: 0,
		HistoryLen: 64,
	}
	gpu := StreamingExtractResult{
		SignalID:   1,
		IQ:         []complex64{1 + 1i, 2.1 + 2i},
		Rate:       200000,
		NOut:       2,
		PhaseCount: 3,
		HistoryLen: 64,
	}
	metrics, stats := CompareOracleAndGPUStub(oracle, gpu)
	if metrics.SignalID != 1 {
		t.Fatalf("unexpected signal id: %d", metrics.SignalID)
	}
	if stats.Count != 2 {
		t.Fatalf("unexpected compare count: %d", stats.Count)
	}
	if metrics.RefMaxAbsErr <= 0 {
		t.Fatalf("expected positive max abs error")
	}
}

package gpudemod

import "testing"

func requireComplexSlicesCloseHarness(t *testing.T, a []complex64, b []complex64, tol float64) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("length mismatch: %d vs %d", len(a), len(b))
	}
	for i := range a {
		d := CompareComplexSlices([]complex64{a[i]}, []complex64{b[i]})
		if d.MaxAbsErr > tol {
			t.Fatalf("slice mismatch at %d: %v vs %v (tol=%f)", i, a[i], b[i], tol)
		}
	}
}

func TestHarnessChunkedCPUOraclePolyphase(t *testing.T) {
	cfg := OracleHarnessConfig{
		SignalID:   1,
		ConfigHash: 123,
		NCOPhase:   0,
		Decim:      20,
		NumTaps:    65,
		PhaseInc:   0.017,
	}
	iq := MakeDeterministicIQ(150000)
	mk := func() *CPUOracleState { return MakeCPUOracleState(cfg) }
	mono := CPUOracleExtractPolyphase(iq, mk(), cfg.PhaseInc)
	chunked := RunChunkedCPUOraclePolyphase(iq, []int{4096, 5000, 8192, 27307}, mk, cfg.PhaseInc)
	requireComplexSlicesCloseHarness(t, mono, chunked, 1e-5)
}

func TestHarnessToneIQ(t *testing.T) {
	iq := MakeToneIQ(1024, 0.05)
	if len(iq) != 1024 {
		t.Fatalf("unexpected tone iq length: %d", len(iq))
	}
}

package gpudemod

import "testing"

func TestRunStreamingPolyphaseHostCoreMatchesCPUOraclePolyphase(t *testing.T) {
	cfg := OracleHarnessConfig{
		SignalID:   1,
		ConfigHash: 123,
		NCOPhase:   0,
		Decim:      20,
		NumTaps:    65,
		PhaseInc:   0.017,
	}
	state := MakeCPUOracleState(cfg)
	iq := MakeDeterministicIQ(12000)
	oracle := CPUOracleExtractPolyphase(iq, state, cfg.PhaseInc)

	state2 := MakeCPUOracleState(cfg)
	out, phase, phaseCount, hist := runStreamingPolyphaseHostCore(
		iq,
		4000000,
		-cfg.PhaseInc*4000000/(2*3.141592653589793),
		state2.NCOPhase,
		state2.PhaseCount,
		state2.NumTaps,
		state2.Decim,
		state2.ShiftedHistory,
		state2.PolyphaseTaps,
	)
	requireComplexSlicesClose(t, oracle, out, 1e-5)
	if phase == 0 && len(iq) > 0 {
		t.Fatalf("expected phase to advance")
	}
	if phaseCount < 0 || phaseCount >= state2.Decim {
		t.Fatalf("unexpected phaseCount: %d", phaseCount)
	}
	if len(hist) == 0 {
		t.Fatalf("expected history to be retained")
	}
}

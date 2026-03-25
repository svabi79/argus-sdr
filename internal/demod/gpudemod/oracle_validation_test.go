package gpudemod

import "testing"

func TestCPUOracleMonolithicVsChunkedPolyphase(t *testing.T) {
	iq := makeDeterministicIQ(120000)
	mk := func() *CPUOracleState {
		taps := makeLowpassTaps(65)
		return &CPUOracleState{
			SignalID:       1,
			ConfigHash:     999,
			NCOPhase:       0,
			Decim:          20,
			PhaseCount:     0,
			NumTaps:        65,
			ShiftedHistory: make([]complex64, 0, 64),
			BaseTaps:       taps,
			PolyphaseTaps:  BuildPolyphaseTapsPhaseMajor(taps, 20),
		}
	}
	phaseInc := 0.013
	mono := CPUOracleExtractPolyphase(iq, mk(), phaseInc)
	chunked := func() []complex64 {
		state := mk()
		out := make([]complex64, 0)
		chunks := []int{4096, 3000, 8192, 7777, 12000}
		pos := 0
		for _, n := range chunks {
			if pos >= len(iq) {
				break
			}
			end := pos + n
			if end > len(iq) {
				end = len(iq)
			}
			out = append(out, CPUOracleExtractPolyphase(iq[pos:end], state, phaseInc)...)
			pos = end
		}
		if pos < len(iq) {
			out = append(out, CPUOracleExtractPolyphase(iq[pos:], state, phaseInc)...)
		}
		return out
	}()
	requireComplexSlicesClose(t, mono, chunked, 1e-5)
}

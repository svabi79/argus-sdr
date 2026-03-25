package gpudemod

import (
	"math"
)

type OracleHarnessConfig struct {
	SignalID   int64
	ConfigHash uint64
	NCOPhase   float64
	Decim      int
	NumTaps    int
	PhaseInc   float64
}

func MakeDeterministicIQ(n int) []complex64 {
	out := make([]complex64, n)
	for i := 0; i < n; i++ {
		a := 0.017 * float64(i)
		b := 0.031 * float64(i)
		out[i] = complex64(complex(math.Cos(a)+0.2*math.Cos(b), math.Sin(a)+0.15*math.Sin(b)))
	}
	return out
}

func MakeToneIQ(n int, phaseInc float64) []complex64 {
	out := make([]complex64, n)
	phase := 0.0
	for i := 0; i < n; i++ {
		out[i] = complex64(complex(math.Cos(phase), math.Sin(phase)))
		phase += phaseInc
	}
	return out
}

func MakeLowpassTaps(n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = 1.0 / float32(n)
	}
	return out
}

func MakeCPUOracleState(cfg OracleHarnessConfig) *CPUOracleState {
	taps := MakeLowpassTaps(cfg.NumTaps)
	return &CPUOracleState{
		SignalID:       cfg.SignalID,
		ConfigHash:     cfg.ConfigHash,
		NCOPhase:       cfg.NCOPhase,
		Decim:          cfg.Decim,
		PhaseCount:     0,
		NumTaps:        cfg.NumTaps,
		ShiftedHistory: make([]complex64, 0, maxInt(0, cfg.NumTaps-1)),
		BaseTaps:       taps,
		PolyphaseTaps:  BuildPolyphaseTapsPhaseMajor(taps, cfg.Decim),
	}
}

func RunChunkedCPUOraclePolyphase(all []complex64, chunkSizes []int, mkState func() *CPUOracleState, phaseInc float64) []complex64 {
	state := mkState()
	out := make([]complex64, 0)
	pos := 0
	for _, n := range chunkSizes {
		if pos >= len(all) {
			break
		}
		end := pos + n
		if end > len(all) {
			end = len(all)
		}
		out = append(out, CPUOracleExtractPolyphase(all[pos:end], state, phaseInc)...)
		pos = end
	}
	if pos < len(all) {
		out = append(out, CPUOracleExtractPolyphase(all[pos:], state, phaseInc)...)
	}
	return out
}

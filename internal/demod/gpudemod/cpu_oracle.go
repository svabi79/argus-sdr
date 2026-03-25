package gpudemod

import (
	"fmt"
	"math"
)

type CPUOracleState struct {
	SignalID       int64
	ConfigHash     uint64
	NCOPhase       float64
	Decim          int
	PhaseCount     int
	NumTaps        int
	ShiftedHistory []complex64
	BaseTaps       []float32
	PolyphaseTaps  []float32
}

func ResetCPUOracleStateIfConfigChanged(state *CPUOracleState, newHash uint64) {
	if state == nil {
		return
	}
	if state.ConfigHash != newHash {
		state.ConfigHash = newHash
		state.NCOPhase = 0
		state.PhaseCount = 0
		state.ShiftedHistory = state.ShiftedHistory[:0]
	}
}

func CPUOracleExtract(iqNew []complex64, state *CPUOracleState, phaseInc float64) []complex64 {
	if state == nil || state.NumTaps <= 0 || state.Decim <= 0 || len(state.BaseTaps) < state.NumTaps {
		return nil
	}
	out := make([]complex64, 0, len(iqNew)/maxInt(1, state.Decim)+2)
	phase := state.NCOPhase
	hist := append([]complex64(nil), state.ShiftedHistory...)

	for _, x := range iqNew {
		rot := complex64(complex(math.Cos(phase), math.Sin(phase)))
		s := x * rot
		hist = append(hist, s)
		state.PhaseCount++

		if state.PhaseCount == state.Decim {
			var y complex64
			for k := 0; k < state.NumTaps; k++ {
				idx := len(hist) - 1 - k
				var sample complex64
				if idx >= 0 {
					sample = hist[idx]
				}
				y += complex(state.BaseTaps[k], 0) * sample
			}
			out = append(out, y)
			state.PhaseCount = 0
		}

		if len(hist) > state.NumTaps-1 {
			hist = hist[len(hist)-(state.NumTaps-1):]
		}

		phase += phaseInc
		if phase >= math.Pi {
			phase -= 2 * math.Pi
		} else if phase < -math.Pi {
			phase += 2 * math.Pi
		}
	}

	state.NCOPhase = phase
	state.ShiftedHistory = append(state.ShiftedHistory[:0], hist...)
	return out
}

// CPUOracleExtractPolyphase keeps the same streaming state semantics as CPUOracleExtract,
// but computes outputs using the explicit phase-major polyphase tap layout.
func CPUOracleExtractPolyphase(iqNew []complex64, state *CPUOracleState, phaseInc float64) []complex64 {
	if state == nil || state.NumTaps <= 0 || state.Decim <= 0 || len(state.BaseTaps) < state.NumTaps {
		return nil
	}
	if len(state.PolyphaseTaps) == 0 {
		state.PolyphaseTaps = BuildPolyphaseTapsPhaseMajor(state.BaseTaps, state.Decim)
	}
	phaseLen := PolyphasePhaseLen(len(state.BaseTaps), state.Decim)
	out := make([]complex64, 0, len(iqNew)/maxInt(1, state.Decim)+2)
	phase := state.NCOPhase
	hist := append([]complex64(nil), state.ShiftedHistory...)

	for _, x := range iqNew {
		rot := complex64(complex(math.Cos(phase), math.Sin(phase)))
		s := x * rot
		hist = append(hist, s)
		state.PhaseCount++

		if state.PhaseCount == state.Decim {
			var y complex64
			for p := 0; p < state.Decim; p++ {
				for k := 0; k < phaseLen; k++ {
					tap := state.PolyphaseTaps[p*phaseLen+k]
					if tap == 0 {
						continue
					}
					srcBack := p + k*state.Decim
					idx := len(hist) - 1 - srcBack
					if idx < 0 {
						continue
					}
					y += complex(tap, 0) * hist[idx]
				}
			}
			out = append(out, y)
			state.PhaseCount = 0
		}

		if len(hist) > state.NumTaps-1 {
			hist = hist[len(hist)-(state.NumTaps-1):]
		}

		phase += phaseInc
		if phase >= math.Pi {
			phase -= 2 * math.Pi
		} else if phase < -math.Pi {
			phase += 2 * math.Pi
		}
	}

	state.NCOPhase = phase
	state.ShiftedHistory = append(state.ShiftedHistory[:0], hist...)
	return out
}

func RunChunkedCPUOracle(all []complex64, chunkSizes []int, mkState func() *CPUOracleState, phaseInc float64) []complex64 {
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
		out = append(out, CPUOracleExtract(all[pos:end], state, phaseInc)...)
		pos = end
	}
	if pos < len(all) {
		out = append(out, CPUOracleExtract(all[pos:], state, phaseInc)...)
	}
	return out
}

func ExactIntegerDecimation(sampleRate int, outRate int) (int, error) {
	if sampleRate <= 0 || outRate <= 0 {
		return 0, fmt.Errorf("invalid sampleRate/outRate: %d/%d", sampleRate, outRate)
	}
	if sampleRate%outRate != 0 {
		return 0, fmt.Errorf("streaming polyphase extractor requires integer decimation: sampleRate=%d outRate=%d", sampleRate, outRate)
	}
	return sampleRate / outRate, nil
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

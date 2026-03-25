package gpudemod

import (
	"fmt"

	"sdr-wideband-suite/internal/dsp"
)

type CPUOracleRunner struct {
	SampleRate int
	States     map[int64]*CPUOracleState
}

func (r *CPUOracleRunner) ResetAllStates() {
	if r == nil {
		return
	}
	r.States = make(map[int64]*CPUOracleState)
}

func NewCPUOracleRunner(sampleRate int) *CPUOracleRunner {
	return &CPUOracleRunner{
		SampleRate: sampleRate,
		States:     make(map[int64]*CPUOracleState),
	}
}

func (r *CPUOracleRunner) ResetSignalState(signalID int64) {
	if r == nil || r.States == nil {
		return
	}
	delete(r.States, signalID)
}

func (r *CPUOracleRunner) getOrInitState(job StreamingExtractJob) (*CPUOracleState, error) {
	if r == nil {
		return nil, fmt.Errorf("nil CPUOracleRunner")
	}
	if r.States == nil {
		r.States = make(map[int64]*CPUOracleState)
	}
	decim, err := ExactIntegerDecimation(r.SampleRate, job.OutRate)
	if err != nil {
		return nil, err
	}
	state := r.States[job.SignalID]
	if state == nil {
		state = &CPUOracleState{SignalID: job.SignalID}
		r.States[job.SignalID] = state
	}
	ResetCPUOracleStateIfConfigChanged(state, job.ConfigHash)
	state.Decim = decim
	state.NumTaps = job.NumTaps
	if state.NumTaps <= 0 {
		state.NumTaps = 101
	}
	cutoff := job.Bandwidth / 2
	if cutoff < 200 {
		cutoff = 200
	}
	base := dsp.LowpassFIR(cutoff, r.SampleRate, state.NumTaps)
	state.BaseTaps = make([]float32, len(base))
	for i, v := range base {
		state.BaseTaps[i] = float32(v)
	}
	state.PolyphaseTaps = BuildPolyphaseTapsPhaseMajor(state.BaseTaps, state.Decim)
	if state.ShiftedHistory == nil {
		state.ShiftedHistory = make([]complex64, 0, maxInt(0, state.NumTaps-1))
	}
	return state, nil
}

func (r *CPUOracleRunner) StreamingExtract(iqNew []complex64, jobs []StreamingExtractJob) ([]StreamingExtractResult, error) {
	results := make([]StreamingExtractResult, len(jobs))
	active := make(map[int64]struct{}, len(jobs))
	for i, job := range jobs {
		active[job.SignalID] = struct{}{}
		state, err := r.getOrInitState(job)
		if err != nil {
			return nil, err
		}
		out, phase, phaseCount, hist := runStreamingPolyphaseHostCore(
			iqNew,
			r.SampleRate,
			job.OffsetHz,
			state.NCOPhase,
			state.PhaseCount,
			state.NumTaps,
			state.Decim,
			state.ShiftedHistory,
			state.PolyphaseTaps,
		)
		state.NCOPhase = phase
		state.PhaseCount = phaseCount
		state.ShiftedHistory = append(state.ShiftedHistory[:0], hist...)
		results[i] = StreamingExtractResult{
			SignalID:   job.SignalID,
			IQ:         out,
			Rate:       job.OutRate,
			NOut:       len(out),
			PhaseCount: state.PhaseCount,
			HistoryLen: len(state.ShiftedHistory),
		}
	}
	for signalID := range r.States {
		if _, ok := active[signalID]; !ok {
			delete(r.States, signalID)
		}
	}
	return results, nil
}

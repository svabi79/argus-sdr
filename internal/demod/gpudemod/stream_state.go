package gpudemod

import "sdr-wideband-suite/internal/dsp"

func (r *BatchRunner) ResetSignalState(signalID int64) {
	if r == nil || r.streamState == nil {
		return
	}
	delete(r.streamState, signalID)
}

func (r *BatchRunner) ResetAllSignalStates() {
	if r == nil {
		return
	}
	r.streamState = make(map[int64]*ExtractStreamState)
}

func (r *BatchRunner) getOrInitExtractState(job StreamingExtractJob, sampleRate int) (*ExtractStreamState, error) {
	if r == nil {
		return nil, ErrUnavailable
	}
	if r.streamState == nil {
		r.streamState = make(map[int64]*ExtractStreamState)
	}
	decim, err := ExactIntegerDecimation(sampleRate, job.OutRate)
	if err != nil {
		return nil, err
	}
	state := r.streamState[job.SignalID]
	if state == nil {
		state = &ExtractStreamState{SignalID: job.SignalID}
		r.streamState[job.SignalID] = state
	}
	if state.ConfigHash != job.ConfigHash {
		ResetExtractStreamState(state, job.ConfigHash)
	}
	state.Decim = decim
	state.NumTaps = job.NumTaps
	if state.NumTaps <= 0 {
		state.NumTaps = 101
	}
	cutoff := job.Bandwidth / 2
	if cutoff < 200 {
		cutoff = 200
	}
	base := dsp.LowpassFIR(cutoff, sampleRate, state.NumTaps)
	state.BaseTaps = make([]float32, len(base))
	for i, v := range base {
		state.BaseTaps[i] = float32(v)
	}
	state.PolyphaseTaps = BuildPolyphaseTapsPhaseMajor(state.BaseTaps, state.Decim)
	if cap(state.ShiftedHistory) < maxInt(0, state.NumTaps-1) {
		state.ShiftedHistory = make([]complex64, 0, maxInt(0, state.NumTaps-1))
	} else if state.ShiftedHistory == nil {
		state.ShiftedHistory = make([]complex64, 0, maxInt(0, state.NumTaps-1))
	}
	state.Initialized = true
	return state, nil
}

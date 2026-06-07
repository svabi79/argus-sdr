package gpudemod

import (
	"log"

	"sdr-wideband-suite/internal/dsp"
)

func (r *BatchRunner) ResetSignalState(signalID int64) {
	if r == nil || r.streamState == nil {
		return
	}
	delete(r.streamState, signalID)
	r.resetNativeStreamingState(signalID)
}

func (r *BatchRunner) ResetAllSignalStates() {
	if r == nil {
		return
	}
	r.streamState = make(map[int64]*ExtractStreamState)
	r.resetAllNativeStreamingStates()
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
		if state.Initialized {
			log.Printf("STREAMING STATE RESET: signal=%d oldHash=%d newHash=%d historyLen=%d",
				job.SignalID, state.ConfigHash, job.ConfigHash, len(state.ShiftedHistory))
		}
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
	// Rebuild taps only when a tap-relevant input changes (#20 / OI-07). Identical
	// inputs produce identical taps, so the cached path is byte-for-byte equivalent
	// to rebuilding every frame; it just skips the per-frame LowpassFIR + polyphase
	// allocation for the (common) steady signal.
	if state.BaseTaps == nil || cutoff != state.tapsCutoff || state.NumTaps != state.tapsNumTaps ||
		state.Decim != state.tapsDecim || sampleRate != state.tapsSampleRate {
		base := dsp.LowpassFIR(cutoff, sampleRate, state.NumTaps)
		if cap(state.BaseTaps) < len(base) {
			state.BaseTaps = make([]float32, len(base))
		} else {
			state.BaseTaps = state.BaseTaps[:len(base)]
		}
		for i, v := range base {
			state.BaseTaps[i] = float32(v)
		}
		state.PolyphaseTaps = BuildPolyphaseTapsPhaseMajor(state.BaseTaps, state.Decim)
		state.tapsCutoff = cutoff
		state.tapsNumTaps = state.NumTaps
		state.tapsDecim = state.Decim
		state.tapsSampleRate = sampleRate
	}
	if cap(state.ShiftedHistory) < maxInt(0, state.NumTaps-1) {
		state.ShiftedHistory = make([]complex64, 0, maxInt(0, state.NumTaps-1))
	} else if state.ShiftedHistory == nil {
		state.ShiftedHistory = make([]complex64, 0, maxInt(0, state.NumTaps-1))
	}
	state.Initialized = true
	return state, nil
}

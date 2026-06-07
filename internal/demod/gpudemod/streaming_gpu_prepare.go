package gpudemod

func (r *BatchRunner) buildStreamingGPUInvocations(iqNew []complex64, jobs []StreamingExtractJob) ([]StreamingGPUInvocation, error) {
	if r == nil || r.eng == nil {
		return nil, ErrUnavailable
	}
	invocations := make([]StreamingGPUInvocation, len(jobs))
	active := make(map[int64]struct{}, len(jobs))
	for i, job := range jobs {
		active[job.SignalID] = struct{}{}
		state, err := r.getOrInitExtractState(job, r.eng.sampleRate)
		if err != nil {
			return nil, err
		}
		invocations[i] = StreamingGPUInvocation{
			SignalID:     job.SignalID,
			ConfigHash:   state.ConfigHash,
			OffsetHz:     job.OffsetHz,
			OutRate:      job.OutRate,
			Bandwidth:    job.Bandwidth,
			SampleRate:   r.eng.sampleRate,
			NumTaps:      state.NumTaps,
			Decim:        state.Decim,
			PhaseCountIn: state.PhaseCount,
			NCOPhaseIn:   state.NCOPhase,
			HistoryLen:   len(state.ShiftedHistory),
			// Reference the stable per-signal state slices instead of copying them
			// every frame (#20). The native exec path reads these read-only (taps are
			// uploaded to the GPU only on reset) and synchronously, before
			// applyStreamingGPUExecutionResults next mutates ShiftedHistory — same
			// goroutine, so no aliasing. With the tap cache the slices are stable.
			BaseTaps:       state.BaseTaps,
			PolyphaseTaps:  state.PolyphaseTaps,
			ShiftedHistory: state.ShiftedHistory,
			IQNew:          iqNew,
		}
	}
	for signalID := range r.streamState {
		if _, ok := active[signalID]; !ok {
			delete(r.streamState, signalID)
		}
	}
	r.syncNativeStreamingStates(active)
	return invocations, nil
}

func (r *BatchRunner) applyStreamingGPUExecutionResults(results []StreamingGPUExecutionResult) []StreamingExtractResult {
	out := make([]StreamingExtractResult, len(results))
	for i, res := range results {
		state := r.streamState[res.SignalID]
		if state != nil {
			state.NCOPhase = res.NCOPhaseOut
			state.PhaseCount = res.PhaseCountOut
			state.ShiftedHistory = append(state.ShiftedHistory[:0], res.HistoryOut...)
		}
		out[i] = StreamingExtractResult{
			SignalID:   res.SignalID,
			IQ:         res.IQ,
			Rate:       res.Rate,
			NOut:       res.NOut,
			PhaseCount: res.PhaseCountOut,
			HistoryLen: res.HistoryLenOut,
		}
	}
	return out
}

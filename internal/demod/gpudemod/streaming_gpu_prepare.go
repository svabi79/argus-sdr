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
			SignalID:       job.SignalID,
			OffsetHz:       job.OffsetHz,
			OutRate:        job.OutRate,
			Bandwidth:      job.Bandwidth,
			SampleRate:     r.eng.sampleRate,
			NumTaps:        state.NumTaps,
			Decim:          state.Decim,
			PhaseCountIn:   state.PhaseCount,
			NCOPhaseIn:     state.NCOPhase,
			HistoryLen:     len(state.ShiftedHistory),
			BaseTaps:       append([]float32(nil), state.BaseTaps...),
			PolyphaseTaps:  append([]float32(nil), state.PolyphaseTaps...),
			ShiftedHistory: append([]complex64(nil), state.ShiftedHistory...),
			IQNew:          iqNew,
		}
	}
	for signalID := range r.streamState {
		if _, ok := active[signalID]; !ok {
			delete(r.streamState, signalID)
		}
	}
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

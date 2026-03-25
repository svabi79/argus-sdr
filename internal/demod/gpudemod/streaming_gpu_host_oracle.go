package gpudemod

// StreamingExtractGPUHostOracle is a temporary host-side execution of the intended
// streaming semantics using GPU-owned stream state. It is not the final GPU
// production implementation, but it allows the new production entrypoint to move
// from pure stub semantics toward real NEW-samples-only streaming behavior
// without reintroducing overlap+trim.
func (r *BatchRunner) StreamingExtractGPUHostOracle(iqNew []complex64, jobs []StreamingExtractJob) ([]StreamingExtractResult, error) {
	if r == nil || r.eng == nil {
		return nil, ErrUnavailable
	}
	results := make([]StreamingExtractResult, len(jobs))
	active := make(map[int64]struct{}, len(jobs))
	for i, job := range jobs {
		active[job.SignalID] = struct{}{}
		state, err := r.getOrInitExtractState(job, r.eng.sampleRate)
		if err != nil {
			return nil, err
		}
		out, phase, phaseCount, hist := runStreamingPolyphaseHostCore(
			iqNew,
			r.eng.sampleRate,
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
	for signalID := range r.streamState {
		if _, ok := active[signalID]; !ok {
			delete(r.streamState, signalID)
		}
	}
	return results, nil
}

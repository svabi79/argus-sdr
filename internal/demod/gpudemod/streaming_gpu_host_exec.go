package gpudemod

func (r *BatchRunner) executeStreamingGPUHostOraclePrepared(invocations []StreamingGPUInvocation) ([]StreamingGPUExecutionResult, error) {
	results := make([]StreamingGPUExecutionResult, len(invocations))
	for i, inv := range invocations {
		out, phase, phaseCount, hist := runStreamingPolyphaseHostCore(
			inv.IQNew,
			inv.SampleRate,
			inv.OffsetHz,
			inv.NCOPhaseIn,
			inv.PhaseCountIn,
			inv.NumTaps,
			inv.Decim,
			inv.ShiftedHistory,
			inv.PolyphaseTaps,
		)
		results[i] = StreamingGPUExecutionResult{
			SignalID:      inv.SignalID,
			Mode:          StreamingGPUExecHostOracle,
			IQ:            out,
			Rate:          inv.OutRate,
			NOut:          len(out),
			PhaseCountOut: phaseCount,
			NCOPhaseOut:   phase,
			HistoryOut:    hist,
			HistoryLenOut: len(hist),
		}
	}
	return results, nil
}

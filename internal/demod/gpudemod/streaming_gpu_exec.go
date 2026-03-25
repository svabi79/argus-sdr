package gpudemod

// StreamingExtractGPUExec is the internal execution selector for the new
// production-path semantics. It intentionally keeps the public API stable while
// allowing the implementation to evolve from host-side oracle execution toward
// a real GPU polyphase path.
func (r *BatchRunner) StreamingExtractGPUExec(iqNew []complex64, jobs []StreamingExtractJob) ([]StreamingExtractResult, error) {
	invocations, err := r.buildStreamingGPUInvocations(iqNew, jobs)
	if err != nil {
		return nil, err
	}
	if useGPUHostOracleExecution {
		execResults, err := r.executeStreamingGPUHostOraclePrepared(invocations)
		if err != nil {
			return nil, err
		}
		return r.applyStreamingGPUExecutionResults(execResults), nil
	}
	if useGPUNativePreparedExecution {
		execResults, err := r.executeStreamingGPUNativePrepared(invocations)
		if err != nil {
			return nil, err
		}
		return r.applyStreamingGPUExecutionResults(execResults), nil
	}
	return nil, ErrUnavailable
}

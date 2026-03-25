package gpudemod

func updateShiftedHistory(prev []complex64, shiftedNew []complex64, numTaps int) []complex64 {
	need := numTaps - 1
	if need <= 0 {
		return nil
	}
	combined := append(append(make([]complex64, 0, len(prev)+len(shiftedNew)), prev...), shiftedNew...)
	if len(combined) <= need {
		out := make([]complex64, len(combined))
		copy(out, combined)
		return out
	}
	out := make([]complex64, need)
	copy(out, combined[len(combined)-need:])
	return out
}

// StreamingExtractGPU is the production entry point for the stateful streaming
// extractor path. Execution strategy is selected by StreamingExtractGPUExec.
func (r *BatchRunner) StreamingExtractGPU(iqNew []complex64, jobs []StreamingExtractJob) ([]StreamingExtractResult, error) {
	if r == nil || r.eng == nil {
		return nil, ErrUnavailable
	}
	return r.StreamingExtractGPUExec(iqNew, jobs)
}

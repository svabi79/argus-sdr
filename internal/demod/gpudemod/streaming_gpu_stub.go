package gpudemod

import "fmt"

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

// StreamingExtractGPU is the planned production entry point for the stateful
// GPU extractor path. It intentionally exists early as an explicit boundary so
// callers can migrate away from legacy overlap+trim semantics.
//
// Current status:
// - validates jobs against persistent per-signal state ownership
// - enforces exact integer decimation
// - initializes per-signal state (config hash, taps, history capacity)
// - does not yet execute the final stateful polyphase GPU kernel path
func (r *BatchRunner) StreamingExtractGPU(iqNew []complex64, jobs []StreamingExtractJob) ([]StreamingExtractResult, error) {
	if r == nil || r.eng == nil {
		return nil, ErrUnavailable
	}
	if results, err := r.StreamingExtractGPUExec(iqNew, jobs); err == nil {
		return results, nil
	}
	_, _ = iqNew, jobs
	return nil, fmt.Errorf("StreamingExtractGPU not implemented yet: stateful polyphase GPU path pending")
}

//go:build !cufft || !windows

package gpudemod

func (r *BatchRunner) executeStreamingGPUNativePrepared(invocations []StreamingGPUInvocation) ([]StreamingGPUExecutionResult, error) {
	_ = invocations
	return nil, ErrUnavailable
}

//go:build !cufft || !windows

package gpudemod

func (r *BatchRunner) executeStreamingGPUNativePrepared(invocations []StreamingGPUInvocation) ([]StreamingGPUExecutionResult, error) {
	_ = invocations
	return nil, ErrUnavailable
}

func (r *BatchRunner) syncNativeStreamingStates(active map[int64]struct{}) {
	_ = active
	if r == nil {
		return
	}
	if r.nativeState == nil {
		r.nativeState = make(map[int64]*nativeStreamingSignalState)
	}
	for id := range r.nativeState {
		if _, ok := active[id]; !ok {
			delete(r.nativeState, id)
		}
	}
}

func (r *BatchRunner) resetNativeStreamingState(signalID int64) {
	if r == nil || r.nativeState == nil {
		return
	}
	delete(r.nativeState, signalID)
}

func (r *BatchRunner) resetAllNativeStreamingStates() {
	if r == nil {
		return
	}
	r.nativeState = make(map[int64]*nativeStreamingSignalState)
}

func (r *BatchRunner) freeAllNativeStreamingStates() {
	if r == nil {
		return
	}
	r.nativeState = nil
}

//go:build cufft && windows

package gpudemod

/*
#cgo windows CFLAGS: -I"C:/Program Files/NVIDIA GPU Computing Toolkit/CUDA/v13.2/include"
#include <cuda_runtime.h>
typedef struct { float x; float y; } gpud_float2;
*/
import "C"

import (
	"math"
	"unsafe"
)

func (r *BatchRunner) executeStreamingGPUNativePrepared(invocations []StreamingGPUInvocation) ([]StreamingGPUExecutionResult, error) {
	if r == nil || r.eng == nil {
		return nil, ErrUnavailable
	}
	if r.nativeState == nil {
		r.nativeState = make(map[int64]*nativeStreamingSignalState)
	}
	results := make([]StreamingGPUExecutionResult, len(invocations))
	for i, inv := range invocations {
		state, err := r.getOrInitNativeStreamingState(inv)
		if err != nil {
			return nil, err
		}
		if len(inv.IQNew) > 0 {
			if err := ensureNativeBuffer(&state.dInNew, &state.inNewCap, len(inv.IQNew), unsafe.Sizeof(C.gpud_float2{})); err != nil {
				return nil, err
			}
			if bridgeMemcpyH2D(state.dInNew, unsafe.Pointer(&inv.IQNew[0]), uintptr(len(inv.IQNew))*unsafe.Sizeof(complex64(0))) != 0 {
				return nil, ErrUnavailable
			}
		}
		outCap := len(inv.IQNew)/maxInt(1, inv.Decim) + 2
		if outCap > 0 {
			if err := ensureNativeBuffer(&state.dOut, &state.outCap, outCap, unsafe.Sizeof(C.gpud_float2{})); err != nil {
				return nil, err
			}
		}

		phaseInc := -2.0 * math.Pi * inv.OffsetHz / float64(inv.SampleRate)
		// The native export consumes phase carry as host scalars while sample/history
		// buffers remain device-resident, so keep these counters in nativeState.
		var nOut C.int
		historyLen := C.int(state.historyLen)
		phaseCount := C.int(state.phaseCount)
		phaseNCO := C.double(state.phaseNCO)
		res := bridgeLaunchStreamingPolyphaseStateful(
			(*C.gpud_float2)(state.dInNew),
			len(inv.IQNew),
			(*C.gpud_float2)(state.dShifted),
			(*C.float)(state.dTaps),
			state.tapsLen,
			state.decim,
			state.numTaps,
			(*C.gpud_float2)(state.dHistory),
			(*C.gpud_float2)(state.dHistoryScratch),
			state.historyCap,
			&historyLen,
			&phaseCount,
			&phaseNCO,
			phaseInc,
			(*C.gpud_float2)(state.dOut),
			outCap,
			&nOut,
		)
		if res != 0 {
			return nil, ErrUnavailable
		}
		state.historyLen = int(historyLen)
		state.phaseCount = int(phaseCount)
		state.phaseNCO = float64(phaseNCO)

		// Per-signal ring buffer (allocation-free across frames; #20). The snippet
		// crosses the async streamer feed channel, so a single reused buffer would
		// race — the ring (depth streamOutRingDepth) guarantees the producer never
		// overwrites a slot the consumer still holds.
		outHost := r.outRingFor(inv.SignalID).next(int(nOut))
		if len(outHost) > 0 {
			if bridgeMemcpyD2H(unsafe.Pointer(&outHost[0]), state.dOut, uintptr(len(outHost))*unsafe.Sizeof(complex64(0))) != 0 {
				return nil, ErrUnavailable
			}
		}
		histHost := make([]complex64, state.historyLen)
		if state.historyLen > 0 {
			if bridgeMemcpyD2H(unsafe.Pointer(&histHost[0]), state.dHistory, uintptr(state.historyLen)*unsafe.Sizeof(complex64(0))) != 0 {
				return nil, ErrUnavailable
			}
		}

		results[i] = StreamingGPUExecutionResult{
			SignalID:      inv.SignalID,
			Mode:          StreamingGPUExecCUDA,
			IQ:            outHost,
			Rate:          inv.OutRate,
			NOut:          len(outHost),
			PhaseCountOut: state.phaseCount,
			NCOPhaseOut:   state.phaseNCO,
			HistoryOut:    histHost,
			HistoryLenOut: len(histHost),
		}
	}
	return results, nil
}

func (r *BatchRunner) getOrInitNativeStreamingState(inv StreamingGPUInvocation) (*nativeStreamingSignalState, error) {
	state := r.nativeState[inv.SignalID]
	needReset := false
	historyCap := maxInt(0, inv.NumTaps-1)
	if state == nil {
		state = &nativeStreamingSignalState{signalID: inv.SignalID}
		r.nativeState[inv.SignalID] = state
		needReset = true
	}
	if state.configHash != inv.ConfigHash {
		needReset = true
	}
	if state.decim != inv.Decim || state.numTaps != inv.NumTaps || state.tapsLen != len(inv.PolyphaseTaps) {
		needReset = true
	}
	if state.historyCap != historyCap {
		needReset = true
	}
	if needReset {
		releaseNativeStreamingSignalState(state)
	}
	if len(inv.PolyphaseTaps) == 0 {
		return nil, ErrUnavailable
	}
	if state.dTaps == nil && len(inv.PolyphaseTaps) > 0 {
		if bridgeCudaMalloc(&state.dTaps, uintptr(len(inv.PolyphaseTaps))*unsafe.Sizeof(C.float(0))) != 0 {
			return nil, ErrUnavailable
		}
		if bridgeMemcpyH2D(state.dTaps, unsafe.Pointer(&inv.PolyphaseTaps[0]), uintptr(len(inv.PolyphaseTaps))*unsafe.Sizeof(float32(0))) != 0 {
			return nil, ErrUnavailable
		}
		state.tapsLen = len(inv.PolyphaseTaps)
	}
	if state.dShifted == nil {
		minCap := maxInt(1, len(inv.IQNew))
		if bridgeCudaMalloc(&state.dShifted, uintptr(minCap)*unsafe.Sizeof(C.gpud_float2{})) != 0 {
			return nil, ErrUnavailable
		}
		state.shiftedCap = minCap
	}
	if state.shiftedCap < len(inv.IQNew) {
		if bridgeCudaFree(state.dShifted) != 0 {
			return nil, ErrUnavailable
		}
		state.dShifted = nil
		state.shiftedCap = 0
		if bridgeCudaMalloc(&state.dShifted, uintptr(len(inv.IQNew))*unsafe.Sizeof(C.gpud_float2{})) != 0 {
			return nil, ErrUnavailable
		}
		state.shiftedCap = len(inv.IQNew)
	}
	if state.dHistory == nil && historyCap > 0 {
		if bridgeCudaMalloc(&state.dHistory, uintptr(historyCap)*unsafe.Sizeof(C.gpud_float2{})) != 0 {
			return nil, ErrUnavailable
		}
	}
	if state.dHistoryScratch == nil && historyCap > 0 {
		if bridgeCudaMalloc(&state.dHistoryScratch, uintptr(historyCap)*unsafe.Sizeof(C.gpud_float2{})) != 0 {
			return nil, ErrUnavailable
		}
		state.historyScratchCap = historyCap
	}
	if needReset {
		state.phaseCount = inv.PhaseCountIn
		state.phaseNCO = inv.NCOPhaseIn
		state.historyLen = minInt(len(inv.ShiftedHistory), historyCap)
		if state.historyLen > 0 {
			if bridgeMemcpyH2D(state.dHistory, unsafe.Pointer(&inv.ShiftedHistory[len(inv.ShiftedHistory)-state.historyLen]), uintptr(state.historyLen)*unsafe.Sizeof(complex64(0))) != 0 {
				return nil, ErrUnavailable
			}
		}
	}
	state.decim = inv.Decim
	state.numTaps = inv.NumTaps
	state.historyCap = historyCap
	state.historyScratchCap = historyCap
	state.configHash = inv.ConfigHash
	return state, nil
}

func ensureNativeBuffer(ptr *unsafe.Pointer, capRef *int, need int, elemSize uintptr) error {
	if need <= 0 {
		return nil
	}
	if *ptr != nil && *capRef >= need {
		return nil
	}
	if *ptr != nil {
		if bridgeCudaFree(*ptr) != 0 {
			return ErrUnavailable
		}
		*ptr = nil
		*capRef = 0
	}
	if bridgeCudaMalloc(ptr, uintptr(need)*elemSize) != 0 {
		return ErrUnavailable
	}
	*capRef = need
	return nil
}

func (r *BatchRunner) syncNativeStreamingStates(active map[int64]struct{}) {
	if r == nil || r.nativeState == nil {
		return
	}
	for id, state := range r.nativeState {
		if _, ok := active[id]; ok {
			continue
		}
		releaseNativeStreamingSignalState(state)
		delete(r.nativeState, id)
	}
	r.pruneOutRings(active)
}

func (r *BatchRunner) resetNativeStreamingState(signalID int64) {
	if r == nil || r.nativeState == nil {
		return
	}
	if state := r.nativeState[signalID]; state != nil {
		releaseNativeStreamingSignalState(state)
	}
	delete(r.nativeState, signalID)
}

func (r *BatchRunner) resetAllNativeStreamingStates() {
	if r == nil {
		return
	}
	r.freeAllNativeStreamingStates()
	r.nativeState = make(map[int64]*nativeStreamingSignalState)
}

func (r *BatchRunner) freeAllNativeStreamingStates() {
	if r == nil || r.nativeState == nil {
		return
	}
	for id, state := range r.nativeState {
		releaseNativeStreamingSignalState(state)
		delete(r.nativeState, id)
	}
}

func releaseNativeStreamingSignalState(state *nativeStreamingSignalState) {
	if state == nil {
		return
	}
	for _, ptr := range []*unsafe.Pointer{
		&state.dInNew,
		&state.dShifted,
		&state.dOut,
		&state.dTaps,
		&state.dHistory,
		&state.dHistoryScratch,
	} {
		if *ptr != nil {
			_ = bridgeCudaFree(*ptr)
			*ptr = nil
		}
	}
	state.inNewCap = 0
	state.shiftedCap = 0
	state.outCap = 0
	state.tapsLen = 0
	state.historyCap = 0
	state.historyLen = 0
	state.historyScratchCap = 0
	state.phaseCount = 0
	state.phaseNCO = 0
	state.decim = 0
	state.numTaps = 0
	state.configHash = 0
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

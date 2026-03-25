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
	results := make([]StreamingGPUExecutionResult, len(invocations))
	for i, inv := range invocations {
		phaseInc := -2.0 * math.Pi * inv.OffsetHz / float64(inv.SampleRate)
		outCap := len(inv.IQNew)/maxInt(1, inv.Decim) + 2
		outHost := make([]complex64, outCap)
		histCap := maxInt(0, inv.NumTaps-1)
		histHost := make([]complex64, histCap)
		var nOut C.int
		var phaseCountOut C.int
		var phaseEndOut C.double

		var dInNew, dHistIn, dOut, dHistOut unsafe.Pointer
		var dTaps unsafe.Pointer
		if len(inv.IQNew) > 0 {
			if bridgeCudaMalloc(&dInNew, uintptr(len(inv.IQNew))*unsafe.Sizeof(C.gpud_float2{})) != 0 {
				return nil, ErrUnavailable
			}
			defer bridgeCudaFree(dInNew)
			if bridgeMemcpyH2D(dInNew, unsafe.Pointer(&inv.IQNew[0]), uintptr(len(inv.IQNew))*unsafe.Sizeof(complex64(0))) != 0 {
				return nil, ErrUnavailable
			}
		}
		if len(inv.ShiftedHistory) > 0 {
			if bridgeCudaMalloc(&dHistIn, uintptr(len(inv.ShiftedHistory))*unsafe.Sizeof(C.gpud_float2{})) != 0 {
				return nil, ErrUnavailable
			}
			defer bridgeCudaFree(dHistIn)
			if bridgeMemcpyH2D(dHistIn, unsafe.Pointer(&inv.ShiftedHistory[0]), uintptr(len(inv.ShiftedHistory))*unsafe.Sizeof(complex64(0))) != 0 {
				return nil, ErrUnavailable
			}
		}
		if len(inv.PolyphaseTaps) > 0 {
			if bridgeCudaMalloc(&dTaps, uintptr(len(inv.PolyphaseTaps))*unsafe.Sizeof(C.float(0))) != 0 {
				return nil, ErrUnavailable
			}
			defer bridgeCudaFree(dTaps)
			if bridgeMemcpyH2D(dTaps, unsafe.Pointer(&inv.PolyphaseTaps[0]), uintptr(len(inv.PolyphaseTaps))*unsafe.Sizeof(float32(0))) != 0 {
				return nil, ErrUnavailable
			}
		}
		if outCap > 0 {
			if bridgeCudaMalloc(&dOut, uintptr(outCap)*unsafe.Sizeof(C.gpud_float2{})) != 0 {
				return nil, ErrUnavailable
			}
			defer bridgeCudaFree(dOut)
		}
		if histCap > 0 {
			if bridgeCudaMalloc(&dHistOut, uintptr(histCap)*unsafe.Sizeof(C.gpud_float2{})) != 0 {
				return nil, ErrUnavailable
			}
			defer bridgeCudaFree(dHistOut)
		}

		res := bridgeLaunchStreamingPolyphasePrepare(
			(*C.gpud_float2)(dInNew),
			len(inv.IQNew),
			(*C.gpud_float2)(dHistIn),
			len(inv.ShiftedHistory),
			(*C.float)(dTaps),
			len(inv.PolyphaseTaps),
			inv.Decim,
			inv.NumTaps,
			inv.PhaseCountIn,
			inv.NCOPhaseIn,
			phaseInc,
			(*C.gpud_float2)(dOut),
			&nOut,
			&phaseCountOut,
			&phaseEndOut,
			(*C.gpud_float2)(dHistOut),
		)
		if res != 0 {
			return nil, ErrUnavailable
		}
		if int(nOut) > 0 {
			if bridgeMemcpyD2H(unsafe.Pointer(&outHost[0]), dOut, uintptr(int(nOut))*unsafe.Sizeof(complex64(0))) != 0 {
				return nil, ErrUnavailable
			}
		}
		if histCap > 0 {
			if bridgeMemcpyD2H(unsafe.Pointer(&histHost[0]), dHistOut, uintptr(histCap)*unsafe.Sizeof(complex64(0))) != 0 {
				return nil, ErrUnavailable
			}
		}
		results[i] = StreamingGPUExecutionResult{
			SignalID:      inv.SignalID,
			Mode:          StreamingGPUExecCUDA,
			IQ:            append([]complex64(nil), outHost[:int(nOut)]...),
			Rate:          inv.OutRate,
			NOut:          int(nOut),
			PhaseCountOut: int(phaseCountOut),
			NCOPhaseOut:   float64(phaseEndOut),
			HistoryOut:    append([]complex64(nil), histHost...),
			HistoryLenOut: histCap,
		}
	}
	return results, nil
}

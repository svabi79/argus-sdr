//go:build cufft && windows

package gpudemod

/*
#include <cuda_runtime.h>
*/
import "C"

import (
	"errors"
	"math"
	"unsafe"

	"sdr-visual-suite/internal/dsp"
)

type slotBuffers struct {
	dShifted   unsafe.Pointer
	dFiltered  unsafe.Pointer
	dDecimated unsafe.Pointer
	dTaps      unsafe.Pointer
	stream     streamHandle
}

type windowsBatchRunner struct {
	*BatchRunner
	slotBufs []slotBuffers
}

func asWindowsBatchRunner(r *BatchRunner) *windowsBatchRunner {
	return (*windowsBatchRunner)(unsafe.Pointer(r))
}

func (r *windowsBatchRunner) freeSlotBuffers() {
	for i := range r.slotBufs {
		if r.slotBufs[i].dShifted != nil {
			_ = bridgeCudaFree(r.slotBufs[i].dShifted)
			r.slotBufs[i].dShifted = nil
		}
		if r.slotBufs[i].dFiltered != nil {
			_ = bridgeCudaFree(r.slotBufs[i].dFiltered)
			r.slotBufs[i].dFiltered = nil
		}
		if r.slotBufs[i].dDecimated != nil {
			_ = bridgeCudaFree(r.slotBufs[i].dDecimated)
			r.slotBufs[i].dDecimated = nil
		}
		if r.slotBufs[i].dTaps != nil {
			_ = bridgeCudaFree(r.slotBufs[i].dTaps)
			r.slotBufs[i].dTaps = nil
		}
		if r.slotBufs[i].stream != nil {
			_ = bridgeStreamDestroy(r.slotBufs[i].stream)
			r.slotBufs[i].stream = nil
		}
	}
	r.slotBufs = nil
}

func (r *windowsBatchRunner) allocSlotBuffers(n int) error {
	if len(r.slotBufs) == len(r.slots) && len(r.slotBufs) > 0 {
		return nil
	}
	r.freeSlotBuffers()
	if len(r.slots) == 0 {
		return nil
	}
	iqBytes := uintptr(n) * unsafe.Sizeof(complex64(0))
	tapsBytes := uintptr(256) * unsafe.Sizeof(float32(0))
	r.slotBufs = make([]slotBuffers, len(r.slots))
	for i := range r.slotBufs {
		for _, ptr := range []*unsafe.Pointer{&r.slotBufs[i].dShifted, &r.slotBufs[i].dFiltered, &r.slotBufs[i].dDecimated} {
			if bridgeCudaMalloc(ptr, iqBytes) != 0 {
				r.freeSlotBuffers()
				return errors.New("cudaMalloc slot buffer failed")
			}
		}
		if bridgeCudaMalloc(&r.slotBufs[i].dTaps, tapsBytes) != 0 {
			r.freeSlotBuffers()
			return errors.New("cudaMalloc slot taps failed")
		}
		s, res := bridgeStreamCreate()
		if res != 0 {
			r.freeSlotBuffers()
			return errors.New("cudaStreamCreate failed")
		}
		r.slotBufs[i].stream = s
	}
	return nil
}

func (r *BatchRunner) shiftFilterDecimateBatchImpl(iq []complex64) ([][]complex64, []int, error) {
	wr := asWindowsBatchRunner(r)
	e := r.eng
	if e == nil || !e.cudaReady {
		return nil, nil, ErrUnavailable
	}
	outs := make([][]complex64, len(r.slots))
	rates := make([]int, len(r.slots))
	n := len(iq)
	if n == 0 {
		return outs, rates, nil
	}
	if err := wr.allocSlotBuffers(n); err != nil {
		return nil, nil, err
	}
	bytesIn := uintptr(n) * unsafe.Sizeof(complex64(0))
	if bridgeMemcpyH2D(unsafe.Pointer(e.dIQIn), unsafe.Pointer(&iq[0]), bytesIn) != 0 {
		return nil, nil, errors.New("cudaMemcpy H2D failed")
	}
	for i := range r.slots {
		if !r.slots[i].active {
			continue
		}
		nOut, rate, err := r.shiftFilterDecimateSlotParallel(iq, r.slots[i].job, wr.slotBufs[i])
		if err != nil {
			return nil, nil, err
		}
		r.slots[i].rate = rate
		outs[i] = make([]complex64, nOut)
		rates[i] = rate
	}
	for i := range r.slots {
		if !r.slots[i].active {
			continue
		}
		buf := wr.slotBufs[i]
		if bridgeStreamSync(buf.stream) != 0 {
			return nil, nil, errors.New("cuda stream sync failed")
		}
		out := outs[i]
		if len(out) == 0 {
			continue
		}
		outBytes := uintptr(len(out)) * unsafe.Sizeof(complex64(0))
		if bridgeMemcpyD2H(unsafe.Pointer(&out[0]), buf.dDecimated, outBytes) != 0 {
			return nil, nil, errors.New("cudaMemcpy D2H failed")
		}
		r.slots[i].out = out
	}
	return outs, rates, nil
}

func (r *BatchRunner) shiftFilterDecimateSlotParallel(iq []complex64, job ExtractJob, buf slotBuffers) (int, int, error) {
	e := r.eng
	if e == nil || !e.cudaReady {
		return 0, 0, ErrUnavailable
	}
	n := len(iq)
	if n == 0 {
		return 0, 0, nil
	}
	cutoff := job.BW / 2
	if cutoff < 200 {
		cutoff = 200
	}
	base := dsp.LowpassFIR(cutoff, e.sampleRate, 101)
	taps := make([]float32, len(base))
	for i, v := range base {
		taps[i] = float32(v)
	}
	if len(taps) == 0 {
		return 0, 0, errors.New("no FIR taps configured")
	}
	tapsBytes := uintptr(len(taps)) * unsafe.Sizeof(float32(0))
	if bridgeMemcpyH2D(buf.dTaps, unsafe.Pointer(&taps[0]), tapsBytes) != 0 {
		return 0, 0, errors.New("taps H2D failed")
	}
	decim := int(math.Round(float64(e.sampleRate) / float64(job.OutRate)))
	if decim < 1 {
		decim = 1
	}
	nOut := n / decim
	if nOut <= 0 {
		return 0, 0, errors.New("not enough output samples after decimation")
	}
	phaseInc := -2.0 * math.Pi * job.OffsetHz / float64(e.sampleRate)
	if bridgeLaunchFreqShiftStream(e.dIQIn, (*gpuFloat2)(buf.dShifted), n, phaseInc, e.phase, buf.stream) != 0 {
		return 0, 0, errors.New("gpu freq shift failed")
	}
	if bridgeLaunchFIRv2Stream((*gpuFloat2)(buf.dShifted), (*gpuFloat2)(buf.dFiltered), (*C.float)(buf.dTaps), n, len(taps), buf.stream) != 0 {
		return 0, 0, errors.New("gpu FIR v2 failed")
	}
	if bridgeLaunchDecimateStream((*gpuFloat2)(buf.dFiltered), (*gpuFloat2)(buf.dDecimated), nOut, decim, buf.stream) != 0 {
		return 0, 0, errors.New("gpu decimate failed")
	}
	return nOut, e.sampleRate / decim, nil
}

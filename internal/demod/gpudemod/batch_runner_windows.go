//go:build cufft && windows

package gpudemod

import (
	"errors"
	"math"
	"unsafe"

	"sdr-visual-suite/internal/dsp"
)

func (r *BatchRunner) shiftFilterDecimateBatchImpl(iq []complex64) ([][]complex64, []int, error) {
	outs := make([][]complex64, len(r.slots))
	rates := make([]int, len(r.slots))
	streams := make([]streamHandle, len(r.slots))
	for i := range streams {
		s, _ := bridgeStreamCreate()
		streams[i] = s
	}
	defer func() {
		for _, s := range streams {
			if s != nil {
				_ = bridgeStreamDestroy(s)
			}
		}
	}()
	for i := range r.slots {
		if !r.slots[i].active {
			continue
		}
		out, rate, err := r.shiftFilterDecimateSlot(iq, r.slots[i].job, streams[i])
		if err != nil {
			return nil, nil, err
		}
		r.slots[i].out = out
		r.slots[i].rate = rate
		outs[i] = out
		rates[i] = rate
	}
	return outs, rates, nil
}

func (r *BatchRunner) shiftFilterDecimateSlot(iq []complex64, job ExtractJob, stream streamHandle) ([]complex64, int, error) {
	e := r.eng
	if e == nil || !e.cudaReady {
		return nil, 0, ErrUnavailable
	}
	if len(iq) == 0 {
		return nil, 0, nil
	}
	cutoff := job.BW / 2
	if cutoff < 200 {
		cutoff = 200
	}
	taps := e.firTaps
	if len(taps) == 0 {
		base64 := dsp.LowpassFIR(cutoff, e.sampleRate, 101)
		taps = make([]float32, len(base64))
		for i, v := range base64 {
			taps[i] = float32(v)
		}
		e.SetFIR(taps)
	}
	decim := int(math.Round(float64(e.sampleRate) / float64(job.OutRate)))
	if decim < 1 {
		decim = 1
	}
	n := len(iq)
	nOut := n / decim
	if nOut <= 0 {
		return nil, 0, errors.New("not enough output samples after decimation")
	}
	bytesIn := uintptr(n) * unsafe.Sizeof(complex64(0))
	if bridgeMemcpyH2D(unsafe.Pointer(e.dIQIn), unsafe.Pointer(&iq[0]), bytesIn) != 0 {
		return nil, 0, errors.New("cudaMemcpy H2D failed")
	}
	phaseInc := -2.0 * math.Pi * job.OffsetHz / float64(e.sampleRate)
	if bridgeLaunchFreqShiftStream(e.dIQIn, e.dShifted, n, phaseInc, e.phase, stream) != 0 {
		return nil, 0, errors.New("gpu freq shift failed")
	}
	if bridgeLaunchFIRStream(e.dShifted, e.dFiltered, n, len(taps), stream) != 0 {
		return nil, 0, errors.New("gpu FIR failed")
	}
	if bridgeLaunchDecimateStream(e.dFiltered, e.dDecimated, nOut, decim, stream) != 0 {
		return nil, 0, errors.New("gpu decimate failed")
	}
	if bridgeStreamSync(stream) != 0 {
		return nil, 0, errors.New("cuda stream sync failed")
	}
	out := make([]complex64, nOut)
	outBytes := uintptr(nOut) * unsafe.Sizeof(complex64(0))
	if bridgeMemcpyD2H(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dDecimated), outBytes) != 0 {
		return nil, 0, errors.New("cudaMemcpy D2H failed")
	}
	return out, e.sampleRate / decim, nil
}

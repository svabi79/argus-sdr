//go:build cufft && windows

package gpudemod

/*
#include <stdlib.h>
#include <cuda_runtime.h>
typedef struct { float x; float y; } gpud_float2;
typedef void* gpud_stream_handle;
extern int gpud_stream_create(gpud_stream_handle* out);
extern int gpud_stream_destroy(gpud_stream_handle stream);
extern int gpud_stream_sync(gpud_stream_handle stream);
extern int gpud_launch_freq_shift_stream(gpud_float2 *in, gpud_float2 *out, int n, double phase_inc, double phase_start, gpud_stream_handle stream);
extern int gpud_launch_fir_stream(gpud_float2 *in, gpud_float2 *out, int n, int num_taps, gpud_stream_handle stream);
extern int gpud_launch_decimate_stream(gpud_float2 *in, gpud_float2 *out, int n_out, int factor, gpud_stream_handle stream);
extern int gpud_memcpy_h2d(void *dst, const void *src, size_t bytes);
extern int gpud_memcpy_d2h(void *dst, const void *src, size_t bytes);
*/
import "C"

import (
	"errors"
	"math"
	"unsafe"

	"sdr-visual-suite/internal/dsp"
)

func (r *BatchRunner) shiftFilterDecimateBatchImpl(iq []complex64) ([][]complex64, []int, error) {
	outs := make([][]complex64, len(r.slots))
	rates := make([]int, len(r.slots))
	streams := make([]C.gpud_stream_handle, len(r.slots))
	for i := range streams {
		_ = C.gpud_stream_create(&streams[i])
	}
	defer func() {
		for _, s := range streams {
			if s != nil {
				_ = C.gpud_stream_destroy(s)
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

func (r *BatchRunner) shiftFilterDecimateSlot(iq []complex64, job ExtractJob, stream C.gpud_stream_handle) ([]complex64, int, error) {
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
	bytesIn := C.size_t(n) * C.size_t(unsafe.Sizeof(complex64(0)))
	if C.gpud_memcpy_h2d(unsafe.Pointer(e.dIQIn), unsafe.Pointer(&iq[0]), bytesIn) != C.cudaSuccess {
		return nil, 0, errors.New("cudaMemcpy H2D failed")
	}
	phaseInc := -2.0 * math.Pi * job.OffsetHz / float64(e.sampleRate)
	if C.gpud_launch_freq_shift_stream(e.dIQIn, e.dShifted, C.int(n), C.double(phaseInc), C.double(e.phase), stream) != 0 {
		return nil, 0, errors.New("gpu freq shift failed")
	}
	if C.gpud_launch_fir_stream(e.dShifted, e.dFiltered, C.int(n), C.int(len(taps)), stream) != 0 {
		return nil, 0, errors.New("gpu FIR failed")
	}
	if C.gpud_launch_decimate_stream(e.dFiltered, e.dDecimated, C.int(nOut), C.int(decim), stream) != 0 {
		return nil, 0, errors.New("gpu decimate failed")
	}
	if C.gpud_stream_sync(stream) != 0 {
		return nil, 0, errors.New("cuda stream sync failed")
	}
	out := make([]complex64, nOut)
	outBytes := C.size_t(nOut) * C.size_t(unsafe.Sizeof(complex64(0)))
	if C.gpud_memcpy_d2h(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dDecimated), outBytes) != C.cudaSuccess {
		return nil, 0, errors.New("cudaMemcpy D2H failed")
	}
	return out, e.sampleRate / decim, nil
}

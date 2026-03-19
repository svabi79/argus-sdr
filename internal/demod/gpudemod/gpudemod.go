//go:build cufft

package gpudemod

/*
#cgo windows LDFLAGS: -L${SRCDIR}/../../../cuda-mingw -lcufft64_12 -lcudart64_13 ${SRCDIR}/build/kernels.obj
#cgo windows CFLAGS: -I"C:/Program Files/NVIDIA GPU Computing Toolkit/CUDA/v13.2/include"
#include <cuda_runtime.h>
#include <cufft.h>

typedef struct { float x; float y; } gpud_float2;

static int gpud_cuda_malloc(void **ptr, size_t bytes) {
	return (int)cudaMalloc(ptr, bytes);
}

static int gpud_cuda_free(void *ptr) {
	return (int)cudaFree(ptr);
}

static int gpud_memcpy_h2d(void *dst, const void *src, size_t bytes) {
	return (int)cudaMemcpy(dst, src, bytes, cudaMemcpyHostToDevice);
}

static int gpud_memcpy_d2h(void *dst, const void *src, size_t bytes) {
	return (int)cudaMemcpy(dst, src, bytes, cudaMemcpyDeviceToHost);
}

static int gpud_device_sync() {
	return (int)cudaDeviceSynchronize();
}

extern int gpud_launch_freq_shift_cuda(const gpud_float2* in, gpud_float2* out, int n, double phase_inc, double phase_start);

static int gpud_launch_freq_shift(gpud_float2 *in, gpud_float2 *out, int n, double phase_inc, double phase_start) {
	return gpud_launch_freq_shift_cuda(in, out, n, phase_inc, phase_start);
}

extern int gpud_launch_fm_discrim_cuda(const gpud_float2* in, float* out, int n);

static int gpud_launch_fm_discrim(gpud_float2 *in, float *out, int n) {
	return gpud_launch_fm_discrim_cuda(in, out, n);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"math"
	"unsafe"

	"sdr-visual-suite/internal/demod"
	"sdr-visual-suite/internal/dsp"
)

type DemodType int

const (
	DemodNFM DemodType = iota
	DemodWFM
	DemodAM
	DemodUSB
	DemodLSB
	DemodCW
)

type Engine struct {
	maxSamples       int
	sampleRate       int
	phase            float64
	bfoPhase         float64
	firTaps          []float32
	cudaReady        bool
	lastShiftUsedGPU bool
	dIQIn            *C.gpud_float2
	dShifted         *C.gpud_float2
	dAudio           *C.float
	iqBytes          C.size_t
	audioBytes       C.size_t
}

func Available() bool {
	var count C.int
	if C.cudaGetDeviceCount(&count) != C.cudaSuccess {
		return false
	}
	return count > 0
}

func New(maxSamples int, sampleRate int) (*Engine, error) {
	if maxSamples <= 0 {
		return nil, errors.New("invalid maxSamples")
	}
	if sampleRate <= 0 {
		return nil, errors.New("invalid sampleRate")
	}
	if !Available() {
		return nil, errors.New("cuda device not available")
	}
	e := &Engine{
		maxSamples: maxSamples,
		sampleRate: sampleRate,
		cudaReady:  true,
		iqBytes:    C.size_t(maxSamples) * C.size_t(unsafe.Sizeof(C.gpud_float2{})),
		audioBytes: C.size_t(maxSamples) * C.size_t(unsafe.Sizeof(C.float(0))),
	}
	var ptr unsafe.Pointer
	if C.gpud_cuda_malloc(&ptr, e.iqBytes) != C.cudaSuccess {
		e.Close()
		return nil, errors.New("cudaMalloc dIQIn failed")
	}
	e.dIQIn = (*C.gpud_float2)(ptr)
	ptr = nil
	if C.gpud_cuda_malloc(&ptr, e.iqBytes) != C.cudaSuccess {
		e.Close()
		return nil, errors.New("cudaMalloc dShifted failed")
	}
	e.dShifted = (*C.gpud_float2)(ptr)
	ptr = nil
	if C.gpud_cuda_malloc(&ptr, e.audioBytes) != C.cudaSuccess {
		e.Close()
		return nil, errors.New("cudaMalloc dAudio failed")
	}
	e.dAudio = (*C.float)(ptr)
	return e, nil
}

func (e *Engine) SetFIR(taps []float32) {
	if len(taps) == 0 {
		e.firTaps = nil
		return
	}
	e.firTaps = append(e.firTaps[:0], taps...)
}

func phaseStatus() string {
	return "phase1c-validated-shift"
}

func (e *Engine) LastShiftUsedGPU() bool {
	if e == nil {
		return false
	}
	return e.lastShiftUsedGPU
}

func (e *Engine) tryCUDAFreqShift(iq []complex64, offsetHz float64) ([]complex64, bool) {
	if e == nil || !e.cudaReady || len(iq) == 0 || e.dIQIn == nil || e.dShifted == nil {
		return nil, false
	}
	bytes := C.size_t(len(iq)) * C.size_t(unsafe.Sizeof(complex64(0)))
	if C.gpud_memcpy_h2d(unsafe.Pointer(e.dIQIn), unsafe.Pointer(&iq[0]), bytes) != C.cudaSuccess {
		return nil, false
	}
	phaseInc := -2.0 * math.Pi * offsetHz / float64(e.sampleRate)
	if C.gpud_launch_freq_shift(e.dIQIn, e.dShifted, C.int(len(iq)), C.double(phaseInc), C.double(e.phase)) != 0 {
		return nil, false
	}
	if C.gpud_device_sync() != C.cudaSuccess {
		return nil, false
	}
	out := make([]complex64, len(iq))
	if C.gpud_memcpy_d2h(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dShifted), bytes) != C.cudaSuccess {
		return nil, false
	}
	e.phase += phaseInc * float64(len(iq))
	return out, true
}

func (e *Engine) tryCUDAFMDiscrim(shifted []complex64) ([]float32, bool) {
	if e == nil || !e.cudaReady || len(shifted) < 2 || e.dShifted == nil || e.dAudio == nil {
		return nil, false
	}
	iqBytes := C.size_t(len(shifted)) * C.size_t(unsafe.Sizeof(complex64(0)))
	if C.gpud_memcpy_h2d(unsafe.Pointer(e.dShifted), unsafe.Pointer(&shifted[0]), iqBytes) != C.cudaSuccess {
		return nil, false
	}
	if C.gpud_launch_fm_discrim(e.dShifted, e.dAudio, C.int(len(shifted))) != 0 {
		return nil, false
	}
	if C.gpud_device_sync() != C.cudaSuccess {
		return nil, false
	}
	out := make([]float32, len(shifted)-1)
	outBytes := C.size_t(len(out)) * C.size_t(unsafe.Sizeof(float32(0)))
	if C.gpud_memcpy_d2h(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dAudio), outBytes) != C.cudaSuccess {
		return nil, false
	}
	return out, true
}

func (e *Engine) Demod(iq []complex64, offsetHz float64, bw float64, mode DemodType) ([]float32, int, error) {
	if e == nil {
		return nil, 0, errors.New("nil CUDA demod engine")
	}
	if !e.cudaReady {
		return nil, 0, errors.New("cuda demod engine is not initialized")
	}
	if len(iq) == 0 {
		return nil, 0, nil
	}
	if len(iq) > e.maxSamples {
		return nil, 0, errors.New("sample count exceeds engine capacity")
	}

	_ = fmt.Sprintf("%s:%0.3f", phaseStatus(), offsetHz)
	shifted, ok := e.tryCUDAFreqShift(iq, offsetHz)
	e.lastShiftUsedGPU = ok && ValidateFreqShift(iq, e.sampleRate, offsetHz, shifted, 1e-3)
	if !e.lastShiftUsedGPU {
		shifted = dsp.FreqShift(iq, e.sampleRate, offsetHz)
	}

	var outRate int
	switch mode {
	case DemodNFM, DemodAM, DemodUSB, DemodLSB, DemodCW:
		outRate = 48000
	case DemodWFM:
		outRate = 192000
	default:
		return nil, 0, errors.New("unsupported demod type")
	}

	cutoff := bw / 2
	if cutoff < 200 {
		cutoff = 200
	}
	taps := e.firTaps
	if len(taps) == 0 {
		base := dsp.LowpassFIR(cutoff, e.sampleRate, 101)
		taps = append(make([]float32, 0, len(base)), base...)
	}
	filtered := dsp.ApplyFIR(shifted, taps)
	decim := int(math.Round(float64(e.sampleRate) / float64(outRate)))
	if decim < 1 {
		decim = 1
	}
	dec := dsp.Decimate(filtered, decim)
	inputRate := e.sampleRate / decim

	switch mode {
	case DemodNFM:
		if gpuAudio, ok := e.tryCUDAFMDiscrim(dec); ok {
			return gpuAudio, inputRate, nil
		}
		return demod.NFM{}.Demod(dec, inputRate), inputRate, nil
	case DemodWFM:
		if gpuAudio, ok := e.tryCUDAFMDiscrim(dec); ok {
			return gpuAudio, inputRate, nil
		}
		return demod.WFM{}.Demod(dec, inputRate), inputRate, nil
	case DemodAM:
		return demod.AM{}.Demod(dec, inputRate), inputRate, nil
	case DemodUSB:
		return demod.USB{}.Demod(dec, inputRate), inputRate, nil
	case DemodLSB:
		return demod.LSB{}.Demod(dec, inputRate), inputRate, nil
	case DemodCW:
		return demod.CW{}.Demod(dec, inputRate), inputRate, nil
	default:
		return nil, 0, errors.New("unsupported demod type")
	}
}

func (e *Engine) Close() {
	if e == nil {
		return
	}
	if e.dIQIn != nil {
		_ = C.gpud_cuda_free(unsafe.Pointer(e.dIQIn))
		e.dIQIn = nil
	}
	if e.dShifted != nil {
		_ = C.gpud_cuda_free(unsafe.Pointer(e.dShifted))
		e.dShifted = nil
	}
	if e.dAudio != nil {
		_ = C.gpud_cuda_free(unsafe.Pointer(e.dAudio))
		e.dAudio = nil
	}
	e.firTaps = nil
	e.cudaReady = false
}

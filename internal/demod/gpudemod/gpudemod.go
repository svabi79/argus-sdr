//go:build cufft && !windows

package gpudemod

/*
#cgo windows LDFLAGS: -L${SRCDIR}/../../../cuda-mingw -L${SRCDIR}/build -lgpudemod_kernels -lcufft64_12 -lcudart64_13 -lstdc++
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
extern int gpud_launch_fm_discrim_cuda(const gpud_float2* in, float* out, int n);
extern int gpud_upload_fir_taps_cuda(const float* taps, int n);
extern int gpud_launch_fir_cuda(const gpud_float2* in, gpud_float2* out, int n, int num_taps);
extern int gpud_launch_decimate_cuda(const gpud_float2* in, gpud_float2* out, int n_out, int factor);
extern int gpud_launch_am_envelope_cuda(const gpud_float2* in, float* out, int n);
extern int gpud_launch_ssb_product_cuda(const gpud_float2* in, float* out, int n, double phase_inc, double phase_start);

static int gpud_launch_freq_shift(gpud_float2 *in, gpud_float2 *out, int n, double phase_inc, double phase_start) {
	return gpud_launch_freq_shift_cuda(in, out, n, phase_inc, phase_start);
}

static int gpud_launch_fm_discrim(gpud_float2 *in, float *out, int n) {
	return gpud_launch_fm_discrim_cuda(in, out, n);
}

static int gpud_upload_fir_taps(const float* taps, int n) {
	return gpud_upload_fir_taps_cuda(taps, n);
}

static int gpud_launch_fir(gpud_float2 *in, gpud_float2 *out, int n, int num_taps) {
	return gpud_launch_fir_cuda(in, out, n, num_taps);
}

static int gpud_launch_decimate(gpud_float2 *in, gpud_float2 *out, int n_out, int factor) {
	return gpud_launch_decimate_cuda(in, out, n_out, factor);
}

static int gpud_launch_am_envelope(gpud_float2 *in, float *out, int n) {
	return gpud_launch_am_envelope_cuda(in, out, n);
}

static int gpud_launch_ssb_product(gpud_float2 *in, float *out, int n, double phase_inc, double phase_start) {
	return gpud_launch_ssb_product_cuda(in, out, n, phase_inc, phase_start);
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
	maxSamples        int
	sampleRate        int
	phase             float64
	bfoPhase          float64
	firTaps           []float32
	cudaReady         bool
	lastShiftUsedGPU  bool
	lastFIRUsedGPU    bool
	lastDecimUsedGPU  bool
	lastDemodUsedGPU  bool
	dIQIn             *C.gpud_float2
	dShifted          *C.gpud_float2
	dFiltered         *C.gpud_float2
	dDecimated        *C.gpud_float2
	dAudio            *C.float
	iqBytes           C.size_t
	audioBytes        C.size_t
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
	if C.gpud_cuda_malloc(&ptr, e.iqBytes) != C.cudaSuccess {
		e.Close()
		return nil, errors.New("cudaMalloc dFiltered failed")
	}
	e.dFiltered = (*C.gpud_float2)(ptr)
	ptr = nil
	if C.gpud_cuda_malloc(&ptr, e.iqBytes) != C.cudaSuccess {
		e.Close()
		return nil, errors.New("cudaMalloc dDecimated failed")
	}
	e.dDecimated = (*C.gpud_float2)(ptr)
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
	if len(taps) > 256 {
		taps = taps[:256]
	}
	e.firTaps = append(e.firTaps[:0], taps...)
	if e.cudaReady {
		_ = C.gpud_upload_fir_taps((*C.float)(unsafe.Pointer(&e.firTaps[0])), C.int(len(e.firTaps)))
	}
}

func (e *Engine) LastShiftUsedGPU() bool { return e != nil && e.lastShiftUsedGPU }
func (e *Engine) LastDemodUsedGPU() bool { return e != nil && e.lastDemodUsedGPU }

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

func (e *Engine) tryCUDAFIR(iq []complex64, numTaps int) ([]complex64, bool) {
	if e == nil || !e.cudaReady || len(iq) == 0 || numTaps <= 0 || e.dShifted == nil || e.dFiltered == nil {
		return nil, false
	}
	iqBytes := C.size_t(len(iq)) * C.size_t(unsafe.Sizeof(complex64(0)))
	if C.gpud_memcpy_h2d(unsafe.Pointer(e.dShifted), unsafe.Pointer(&iq[0]), iqBytes) != C.cudaSuccess {
		return nil, false
	}
	if C.gpud_launch_fir(e.dShifted, e.dFiltered, C.int(len(iq)), C.int(numTaps)) != 0 {
		return nil, false
	}
	if C.gpud_device_sync() != C.cudaSuccess {
		return nil, false
	}
	out := make([]complex64, len(iq))
	if C.gpud_memcpy_d2h(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dFiltered), iqBytes) != C.cudaSuccess {
		return nil, false
	}
	return out, true
}

func (e *Engine) tryCUDADecimate(filtered []complex64, factor int) ([]complex64, bool) {
	if e == nil || !e.cudaReady || len(filtered) == 0 || factor <= 0 || e.dFiltered == nil || e.dDecimated == nil {
		return nil, false
	}
	nOut := len(filtered) / factor
	if nOut <= 0 {
		return nil, false
	}
	iqBytes := C.size_t(len(filtered)) * C.size_t(unsafe.Sizeof(complex64(0)))
	if C.gpud_memcpy_h2d(unsafe.Pointer(e.dFiltered), unsafe.Pointer(&filtered[0]), iqBytes) != C.cudaSuccess {
		return nil, false
	}
	if C.gpud_launch_decimate(e.dFiltered, e.dDecimated, C.int(nOut), C.int(factor)) != 0 {
		return nil, false
	}
	if C.gpud_device_sync() != C.cudaSuccess {
		return nil, false
	}
	out := make([]complex64, nOut)
	outBytes := C.size_t(nOut) * C.size_t(unsafe.Sizeof(complex64(0)))
	if C.gpud_memcpy_d2h(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dDecimated), outBytes) != C.cudaSuccess {
		return nil, false
	}
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

func (e *Engine) tryCUDAAMEnvelope(shifted []complex64) ([]float32, bool) {
	if e == nil || !e.cudaReady || len(shifted) == 0 || e.dShifted == nil || e.dAudio == nil {
		return nil, false
	}
	iqBytes := C.size_t(len(shifted)) * C.size_t(unsafe.Sizeof(complex64(0)))
	if C.gpud_memcpy_h2d(unsafe.Pointer(e.dShifted), unsafe.Pointer(&shifted[0]), iqBytes) != C.cudaSuccess {
		return nil, false
	}
	if C.gpud_launch_am_envelope(e.dShifted, e.dAudio, C.int(len(shifted))) != 0 {
		return nil, false
	}
	if C.gpud_device_sync() != C.cudaSuccess {
		return nil, false
	}
	out := make([]float32, len(shifted))
	outBytes := C.size_t(len(out)) * C.size_t(unsafe.Sizeof(float32(0)))
	if C.gpud_memcpy_d2h(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dAudio), outBytes) != C.cudaSuccess {
		return nil, false
	}
	return out, true
}

func (e *Engine) tryCUDASSBProduct(shifted []complex64, bfoHz float64) ([]float32, bool) {
	if e == nil || !e.cudaReady || len(shifted) == 0 || e.dShifted == nil || e.dAudio == nil {
		return nil, false
	}
	iqBytes := C.size_t(len(shifted)) * C.size_t(unsafe.Sizeof(complex64(0)))
	if C.gpud_memcpy_h2d(unsafe.Pointer(e.dShifted), unsafe.Pointer(&shifted[0]), iqBytes) != C.cudaSuccess {
		return nil, false
	}
	phaseInc := 2.0 * math.Pi * bfoHz / float64(e.sampleRate)
	if C.gpud_launch_ssb_product(e.dShifted, e.dAudio, C.int(len(shifted)), C.double(phaseInc), C.double(e.bfoPhase)) != 0 {
		return nil, false
	}
	if C.gpud_device_sync() != C.cudaSuccess {
		return nil, false
	}
	out := make([]float32, len(shifted))
	outBytes := C.size_t(len(out)) * C.size_t(unsafe.Sizeof(float32(0)))
	if C.gpud_memcpy_d2h(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dAudio), outBytes) != C.cudaSuccess {
		return nil, false
	}
	e.bfoPhase += phaseInc * float64(len(shifted))
	return out, true
}

func (e *Engine) DemodFused(iq []complex64, offsetHz float64, bw float64, mode DemodType) ([]float32, int, error) {
	return e.Demod(iq, offsetHz, bw, mode)
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
		base64 := dsp.LowpassFIR(cutoff, e.sampleRate, 101)
		taps = make([]float32, len(base64))
		for i, v := range base64 {
			taps[i] = float32(v)
		}
		e.SetFIR(taps)
	}
	filtered, ok := e.tryCUDAFIR(shifted, len(taps))
	e.lastFIRUsedGPU = ok && ValidateFIR(shifted, taps, filtered, 1e-3)
	if !e.lastFIRUsedGPU {
		ftaps := make([]float64, len(taps))
		for i, v := range taps {
			ftaps[i] = float64(v)
		}
		filtered = dsp.ApplyFIR(shifted, ftaps)
	}

	decim := int(math.Round(float64(e.sampleRate) / float64(outRate)))
	if decim < 1 {
		decim = 1
	}
	dec, ok := e.tryCUDADecimate(filtered, decim)
	e.lastDecimUsedGPU = ok && ValidateDecimate(filtered, decim, dec, 1e-3)
	if !e.lastDecimUsedGPU {
		dec = dsp.Decimate(filtered, decim)
	}
	inputRate := e.sampleRate / decim

	e.lastDemodUsedGPU = false
	switch mode {
	case DemodNFM:
		if gpuAudio, ok := e.tryCUDAFMDiscrim(dec); ok {
			e.lastDemodUsedGPU = true
			return gpuAudio, inputRate, nil
		}
		return demod.NFM{}.Demod(dec, inputRate), inputRate, nil
	case DemodWFM:
		if gpuAudio, ok := e.tryCUDAFMDiscrim(dec); ok {
			e.lastDemodUsedGPU = true
			return gpuAudio, inputRate, nil
		}
		return demod.WFM{}.Demod(dec, inputRate), inputRate, nil
	case DemodAM:
		if gpuAudio, ok := e.tryCUDAAMEnvelope(dec); ok {
			e.lastDemodUsedGPU = true
			return gpuAudio, inputRate, nil
		}
		return demod.AM{}.Demod(dec, inputRate), inputRate, nil
	case DemodUSB:
		if gpuAudio, ok := e.tryCUDASSBProduct(dec, 700.0); ok {
			e.lastDemodUsedGPU = true
			return gpuAudio, inputRate, nil
		}
		return demod.USB{}.Demod(dec, inputRate), inputRate, nil
	case DemodLSB:
		if gpuAudio, ok := e.tryCUDASSBProduct(dec, -700.0); ok {
			e.lastDemodUsedGPU = true
			return gpuAudio, inputRate, nil
		}
		return demod.LSB{}.Demod(dec, inputRate), inputRate, nil
	case DemodCW:
		if gpuAudio, ok := e.tryCUDASSBProduct(dec, 700.0); ok {
			e.lastDemodUsedGPU = true
			return gpuAudio, inputRate, nil
		}
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
	if e.dFiltered != nil {
		_ = C.gpud_cuda_free(unsafe.Pointer(e.dFiltered))
		e.dFiltered = nil
	}
	if e.dDecimated != nil {
		_ = C.gpud_cuda_free(unsafe.Pointer(e.dDecimated))
		e.dDecimated = nil
	}
	if e.dAudio != nil {
		_ = C.gpud_cuda_free(unsafe.Pointer(e.dAudio))
		e.dAudio = nil
	}
	e.firTaps = nil
	e.cudaReady = false
}

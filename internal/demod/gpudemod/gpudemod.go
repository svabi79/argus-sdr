//go:build cufft

package gpudemod

/*
#cgo windows LDFLAGS: -lcufft64_12 -lcudart64_13
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

static int gpud_launch_freq_shift(gpud_float2 *in, gpud_float2 *out, int n, double phase_inc, double phase_start) {
	// TODO(phase2): replace with real CUDA kernel launch.
	// Phase 1b keeps the launch boundary in place without pretending acceleration.
	(void)in; (void)out; (void)n; (void)phase_inc; (void)phase_start;
	return -1;
}

static int gpud_launch_fm_discrim(gpud_float2 *in, float *out, int n) {
	// TODO(phase2): replace with real CUDA kernel launch.
	(void)in; (void)out; (void)n;
	return -1;
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
	maxSamples int
	sampleRate int
	phase      float64
	bfoPhase   float64
	firTaps    []float32
	cudaReady  bool
	dIQIn      *C.gpud_float2
	dShifted   *C.gpud_float2
	dAudio     *C.float
	iqBytes    C.size_t
	audioBytes C.size_t
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
	return "phase1b-launch-boundary"
}

func (e *Engine) tryCUDAFreqShift(iq []complex64, offsetHz float64) bool {
	if e == nil || !e.cudaReady || len(iq) == 0 || e.dIQIn == nil || e.dShifted == nil {
		return false
	}
	if C.gpud_memcpy_h2d(unsafe.Pointer(e.dIQIn), unsafe.Pointer(&iq[0]), C.size_t(len(iq))*C.size_t(unsafe.Sizeof(complex64(0)))) != C.cudaSuccess {
		return false
	}
	phaseInc := -2.0 * math.Pi * offsetHz / float64(e.sampleRate)
	if C.gpud_launch_freq_shift(e.dIQIn, e.dShifted, C.int(len(iq)), C.double(phaseInc), C.double(e.phase)) != 0 {
		return false
	}
	if C.gpud_device_sync() != C.cudaSuccess {
		return false
	}
	e.phase += phaseInc * float64(len(iq))
	return true
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
	if mode != DemodNFM {
		return nil, 0, errors.New("CUDA demod phase 1 currently supports NFM only")
	}

	// Real CUDA boundary is now present. If the launch wrappers are not yet backed
	// by actual kernels, we fall back to the existing CPU DSP path below.
	_ = fmt.Sprintf("%s:%0.3f", phaseStatus(), offsetHz)
	_ = e.tryCUDAFreqShift(iq, offsetHz)

	shifted := dsp.FreqShift(iq, e.sampleRate, offsetHz)
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
	outRate := demod.NFM{}.OutputSampleRate()
	decim := int(math.Round(float64(e.sampleRate) / float64(outRate)))
	if decim < 1 {
		decim = 1
	}
	dec := dsp.Decimate(filtered, decim)
	inputRate := e.sampleRate / decim
	audio := demod.NFM{}.Demod(dec, inputRate)
	return audio, inputRate, nil
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

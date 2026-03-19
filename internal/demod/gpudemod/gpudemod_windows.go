//go:build cufft && windows

package gpudemod

/*
#cgo windows CFLAGS: -I"C:/Program Files/NVIDIA GPU Computing Toolkit/CUDA/v13.2/include"
#include <stdlib.h>
#include <cuda_runtime.h>
typedef struct { float x; float y; } gpud_float2;
*/
import "C"

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
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

var loadOnce sync.Once
var loadErr error

func ensureDLLLoaded() error {
	loadOnce.Do(func() {
		candidates := []string{}
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			candidates = append(candidates, filepath.Join(dir, "gpudemod_kernels.dll"))
		}
		if wd, err := os.Getwd(); err == nil {
			candidates = append(candidates,
				filepath.Join(wd, "gpudemod_kernels.dll"),
				filepath.Join(wd, "internal", "demod", "gpudemod", "build", "gpudemod_kernels.dll"),
			)
		}
		if env := os.Getenv("GPUMOD_DLL"); env != "" {
			candidates = append([]string{env}, candidates...)
		}
		seen := map[string]bool{}
		for _, p := range candidates {
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			if _, err := os.Stat(p); err == nil {
				res := bridgeLoadLibrary(p)
				if res == 0 {
					loadErr = nil
					fmt.Fprintf(os.Stderr, "gpudemod: loaded DLL %s\n", p)
					return
				}
				loadErr = fmt.Errorf("failed to load gpudemod DLL: %s (code %d)", p, res)
				fmt.Fprintf(os.Stderr, "gpudemod: DLL load failed for %s (code %d)\n", p, res)
			}
		}
		if loadErr == nil {
			loadErr = errors.New("gpudemod_kernels.dll not found")
			fmt.Fprintln(os.Stderr, "gpudemod: gpudemod_kernels.dll not found in search paths")
		}
	})
	return loadErr
}

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
	iqBytes           uintptr
	audioBytes        uintptr
}

func Available() bool {
	if ensureDLLLoaded() != nil {
		return false
	}
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
	if err := ensureDLLLoaded(); err != nil {
		return nil, err
	}
	if !Available() {
		return nil, errors.New("cuda device not available")
	}
	e := &Engine{
		maxSamples: maxSamples,
		sampleRate: sampleRate,
		cudaReady:  true,
		iqBytes:    uintptr(maxSamples) * unsafe.Sizeof(C.gpud_float2{}),
		audioBytes: uintptr(maxSamples) * unsafe.Sizeof(C.float(0)),
	}
	var ptr unsafe.Pointer
	if bridgeCudaMalloc(&ptr, e.iqBytes) != 0 {
		e.Close()
		return nil, errors.New("cudaMalloc dIQIn failed")
	}
	e.dIQIn = (*C.gpud_float2)(ptr)
	ptr = nil
	if bridgeCudaMalloc(&ptr, e.iqBytes) != 0 {
		e.Close()
		return nil, errors.New("cudaMalloc dShifted failed")
	}
	e.dShifted = (*C.gpud_float2)(ptr)
	ptr = nil
	if bridgeCudaMalloc(&ptr, e.iqBytes) != 0 {
		e.Close()
		return nil, errors.New("cudaMalloc dFiltered failed")
	}
	e.dFiltered = (*C.gpud_float2)(ptr)
	ptr = nil
	if bridgeCudaMalloc(&ptr, e.iqBytes) != 0 {
		e.Close()
		return nil, errors.New("cudaMalloc dDecimated failed")
	}
	e.dDecimated = (*C.gpud_float2)(ptr)
	ptr = nil
	if bridgeCudaMalloc(&ptr, e.audioBytes) != 0 {
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
		_ = bridgeUploadFIRTaps((*C.float)(unsafe.Pointer(&e.firTaps[0])), len(e.firTaps))
	}
}

func (e *Engine) LastShiftUsedGPU() bool { return e != nil && e.lastShiftUsedGPU }
func (e *Engine) LastDemodUsedGPU() bool { return e != nil && e.lastDemodUsedGPU }

func (e *Engine) tryCUDAFreqShift(iq []complex64, offsetHz float64) ([]complex64, bool) {
	if e == nil || !e.cudaReady || len(iq) == 0 || e.dIQIn == nil || e.dShifted == nil {
		return nil, false
	}
	bytes := uintptr(len(iq)) * unsafe.Sizeof(complex64(0))
	if bridgeMemcpyH2D(unsafe.Pointer(e.dIQIn), unsafe.Pointer(&iq[0]), bytes) != 0 {
		return nil, false
	}
	phaseInc := -2.0 * math.Pi * offsetHz / float64(e.sampleRate)
	if bridgeLaunchFreqShift(e.dIQIn, e.dShifted, len(iq), phaseInc, e.phase) != 0 {
		return nil, false
	}
	if bridgeDeviceSync() != 0 {
		return nil, false
	}
	out := make([]complex64, len(iq))
	if bridgeMemcpyD2H(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dShifted), bytes) != 0 {
		return nil, false
	}
	e.phase += phaseInc * float64(len(iq))
	return out, true
}

func (e *Engine) tryCUDAFIR(iq []complex64, numTaps int) ([]complex64, bool) {
	if e == nil || !e.cudaReady || len(iq) == 0 || numTaps <= 0 || e.dShifted == nil || e.dFiltered == nil {
		return nil, false
	}
	iqBytes := uintptr(len(iq)) * unsafe.Sizeof(complex64(0))
	if bridgeMemcpyH2D(unsafe.Pointer(e.dShifted), unsafe.Pointer(&iq[0]), iqBytes) != 0 {
		return nil, false
	}
	if bridgeLaunchFIR(e.dShifted, e.dFiltered, len(iq), numTaps) != 0 {
		return nil, false
	}
	if bridgeDeviceSync() != 0 {
		return nil, false
	}
	out := make([]complex64, len(iq))
	if bridgeMemcpyD2H(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dFiltered), iqBytes) != 0 {
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
	iqBytes := uintptr(len(filtered)) * unsafe.Sizeof(complex64(0))
	if bridgeMemcpyH2D(unsafe.Pointer(e.dFiltered), unsafe.Pointer(&filtered[0]), iqBytes) != 0 {
		return nil, false
	}
	if bridgeLaunchDecimate(e.dFiltered, e.dDecimated, nOut, factor) != 0 {
		return nil, false
	}
	if bridgeDeviceSync() != 0 {
		return nil, false
	}
	out := make([]complex64, nOut)
	outBytes := uintptr(nOut) * unsafe.Sizeof(complex64(0))
	if bridgeMemcpyD2H(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dDecimated), outBytes) != 0 {
		return nil, false
	}
	return out, true
}

func (e *Engine) tryCUDAFMDiscrim(shifted []complex64) ([]float32, bool) {
	if e == nil || !e.cudaReady || len(shifted) < 2 || e.dShifted == nil || e.dAudio == nil {
		return nil, false
	}
	iqBytes := uintptr(len(shifted)) * unsafe.Sizeof(complex64(0))
	if bridgeMemcpyH2D(unsafe.Pointer(e.dShifted), unsafe.Pointer(&shifted[0]), iqBytes) != 0 {
		return nil, false
	}
	if bridgeLaunchFMDiscrim(e.dShifted, e.dAudio, len(shifted)) != 0 {
		return nil, false
	}
	if bridgeDeviceSync() != 0 {
		return nil, false
	}
	out := make([]float32, len(shifted)-1)
	outBytes := uintptr(len(out)) * unsafe.Sizeof(float32(0))
	if bridgeMemcpyD2H(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dAudio), outBytes) != 0 {
		return nil, false
	}
	return out, true
}

func (e *Engine) tryCUDAAMEnvelope(shifted []complex64) ([]float32, bool) {
	if e == nil || !e.cudaReady || len(shifted) == 0 || e.dShifted == nil || e.dAudio == nil {
		return nil, false
	}
	iqBytes := uintptr(len(shifted)) * unsafe.Sizeof(complex64(0))
	if bridgeMemcpyH2D(unsafe.Pointer(e.dShifted), unsafe.Pointer(&shifted[0]), iqBytes) != 0 {
		return nil, false
	}
	if bridgeLaunchAMEnvelope(e.dShifted, e.dAudio, len(shifted)) != 0 {
		return nil, false
	}
	if bridgeDeviceSync() != 0 {
		return nil, false
	}
	out := make([]float32, len(shifted))
	outBytes := uintptr(len(out)) * unsafe.Sizeof(float32(0))
	if bridgeMemcpyD2H(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dAudio), outBytes) != 0 {
		return nil, false
	}
	return out, true
}

func (e *Engine) tryCUDASSBProduct(shifted []complex64, bfoHz float64) ([]float32, bool) {
	if e == nil || !e.cudaReady || len(shifted) == 0 || e.dShifted == nil || e.dAudio == nil {
		return nil, false
	}
	iqBytes := uintptr(len(shifted)) * unsafe.Sizeof(complex64(0))
	if bridgeMemcpyH2D(unsafe.Pointer(e.dShifted), unsafe.Pointer(&shifted[0]), iqBytes) != 0 {
		return nil, false
	}
	phaseInc := 2.0 * math.Pi * bfoHz / float64(e.sampleRate)
	if bridgeLaunchSSBProduct(e.dShifted, e.dAudio, len(shifted), phaseInc, e.bfoPhase) != 0 {
		return nil, false
	}
	if bridgeDeviceSync() != 0 {
		return nil, false
	}
	out := make([]float32, len(shifted))
	outBytes := uintptr(len(out)) * unsafe.Sizeof(float32(0))
	if bridgeMemcpyD2H(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dAudio), outBytes) != 0 {
		return nil, false
	}
	e.bfoPhase += phaseInc * float64(len(shifted))
	return out, true
}

func (e *Engine) ShiftFilterDecimate(iq []complex64, offsetHz float64, bw float64, outRate int) ([]complex64, int, error) {
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
	if outRate <= 0 {
		return nil, 0, errors.New("invalid output sample rate")
	}
	e.lastShiftUsedGPU = false
	e.lastFIRUsedGPU = false
	e.lastDecimUsedGPU = false
	e.lastDemodUsedGPU = false

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
	if len(taps) == 0 {
		return nil, 0, errors.New("no FIR taps configured")
	}
	decim := int(math.Round(float64(e.sampleRate) / float64(outRate)))
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
	phaseInc := -2.0 * math.Pi * offsetHz / float64(e.sampleRate)
	if bridgeLaunchFreqShift(e.dIQIn, e.dShifted, n, phaseInc, e.phase) != 0 {
		return nil, 0, errors.New("gpu freq shift failed")
	}
	if bridgeLaunchFIR(e.dShifted, e.dFiltered, n, len(taps)) != 0 {
		return nil, 0, errors.New("gpu FIR failed")
	}
	if bridgeLaunchDecimate(e.dFiltered, e.dDecimated, nOut, decim) != 0 {
		return nil, 0, errors.New("gpu decimate failed")
	}
	if bridgeDeviceSync() != 0 {
		return nil, 0, errors.New("cudaDeviceSynchronize failed")
	}
	out := make([]complex64, nOut)
	outBytes := uintptr(nOut) * unsafe.Sizeof(complex64(0))
	if bridgeMemcpyD2H(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dDecimated), outBytes) != 0 {
		return nil, 0, errors.New("cudaMemcpy D2H failed")
	}
	e.phase += phaseInc * float64(n)
	e.lastShiftUsedGPU = true
	e.lastFIRUsedGPU = true
	e.lastDecimUsedGPU = true
	return out, e.sampleRate / decim, nil
}

func (e *Engine) DemodFused(iq []complex64, offsetHz float64, bw float64, mode DemodType) ([]float32, int, error) {
	if e == nil {
		return nil, 0, errors.New("nil CUDA demod engine")
	}
	if !e.cudaReady {
		return nil, 0, errors.New("cuda demod engine is not initialized")
	}
	if len(iq) == 0 {
		return nil, 0, nil
	}
	e.lastShiftUsedGPU = false
	e.lastFIRUsedGPU = false
	e.lastDecimUsedGPU = false
	e.lastDemodUsedGPU = false
	if len(iq) > e.maxSamples {
		return nil, 0, errors.New("sample count exceeds engine capacity")
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
	if len(taps) == 0 {
		return nil, 0, errors.New("no FIR taps configured")
	}
	decim := int(math.Round(float64(e.sampleRate) / float64(outRate)))
	if decim < 1 {
		decim = 1
	}
	n := len(iq)
	nOut := n / decim
	if nOut <= 1 {
		return nil, 0, errors.New("not enough output samples after decimation")
	}
	bytesIn := uintptr(n) * unsafe.Sizeof(complex64(0))
	if bridgeMemcpyH2D(unsafe.Pointer(e.dIQIn), unsafe.Pointer(&iq[0]), bytesIn) != 0 {
		return nil, 0, errors.New("cudaMemcpy H2D failed")
	}
	phaseInc := -2.0 * math.Pi * offsetHz / float64(e.sampleRate)
	phaseStart := e.phase
	phaseEnd := phaseStart + phaseInc*float64(n)
	defer func() {
		e.phase = phaseEnd
	}()
	if bridgeLaunchFreqShift(e.dIQIn, e.dShifted, n, phaseInc, phaseStart) != 0 {
		return nil, 0, errors.New("gpu freq shift failed")
	}
	if bridgeLaunchFIR(e.dShifted, e.dFiltered, n, len(taps)) != 0 {
		return nil, 0, errors.New("gpu FIR failed")
	}
	if bridgeLaunchDecimate(e.dFiltered, e.dDecimated, nOut, decim) != 0 {
		return nil, 0, errors.New("gpu decimate failed")
	}
	e.lastShiftUsedGPU = true
	e.lastFIRUsedGPU = true
	e.lastDecimUsedGPU = true
	e.lastDemodUsedGPU = false
	switch mode {
	case DemodNFM, DemodWFM:
		if bridgeLaunchFMDiscrim(e.dDecimated, e.dAudio, nOut) != 0 {
			return nil, 0, errors.New("gpu FM discrim failed")
		}
		out := make([]float32, nOut-1)
		outBytes := uintptr(len(out)) * unsafe.Sizeof(float32(0))
		if bridgeDeviceSync() != 0 {
			return nil, 0, errors.New("cudaDeviceSynchronize failed")
		}
		if bridgeMemcpyD2H(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dAudio), outBytes) != 0 {
			return nil, 0, errors.New("cudaMemcpy D2H failed")
		}
		e.lastDemodUsedGPU = true
		return out, e.sampleRate / decim, nil
	case DemodAM:
		if bridgeLaunchAMEnvelope(e.dDecimated, e.dAudio, nOut) != 0 {
			return nil, 0, errors.New("gpu AM envelope failed")
		}
		out := make([]float32, nOut)
		outBytes := uintptr(len(out)) * unsafe.Sizeof(float32(0))
		if bridgeDeviceSync() != 0 {
			return nil, 0, errors.New("cudaDeviceSynchronize failed")
		}
		if bridgeMemcpyD2H(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dAudio), outBytes) != 0 {
			return nil, 0, errors.New("cudaMemcpy D2H failed")
		}
		e.lastDemodUsedGPU = true
		return out, e.sampleRate / decim, nil
	case DemodUSB, DemodLSB, DemodCW:
		bfoHz := 700.0
		if mode == DemodLSB {
			bfoHz = -700.0
		}
		phaseBFO := 2.0 * math.Pi * bfoHz / float64(e.sampleRate)
		if bridgeLaunchSSBProduct(e.dDecimated, e.dAudio, nOut, phaseBFO, e.bfoPhase) != 0 {
			return nil, 0, errors.New("gpu SSB product failed")
		}
		out := make([]float32, nOut)
		outBytes := uintptr(len(out)) * unsafe.Sizeof(float32(0))
		if bridgeDeviceSync() != 0 {
			return nil, 0, errors.New("cudaDeviceSynchronize failed")
		}
		if bridgeMemcpyD2H(unsafe.Pointer(&out[0]), unsafe.Pointer(e.dAudio), outBytes) != 0 {
			return nil, 0, errors.New("cudaMemcpy D2H failed")
		}
		e.bfoPhase += phaseBFO * float64(nOut)
		e.lastDemodUsedGPU = true
		return out, e.sampleRate / decim, nil
	default:
		return nil, 0, errors.New("unsupported demod type")
	}
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
	shifted, ok := e.tryCUDAFreqShift(iq, offsetHz)
	if ok {
		if validationEnabled() {
			e.lastShiftUsedGPU = ValidateFreqShift(iq, e.sampleRate, offsetHz, shifted, 1e-3)
			if !e.lastShiftUsedGPU {
				shifted = dsp.FreqShift(iq, e.sampleRate, offsetHz)
			}
		} else {
			e.lastShiftUsedGPU = true
		}
	} else {
		shifted = dsp.FreqShift(iq, e.sampleRate, offsetHz)
		e.lastShiftUsedGPU = false
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
	if ok {
		if validationEnabled() {
			e.lastFIRUsedGPU = ValidateFIR(shifted, taps, filtered, 1e-3)
			if !e.lastFIRUsedGPU {
				ftaps := make([]float64, len(taps))
				for i, v := range taps {
					ftaps[i] = float64(v)
				}
				filtered = dsp.ApplyFIR(shifted, ftaps)
			}
		} else {
			e.lastFIRUsedGPU = true
		}
	}
	if filtered == nil {
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
	if ok {
		if validationEnabled() {
			e.lastDecimUsedGPU = ValidateDecimate(filtered, decim, dec, 1e-3)
			if !e.lastDecimUsedGPU {
				dec = dsp.Decimate(filtered, decim)
			}
		} else {
			e.lastDecimUsedGPU = true
		}
	}
	if dec == nil {
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
		_ = bridgeCudaFree(unsafe.Pointer(e.dIQIn))
		e.dIQIn = nil
	}
	if e.dShifted != nil {
		_ = bridgeCudaFree(unsafe.Pointer(e.dShifted))
		e.dShifted = nil
	}
	if e.dFiltered != nil {
		_ = bridgeCudaFree(unsafe.Pointer(e.dFiltered))
		e.dFiltered = nil
	}
	if e.dDecimated != nil {
		_ = bridgeCudaFree(unsafe.Pointer(e.dDecimated))
		e.dDecimated = nil
	}
	if e.dAudio != nil {
		_ = bridgeCudaFree(unsafe.Pointer(e.dAudio))
		e.dAudio = nil
	}
	e.firTaps = nil
	e.cudaReady = false
}

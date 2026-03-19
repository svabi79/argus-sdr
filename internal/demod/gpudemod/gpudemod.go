//go:build cufft

package gpudemod

/*
#cgo windows LDFLAGS: -lcufft64_12 -lcudart64_13
#include <cuda_runtime.h>
#include <cufft.h>
*/
import "C"

import (
	"errors"
	"math"

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
}

func Available() bool { return true }

func New(maxSamples int, sampleRate int) (*Engine, error) {
	if maxSamples <= 0 {
		return nil, errors.New("invalid maxSamples")
	}
	if sampleRate <= 0 {
		return nil, errors.New("invalid sampleRate")
	}
	return &Engine{maxSamples: maxSamples, sampleRate: sampleRate}, nil
}

func (e *Engine) SetFIR(taps []float32) {
	if len(taps) == 0 {
		e.firTaps = nil
		return
	}
	e.firTaps = append(e.firTaps[:0], taps...)
}

func (e *Engine) Demod(iq []complex64, offsetHz float64, bw float64, mode DemodType) ([]float32, int, error) {
	if e == nil {
		return nil, 0, errors.New("nil CUDA demod engine")
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

	// Phase 1b note:
	// This package now performs real CUDA availability gating and keeps the
	// runtime/CGO boundary in place, but still intentionally falls back to the
	// existing CPU DSP math for signal processing. The next phase should replace
	// the FreqShift + FM discriminator sections below with actual kernel launches.
	_ = fmt.Sprintf("%s:%0.3f", phaseStatus(), offsetHz)
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
	e.firTaps = nil
}

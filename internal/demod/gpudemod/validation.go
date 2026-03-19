//go:build cufft

package gpudemod

import (
	"math/cmplx"

	"sdr-visual-suite/internal/dsp"
)

// ValidateFreqShift compares a candidate shifted IQ stream against the CPU DSP
// reference. This is intended for bring-up while the first real CUDA launch path
// is being wired in.
func ValidateFreqShift(iq []complex64, sampleRate int, offsetHz float64, shifted []complex64, tol float64) bool {
	if len(iq) != len(shifted) {
		return false
	}
	ref := dsp.FreqShift(iq, sampleRate, offsetHz)
	for i := range ref {
		if cmplx.Abs(complex128(ref[i]-shifted[i])) > tol {
			return false
		}
	}
	return true
}

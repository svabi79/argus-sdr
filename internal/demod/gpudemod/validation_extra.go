//go:build cufft

package gpudemod

import (
	"math/cmplx"

	"sdr-visual-suite/internal/dsp"
)

func ValidateFIR(iq []complex64, taps []float32, filtered []complex64, tol float64) bool {
	if len(iq) != len(filtered) {
		return false
	}
	ftaps := make([]float64, len(taps))
	for i, v := range taps {
		ftaps[i] = float64(v)
	}
	ref := dsp.ApplyFIR(iq, ftaps)
	for i := range ref {
		if cmplx.Abs(complex128(ref[i]-filtered[i])) > tol {
			return false
		}
	}
	return true
}

func ValidateDecimate(iq []complex64, factor int, decimated []complex64, tol float64) bool {
	if factor <= 0 {
		return false
	}
	ref := dsp.Decimate(iq, factor)
	if len(ref) != len(decimated) {
		return false
	}
	for i := range ref {
		if cmplx.Abs(complex128(ref[i]-decimated[i])) > tol {
			return false
		}
	}
	return true
}

package fftutil

import "math"

// WelchPSD estimates the power spectral density of iq by Welch's method:
// average the periodograms of overlapping, windowed segments. Averaging reduces
// the spectral variance (a single FFT has ~100% standard deviation per bin),
// which lowers the noise-floor fluctuation and makes weak signals/skirts visible
// at low SNR.
//
// The result is in dB and fftshifted (DC at index segSize/2), matching Spectrum.
// segSize must be > 0; overlap is a fraction in [0,0.95]; window (if len==segSize)
// is applied per segment. If iq is shorter than one segment, falls back to a
// single zero-padded periodogram.
func WelchPSD(iq []complex64, segSize int, overlap float64, window []float64) []float64 {
	if segSize <= 0 {
		return nil
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap > 0.95 {
		overlap = 0.95
	}
	step := int(float64(segSize) * (1 - overlap))
	if step < 1 {
		step = 1
	}

	plan := NewCmplxPlan(segSize)
	in := make([]complex128, segSize)
	out := make([]complex128, segSize)
	acc := make([]float64, segSize)
	invN := 1.0 / float64(segSize)
	segs := 0

	for start := 0; start+segSize <= len(iq); start += step {
		for i := 0; i < segSize; i++ {
			v := iq[start+i]
			w := 1.0
			if len(window) == segSize {
				w = window[i]
			}
			in[i] = complex(float64(real(v))*w, float64(imag(v))*w)
		}
		plan.FFT(out, in)
		for i := 0; i < segSize; i++ {
			mag := cmplxAbs(out[i]) * invN
			acc[i] += mag * mag // linear power
		}
		segs++
	}

	if segs == 0 {
		// Not enough samples for a full segment: zero-pad a single periodogram.
		return Spectrum(iq, window)
	}

	eps := 1e-24
	psd := make([]float64, segSize)
	for i := 0; i < segSize; i++ {
		idx := (i + segSize/2) % segSize
		psd[i] = 10 * math.Log10(acc[idx]/float64(segs)+eps)
	}
	return psd
}

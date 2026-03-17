package fftutil

import (
	"math"

	"gonum.org/v1/gonum/dsp/fourier"
)

func Hann(n int) []float64 {
	w := make([]float64, n)
	if n <= 1 {
		if n == 1 {
			w[0] = 1
		}
		return w
	}
	for i := 0; i < n; i++ {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(n-1)))
	}
	return w
}

func Spectrum(iq []complex64, window []float64) []float64 {
	n := len(iq)
	if n == 0 {
		return nil
	}
	in := make([]complex128, n)
	for i := 0; i < n; i++ {
		v := iq[i]
		w := 1.0
		if len(window) == n {
			w = window[i]
		}
		in[i] = complex(float64(real(v))*w, float64(imag(v))*w)
	}
	fft := fourier.NewCmplxFFT(n)
	out := make([]complex128, n)
	fft.Coefficients(out, in)

	power := make([]float64, n)
	eps := 1e-12
	invN := 1.0 / float64(n)
	for i := 0; i < n; i++ {
		idx := (i + n/2) % n
		mag := cmplxAbs(out[idx]) * invN
		p := 20 * math.Log10(mag+eps)
		power[i] = p
	}
	return power
}

func SpectrumFromFFT(out []complex64) []float64 {
	n := len(out)
	if n == 0 {
		return nil
	}
	power := make([]float64, n)
	eps := 1e-12
	invN := 1.0 / float64(n)
	for i := 0; i < n; i++ {
		idx := (i + n/2) % n
		v := out[idx]
		mag := math.Hypot(float64(real(v)), float64(imag(v))) * invN
		p := 20 * math.Log10(mag+eps)
		power[i] = p
	}
	return power
}

func cmplxAbs(v complex128) float64 {
	return math.Hypot(real(v), imag(v))
}

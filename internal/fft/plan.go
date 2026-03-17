package fftutil

import "gonum.org/v1/gonum/dsp/fourier"

type CmplxPlan struct {
	fft *fourier.CmplxFFT
	n   int
}

func NewCmplxPlan(n int) *CmplxPlan {
	if n <= 0 {
		return &CmplxPlan{}
	}
	return &CmplxPlan{fft: fourier.NewCmplxFFT(n), n: n}
}

func (p *CmplxPlan) N() int { return p.n }

func (p *CmplxPlan) FFT(out, in []complex128) {
	if p == nil || p.fft == nil {
		return
	}
	p.fft.Coefficients(out, in)
}

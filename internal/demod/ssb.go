package demod

import "math"

type USB struct{}

type LSB struct{}

func (USB) Name() string          { return "USB" }
func (LSB) Name() string          { return "LSB" }
func (USB) OutputSampleRate() int { return 48000 }
func (LSB) OutputSampleRate() int { return 48000 }

func (USB) Demod(iq []complex64, sampleRate int) []float32 { return ssb(iq, sampleRate, true) }
func (LSB) Demod(iq []complex64, sampleRate int) []float32 { return ssb(iq, sampleRate, false) }

func ssb(iq []complex64, sampleRate int, usb bool) []float32 {
	if len(iq) == 0 {
		return nil
	}
	out := make([]float32, len(iq))
	bfo := 700.0
	if !usb {
		bfo = -700.0
	}
	phase := 0.0
	inc := 2 * math.Pi * bfo / float64(sampleRate)
	for i := 0; i < len(iq); i++ {
		phase += inc
		c := math.Cos(phase)
		s := math.Sin(phase)
		v := iq[i]
		// product detector
		out[i] = float32(float64(real(v))*c - float64(imag(v))*s)
	}
	return out
}

func init() {
	Register(USB{})
	Register(LSB{})
}

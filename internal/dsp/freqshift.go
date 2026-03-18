package dsp

import "math"

// FreqShift mixes IQ by -offsetHz to shift signal to baseband.
func FreqShift(iq []complex64, sampleRate int, offsetHz float64) []complex64 {
	if len(iq) == 0 || sampleRate <= 0 || offsetHz == 0 {
		out := make([]complex64, len(iq))
		copy(out, iq)
		return out
	}
	out := make([]complex64, len(iq))
	phase := 0.0
	inc := -2 * math.Pi * offsetHz / float64(sampleRate)
	for i := 0; i < len(iq); i++ {
		phase += inc
		re := math.Cos(phase)
		im := math.Sin(phase)
		v := iq[i]
		out[i] = complex(float32(float64(real(v))*re-float64(imag(v))*im), float32(float64(real(v))*im+float64(imag(v))*re))
	}
	return out
}

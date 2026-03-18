package demod

import "math"

type AM struct{}

func (AM) Name() string          { return "AM" }
func (AM) OutputSampleRate() int { return 48000 }
func (AM) Channels() int         { return 1 }

func (AM) Demod(iq []complex64, sampleRate int) []float32 {
	if len(iq) == 0 {
		return nil
	}
	out := make([]float32, len(iq))
	var mean float64
	for i, v := range iq {
		mag := math.Hypot(float64(real(v)), float64(imag(v)))
		mean += mag
		out[i] = float32(mag)
	}
	mean /= float64(len(iq))
	for i := range out {
		out[i] -= float32(mean)
	}
	return out
}

func init() { Register(AM{}) }

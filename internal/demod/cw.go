package demod

import "math"

type CW struct{}

func (CW) Name() string          { return "CW" }
func (CW) OutputSampleRate() int { return 48000 }

func (CW) Demod(iq []complex64, sampleRate int) []float32 {
	if len(iq) == 0 {
		return nil
	}
	out := make([]float32, len(iq))
	bfo := 700.0
	phase := 0.0
	inc := 2 * math.Pi * bfo / float64(sampleRate)
	for i := 0; i < len(iq); i++ {
		phase += inc
		c := math.Cos(phase)
		v := iq[i]
		out[i] = float32(float64(real(v)) * c)
	}
	return out
}

func init() { Register(CW{}) }

package demod

import "math"

type NFM struct{}

type WFM struct{}

func (NFM) Name() string          { return "NFM" }
func (WFM) Name() string          { return "WFM" }
func (NFM) OutputSampleRate() int { return 48000 }
func (WFM) OutputSampleRate() int { return 192000 }

func (NFM) Demod(iq []complex64, sampleRate int) []float32 {
	return fmDiscrim(iq)
}

func (WFM) Demod(iq []complex64, sampleRate int) []float32 {
	return fmDiscrim(iq)
}

func fmDiscrim(iq []complex64) []float32 {
	if len(iq) < 2 {
		return nil
	}
	out := make([]float32, len(iq)-1)
	for i := 1; i < len(iq); i++ {
		p := iq[i-1]
		c := iq[i]
		num := float64(real(p))*float64(imag(c)) - float64(imag(p))*float64(real(c))
		den := float64(real(p))*float64(real(c)) + float64(imag(p))*float64(imag(c))
		out[i-1] = float32(math.Atan2(num, den))
	}
	return out
}

func init() {
	Register(NFM{})
	Register(WFM{})
}

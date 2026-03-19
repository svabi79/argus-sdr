package recorder

import (
	"math"

	"sdr-visual-suite/internal/demod"
	"sdr-visual-suite/internal/dsp"
)

func demodAudioCPU(d demod.Demodulator, iq []complex64, sampleRate int, offset float64, bw float64) ([]float32, int) {
	shifted := dsp.FreqShift(iq, sampleRate, offset)
	cutoff := bw / 2
	if cutoff < 200 {
		cutoff = 200
	}
	taps := dsp.LowpassFIR(cutoff, sampleRate, 101)
	filtered := dsp.ApplyFIR(shifted, taps)
	decim := int(math.Round(float64(sampleRate) / float64(d.OutputSampleRate())))
	if decim < 1 {
		decim = 1
	}
	dec := dsp.Decimate(filtered, decim)
	inputRate := sampleRate / decim
	return d.Demod(dec, inputRate), inputRate
}

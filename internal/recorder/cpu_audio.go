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
	// For WFM, ensure we capture the full FM composite (at least 75 kHz)
	if cutoff < 75000 && d.OutputSampleRate() >= 192000 {
		cutoff = 75000
	}
	taps := dsp.LowpassFIR(cutoff, sampleRate, 101)
	filtered := dsp.ApplyFIR(shifted, taps)

	// First decimation: get to a demod-friendly rate
	demodRate := d.OutputSampleRate()
	decim1 := int(math.Round(float64(sampleRate) / float64(demodRate)))
	if decim1 < 1 {
		decim1 = 1
	}
	dec := dsp.Decimate(filtered, decim1)
	actualDemodRate := sampleRate / decim1

	// Demodulate at the intermediate rate
	audio := d.Demod(dec, actualDemodRate)

	// Second decimation: resample to exactly 48 kHz for browser playback
	const outputRate = 48000
	if actualDemodRate > outputRate {
		// Anti-alias low-pass before decimation
		aaTaps := dsp.LowpassFIR(float64(outputRate)/2.0*0.9, actualDemodRate, 63)
		channels := d.Channels()
		if channels > 1 {
			// For stereo: de-interleave, filter, decimate, re-interleave
			nFrames := len(audio) / channels
			left := make([]float32, nFrames)
			right := make([]float32, nFrames)
			for i := 0; i < nFrames; i++ {
				left[i] = audio[i*2]
				right[i] = audio[i*2+1]
			}
			left = dsp.ApplyFIRReal(left, aaTaps)
			right = dsp.ApplyFIRReal(right, aaTaps)
			decim2 := int(math.Round(float64(actualDemodRate) / float64(outputRate)))
			if decim2 < 1 {
				decim2 = 1
			}
			outFrames := nFrames / decim2
			resampled := make([]float32, outFrames*2)
			for i := 0; i < outFrames; i++ {
				resampled[i*2] = left[i*decim2]
				resampled[i*2+1] = right[i*decim2]
			}
			return resampled, actualDemodRate / decim2
		}
		audio = dsp.ApplyFIRReal(audio, aaTaps)
		decim2 := int(math.Round(float64(actualDemodRate) / float64(outputRate)))
		if decim2 < 1 {
			decim2 = 1
		}
		resampled := make([]float32, 0, len(audio)/decim2+1)
		for i := 0; i < len(audio); i += decim2 {
			resampled = append(resampled, audio[i])
		}
		return resampled, actualDemodRate / decim2
	}

	return audio, actualDemodRate
}

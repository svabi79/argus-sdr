package recorder

import (
	"math"

	"sdr-wideband-suite/internal/demod"
	"sdr-wideband-suite/internal/dsp"
)

func demodWFMStereoBatchAudio(iq []complex64, sampleRate int, offset float64, bw float64, deemphasisUs float64) ([]float32, int) {
	if len(iq) == 0 || sampleRate <= 0 {
		return nil, 0
	}

	shifted := dsp.FreqShift(iq, sampleRate, offset)
	cutoff := bw / 2
	if cutoff < 200 {
		cutoff = 200
	}
	d := demod.Get("WFM")
	if d == nil {
		return nil, 0
	}
	if cutoff < 75000 && d.OutputSampleRate() >= 192000 {
		cutoff = 75000
	}
	taps := dsp.LowpassFIR(cutoff, sampleRate, 101)
	filtered := dsp.ApplyFIR(shifted, taps)

	demodRate := d.OutputSampleRate()
	decim1 := int(math.Round(float64(sampleRate) / float64(demodRate)))
	if decim1 < 1 {
		decim1 = 1
	}
	dec := dsp.Decimate(filtered, decim1)
	actualDemodRate := sampleRate / decim1
	mono := d.Demod(dec, actualDemodRate)
	if len(mono) == 0 {
		return nil, 0
	}

	stereo, locked := wfmStereoDecodeBatch(mono, actualDemodRate)
	if !locked || len(stereo) == 0 {
		stereo = make([]float32, len(mono)*2)
		for i, s := range mono {
			stereo[i*2] = s
			stereo[i*2+1] = s
		}
	}

	outRate := actualDemodRate
	if actualDemodRate != streamAudioRate {
		resampler := dsp.NewStereoResampler(actualDemodRate, streamAudioRate, resamplerTaps)
		stereo = resampler.Process(stereo)
		outRate = streamAudioRate
	}

	if deemphasisUs > 0 && outRate > 0 {
		tau := deemphasisUs * 1e-6
		alpha := math.Exp(-1.0 / (float64(outRate) * tau))
		var yL, yR float64
		for i := 0; i+1 < len(stereo); i += 2 {
			yL = alpha*yL + (1-alpha)*float64(stereo[i])
			stereo[i] = float32(yL)
			yR = alpha*yR + (1-alpha)*float64(stereo[i+1])
			stereo[i+1] = float32(yR)
		}
	}

	for i := range stereo {
		stereo[i] *= 0.35
	}

	return stereo, outRate
}

func wfmStereoDecodeBatch(mono []float32, sampleRate int) ([]float32, bool) {
	if len(mono) == 0 || sampleRate <= 0 {
		return nil, false
	}

	lp := dsp.LowpassFIR(15000, sampleRate, 101)
	lpr := dsp.ApplyFIRReal(mono, lp)

	hi := dsp.ApplyFIRReal(mono, dsp.LowpassFIR(53000, sampleRate, 101))
	lo := dsp.ApplyFIRReal(mono, dsp.LowpassFIR(23000, sampleRate, 101))
	bpf := make([]float32, len(mono))
	for i := range bpf {
		bpf[i] = hi[i] - lo[i]
	}

	pilotHi := dsp.ApplyFIRReal(mono, dsp.LowpassFIR(21000, sampleRate, 101))
	pilotLo := dsp.ApplyFIRReal(mono, dsp.LowpassFIR(17000, sampleRate, 101))
	pilot := make([]float32, len(mono))
	for i := range pilot {
		pilot[i] = pilotHi[i] - pilotLo[i]
	}

	phase := 0.0
	freq := 2 * math.Pi * 19000 / float64(sampleRate)
	alpha, beta := pllCoefficientsBatch(50, 0.707, sampleRate)
	lpAlpha := 1 - math.Exp(-2*math.Pi*200/float64(sampleRate))
	iState := 0.0
	qState := 0.0
	minFreq := 2 * math.Pi * 17000 / float64(sampleRate)
	maxFreq := 2 * math.Pi * 21000 / float64(sampleRate)
	var pilotPower float64
	var totalPower float64
	var errSum float64

	lr := make([]float32, len(mono))
	for i := 0; i < len(mono); i++ {
		p := float64(pilot[i])
		sinP, cosP := math.Sincos(phase)
		iMix := p * cosP
		qMix := p * -sinP
		iState += lpAlpha * (iMix - iState)
		qState += lpAlpha * (qMix - qState)
		err := math.Atan2(qState, iState)
		freq += beta * err
		if freq < minFreq {
			freq = minFreq
		} else if freq > maxFreq {
			freq = maxFreq
		}
		phase += freq + alpha*err
		if phase > 2*math.Pi {
			phase -= 2 * math.Pi
		} else if phase < 0 {
			phase += 2 * math.Pi
		}

		totalPower += float64(mono[i]) * float64(mono[i])
		pilotPower += p * p
		errSum += math.Abs(err)

		lr[i] = bpf[i] * float32(2*math.Sin(2*phase))
	}

	lr = dsp.ApplyFIRReal(lr, lp)

	pilotRatio := 0.0
	if totalPower > 0 {
		pilotRatio = pilotPower / totalPower
	}
	freqHz := freq * float64(sampleRate) / (2 * math.Pi)
	blockErr := errSum / float64(len(mono))
	locked := pilotRatio > 0.003 && math.Abs(freqHz-19000) < 250 && blockErr < 0.35

	out := make([]float32, len(lpr)*2)
	for i := range lpr {
		out[i*2] = 0.5 * (lpr[i] + lr[i])
		out[i*2+1] = 0.5 * (lpr[i] - lr[i])
	}
	return out, locked
}

func pllCoefficientsBatch(loopBW, damping float64, sampleRate int) (float64, float64) {
	if sampleRate <= 0 || loopBW <= 0 {
		return 0, 0
	}
	bl := loopBW / float64(sampleRate)
	theta := bl / (damping + 0.25/damping)
	d := 1 + 2*damping*theta + theta*theta
	alpha := (4 * damping * theta) / d
	beta := (4 * theta * theta) / d
	return alpha, beta
}

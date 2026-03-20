package classifier

import "math"

type PLLResult struct {
	ExactHz     float64 `json:"exact_hz"`
	OffsetHz    float64 `json:"offset_hz"`
	Locked      bool    `json:"locked"`
	Method      string  `json:"method"`
	PrecisionHz float64 `json:"precision_hz"`
	Stereo      bool    `json:"stereo,omitempty"`
	RDSStation  string  `json:"rds_station,omitempty"`
}

func EstimateExactFrequency(iq []complex64, sampleRate int, detectedHz float64, modType SignalClass) PLLResult {
	if len(iq) < 256 {
		return PLLResult{ExactHz: detectedHz}
	}
	switch modType {
	case ClassWFM:
		return estimateWFMPilot(iq, sampleRate, detectedHz)
	case ClassAM:
		return estimateAMCarrier(iq, sampleRate, detectedHz)
	case ClassNFM:
		return estimateNFMCarrier(iq, sampleRate, detectedHz)
	case ClassCW:
		return estimateCWTone(iq, sampleRate, detectedHz)
	default:
		return PLLResult{ExactHz: detectedHz, Method: "none"}
	}
}

func estimateWFMPilot(iq []complex64, sampleRate int, detectedHz float64) PLLResult {
	if sampleRate < 40000 {
		return PLLResult{ExactHz: detectedHz, Method: "pilot", Locked: false}
	}
	demod := fmDemod(iq)
	if len(demod) == 0 {
		return PLLResult{ExactHz: detectedHz, Method: "pilot"}
	}
	pilotFreq := 19000.0
	bestFreq := pilotFreq
	bestMag := goertzelMagnitude(demod, pilotFreq, sampleRate)
	for offset := -50.0; offset <= 50.0; offset += 1.0 {
		mag := goertzelMagnitude(demod, pilotFreq+offset, sampleRate)
		if mag > bestMag {
			bestMag = mag
			bestFreq = pilotFreq + offset
		}
	}
	freqError := bestFreq - 19000.0
	noiseMag := goertzelMagnitude(demod, 17500, sampleRate)
	locked := bestMag > noiseMag*5
	if !locked {
		return PLLResult{ExactHz: detectedHz, Method: "pilot", Locked: false}
	}
	return PLLResult{ExactHz: detectedHz - freqError, OffsetHz: -freqError, Locked: true, Method: "pilot", PrecisionHz: 1.0, Stereo: true}
}

func estimateAMCarrier(iq []complex64, sampleRate int, detectedHz float64) PLLResult {
	offset := meanInstFreqHz(iq, sampleRate)
	locked := math.Abs(offset) < 5000 // Only lock if offset is plausible (<5 kHz)
	return PLLResult{ExactHz: detectedHz + offset, OffsetHz: offset, Locked: locked, Method: "carrier", PrecisionHz: 5.0}
}

func estimateNFMCarrier(iq []complex64, sampleRate int, detectedHz float64) PLLResult {
	offset := meanInstFreqHz(iq, sampleRate)
	return PLLResult{ExactHz: detectedHz + offset, OffsetHz: offset, Locked: math.Abs(offset) < 5000, Method: "fm_dc", PrecisionHz: 20.0}
}

func estimateCWTone(iq []complex64, sampleRate int, detectedHz float64) PLLResult {
	demod := fmDemod(iq)
	if len(demod) == 0 {
		return PLLResult{ExactHz: detectedHz, Method: "tone"}
	}
	bestFreq := 700.0
	bestMag := 0.0
	for f := 300.0; f <= 1200.0; f += 1.0 {
		mag := goertzelMagnitude(demod, f, sampleRate)
		if mag > bestMag {
			bestMag = mag
			bestFreq = f
		}
	}
	bfoHz := 700.0
	toneOffset := bestFreq - bfoHz
	return PLLResult{ExactHz: detectedHz + toneOffset, OffsetHz: toneOffset, Locked: bestMag > 0, Method: "tone", PrecisionHz: 2.0}
}

func fmDemod(iq []complex64) []float64 {
	if len(iq) < 2 {
		return nil
	}
	out := make([]float64, len(iq)-1)
	for i := 1; i < len(iq); i++ {
		p := iq[i-1]
		c := iq[i]
		num := float64(real(p))*float64(imag(c)) - float64(imag(p))*float64(real(c))
		den := float64(real(p))*float64(real(c)) + float64(imag(p))*float64(imag(c))
		out[i-1] = math.Atan2(num, den)
	}
	return out
}

func goertzelMagnitude(samples []float64, targetHz float64, sampleRate int) float64 {
	n := len(samples)
	if n == 0 {
		return 0
	}
	k := targetHz / (float64(sampleRate) / float64(n))
	w := 2.0 * math.Pi * k / float64(n)
	coeff := 2.0 * math.Cos(w)
	s1, s2 := 0.0, 0.0
	for _, v := range samples {
		s0 := v + coeff*s1 - s2
		s2 = s1
		s1 = s0
	}
	return math.Sqrt(s1*s1 + s2*s2 - coeff*s1*s2)
}

func meanInstFreqHz(iq []complex64, sampleRate int) float64 {
	if len(iq) < 2 {
		return 0
	}
	var sum float64
	for i := 1; i < len(iq); i++ {
		p := iq[i-1]
		c := iq[i]
		num := float64(real(p))*float64(imag(c)) - float64(imag(p))*float64(real(c))
		den := float64(real(p))*float64(real(c)) + float64(imag(p))*float64(imag(c))
		sum += math.Atan2(num, den)
	}
	meanRad := sum / float64(len(iq)-1)
	return meanRad * float64(sampleRate) / (2.0 * math.Pi)
}

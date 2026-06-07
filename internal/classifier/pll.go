package classifier

import (
	"log"
	"math"
	"os"
	"sort"
	"strconv"
)

// pilotLockRatio is the min (pilot magnitude / band-median) to declare a stereo
// lock. INTERIM placeholder: with the current ~512-sample PLL snippet the pilot
// cannot be reliably separated from noise (a real station's ratio ~2 is below
// noise spikes ~5), so this is set high to avoid false-lock spam rather than to
// lock real stations. The real fix is a longer integration window (OI-24);
// recalibrate this against the replay capture once that lands.
// Override at runtime with SDRD_PILOT_RATIO.
var pilotLockRatio = 4.0

var pllDebug = os.Getenv("SDRD_PLL_DEBUG") != ""

func init() {
	if v := os.Getenv("SDRD_PILOT_RATIO"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			pilotLockRatio = f
		}
	}
}

func pllDebugLog(detHz, ratio, freqErr float64, n int) {
	log.Printf("PLL pilot: detHz=%.0f ratio=%.1f freqErr=%.0f demodN=%d", detHz, ratio, freqErr, n)
}

func medianFloats(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	return c[len(c)/2]
}

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
	case ClassWFM, ClassWFMStereo:
		// The refinement layer upgrades WFM -> WFM_STEREO *before* calling this,
		// so the stereo class must take the pilot path too — otherwise it falls
		// through to default and the 19 kHz pilot is never looked for (no lock).
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
	// Search a window around 19 kHz wide enough to tolerate residual carrier-
	// center jitter (~1 kHz) without flickering. Because we take the max over many
	// frequencies, the lock test must compare it against a ROBUST noise floor (the
	// median magnitude across the band) — comparing the max to a single off-pilot
	// point would false-lock on the strongest noise bin of a weak/empty signal.
	pilotFreq := 19000.0
	bestFreq := pilotFreq
	bestMag := 0.0
	mags := make([]float64, 0, 256)
	for offset := -1000.0; offset <= 1000.0; offset += 10.0 {
		mag := goertzelMagnitude(demod, pilotFreq+offset, sampleRate)
		mags = append(mags, mag)
		if mag > bestMag {
			bestMag = mag
			bestFreq = pilotFreq + offset
		}
	}
	freqError := bestFreq - 19000.0
	// Median of the search band ≈ the noise floor (only ~1 bin is the pilot).
	noiseMed := medianFloats(mags)
	ratio := 0.0
	if noiseMed > 0 {
		ratio = bestMag / noiseMed
	}
	if pllDebug {
		pllDebugLog(detectedHz, ratio, bestFreq-19000.0, len(demod))
	}
	// A real stereo pilot is a discrete tone tens of dB over the floor; noise
	// maxima sit only a few× above the median. Require a wide margin.
	locked := noiseMed > 0 && ratio > pilotLockRatio
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

package classifier

import (
	"math"
)

// ExtractFeatures computes spectral features for a signal slice.
// spectrum is full-band power in dB (length fftSize).
func ExtractFeatures(s SignalInput, spectrum []float64, sampleRate int, fftSize int) Features {
	if fftSize <= 0 {
		fftSize = len(spectrum)
	}
	if len(spectrum) == 0 || s.FirstBin < 0 || s.LastBin < s.FirstBin || s.FirstBin >= len(spectrum) {
		return Features{}
	}
	if s.LastBin >= len(spectrum) {
		s.LastBin = len(spectrum) - 1
	}
	binHz := float64(sampleRate) / float64(max(1, fftSize))
	// slice
	start := s.FirstBin
	end := s.LastBin
	peakDb := -1e9
	peakIdx := start
	sumLin := 0.0
	geoSum := 0.0
	count := 0
	for i := start; i <= end; i++ {
		db := spectrum[i]
		if db > peakDb {
			peakDb = db
			peakIdx = i
		}
		p := math.Pow(10, db/10.0)
		sumLin += p
		if p > 0 {
			geoSum += math.Log(p)
		}
		count++
	}
	avgLin := 0.0
	if count > 0 {
		avgLin = sumLin / float64(count)
	}
	// Peak-to-avg in dB
	peakToAvg := 0.0
	if avgLin > 0 {
		peakToAvg = 10 * math.Log10(math.Pow(10, peakDb/10.0)/avgLin)
	}
	// Spectral flatness
	flat := 0.0
	if count > 0 && avgLin > 0 {
		geoMean := math.Exp(geoSum / float64(count))
		flat = geoMean / avgLin
	}
	// BW3dB
	bw3 := bwAtThreshold(spectrum, start, end, peakDb-3.0) * binHz
	// BW90 (90% energy)
	bw90 := bwEnergy(spectrum, start, end, 0.90) * binHz
	// Symmetry (power left/right of peak)
	leftSum, rightSum := 0.0, 0.0
	for i := start; i <= end; i++ {
		p := math.Pow(10, spectrum[i]/10.0)
		if i <= peakIdx {
			leftSum += p
		} else {
			rightSum += p
		}
	}
	sym := 0.0
	if leftSum+rightSum > 0 {
		sym = (rightSum - leftSum) / (rightSum + leftSum)
	}
	// Rolloff (dB/kHz) at edges
	leftDb := spectrum[start]
	rightDb := spectrum[end]
	leftHz := math.Max(binHz, float64(peakIdx-start)*binHz)
	rightHz := math.Max(binHz, float64(end-peakIdx)*binHz)
	rollL := (peakDb - leftDb) / (leftHz / 1e3)
	rollR := (peakDb - rightDb) / (rightHz / 1e3)

	return Features{
		BW3dB:        bw3,
		BW90:         bw90,
		SpectralFlat: clamp01(flat),
		PeakToAvg:    peakToAvg,
		Symmetry:     sym,
		RolloffLeft:  rollL,
		RolloffRight: rollR,
	}
}

func bwAtThreshold(spectrum []float64, start, end int, threshDb float64) float64 {
	left := start
	right := end
	for i := start; i <= end; i++ {
		if spectrum[i] >= threshDb {
			left = i
			break
		}
	}
	for i := end; i >= start; i-- {
		if spectrum[i] >= threshDb {
			right = i
			break
		}
	}
	if right < left {
		return float64(end - start + 1)
	}
	return float64(right - left + 1)
}

func bwEnergy(spectrum []float64, start, end int, frac float64) float64 {
	if frac <= 0 {
		return 0
	}
	if frac > 1 {
		frac = 1
	}
	powers := make([]float64, 0, end-start+1)
	sum := 0.0
	for i := start; i <= end; i++ {
		p := math.Pow(10, spectrum[i]/10.0)
		sum += p
		powers = append(powers, p)
	}
	if sum == 0 {
		return float64(end - start + 1)
	}
	// accumulate from center outward
	center := (start + end) / 2
	l := center
	r := center
	acc := powers[center-start]
	for acc/sum < frac && (l > start || r < end) {
		if l > start {
			l--
			acc += powers[l-start]
		}
		if acc/sum >= frac {
			break
		}
		if r < end {
			r++
			acc += powers[r-start]
		}
	}
	return float64(r - l + 1)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

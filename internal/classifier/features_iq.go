package classifier

import (
	"math"
)

// ExtractTemporalFeatures computes simple time-domain features from IQ.
func ExtractTemporalFeatures(iq []complex64) (envVar float64, zeroCross float64, instFreqStd float64, crest float64) {
	if len(iq) == 0 {
		return 0, 0, 0, 0
	}
	env := make([]float64, len(iq))
	var mean, rms float64
	for i, v := range iq {
		a := math.Hypot(float64(real(v)), float64(imag(v)))
		env[i] = a
		mean += a
		rms += a * a
	}
	mean /= float64(len(iq))
	rms = math.Sqrt(rms / float64(len(iq)))
	// normalized env variance (coefficient of variation squared)
	var sumVar float64
	for _, v := range env {
		d := v - mean
		sumVar += d * d
	}
	if mean > 1e-12 {
		envVar = (sumVar / float64(len(iq))) / (mean * mean)
	} else {
		envVar = 0
	}
	if rms > 0 {
		crest = maxFloat(env) / rms
	}
	// zero-crossing on real part
	zc := 0
	for i := 1; i < len(iq); i++ {
		p := real(iq[i-1])
		c := real(iq[i])
		if (p >= 0 && c < 0) || (p < 0 && c >= 0) {
			zc++
		}
	}
	zeroCross = float64(zc) / float64(len(iq))
	// instantaneous frequency std
	if len(iq) > 1 {
		var sum, sumSq float64
		for i := 1; i < len(iq); i++ {
			p := iq[i-1]
			c := iq[i]
			num := float64(real(p))*float64(imag(c)) - float64(imag(p))*float64(real(c))
			den := float64(real(p))*float64(real(c)) + float64(imag(p))*float64(imag(c))
			v := math.Atan2(num, den)
			sum += v
			sumSq += v * v
		}
		n := float64(len(iq) - 1)
		mean := sum / n
		instFreqStd = math.Sqrt(sumSq/n - mean*mean)
	}
	return
}

func maxFloat(vals []float64) float64 {
	m := vals[0]
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}

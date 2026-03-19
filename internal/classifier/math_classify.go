package classifier

import "math"

type MathFeatures struct {
	EnvCoV        float64 `json:"env_cov"`
	EnvKurtosis   float64 `json:"env_kurtosis"`
	InstFreqStd   float64 `json:"inst_freq_std"`
	InstFreqRange float64 `json:"inst_freq_range"`
	AMIndex       float64 `json:"am_index"`
	FMIndex       float64 `json:"fm_index"`
	InstFreqModes int     `json:"inst_freq_modes"`
}

func ExtractMathFeatures(iq []complex64) MathFeatures {
	if len(iq) < 10 {
		return MathFeatures{}
	}
	n := len(iq)
	env := make([]float64, n)
	var envMean float64
	for i, v := range iq {
		a := math.Hypot(float64(real(v)), float64(imag(v)))
		env[i] = a
		envMean += a
	}
	envMean /= float64(n)
	var envVar, envM4 float64
	for _, a := range env {
		d := a - envMean
		envVar += d * d
		envM4 += d * d * d * d
	}
	envVar /= float64(n)
	envM4 /= float64(n)
	envStd := math.Sqrt(envVar)
	envCoV := 0.0
	if envMean > 1e-12 {
		envCoV = envStd / envMean
	}
	envKurtosis := 0.0
	if envVar > 1e-20 {
		envKurtosis = envM4 / (envVar * envVar)
	}
	instFreq := make([]float64, n-1)
	var ifMean float64
	ifMin := math.Inf(1)
	ifMax := math.Inf(-1)
	for i := 1; i < n; i++ {
		p := iq[i-1]
		c := iq[i]
		num := float64(real(p))*float64(imag(c)) - float64(imag(p))*float64(real(c))
		den := float64(real(p))*float64(real(c)) + float64(imag(p))*float64(imag(c))
		f := math.Atan2(num, den)
		instFreq[i-1] = f
		ifMean += f
		if f < ifMin {
			ifMin = f
		}
		if f > ifMax {
			ifMax = f
		}
	}
	ifMean /= float64(n - 1)
	var ifVar float64
	for _, f := range instFreq {
		d := f - ifMean
		ifVar += d * d
	}
	ifVar /= float64(n - 1)
	ifStd := math.Sqrt(ifVar)
	ifRange := ifMax - ifMin
	modes := countHistogramPeaks(instFreq, 32)
	amIndex := envCoV / math.Max(ifStd, 0.001)
	fmIndex := ifStd / math.Max(envCoV, 0.001)
	return MathFeatures{
		EnvCoV:        envCoV,
		EnvKurtosis:   envKurtosis,
		InstFreqStd:   ifStd,
		InstFreqRange: ifRange,
		AMIndex:       amIndex,
		FMIndex:       fmIndex,
		InstFreqModes: modes,
	}
}

func countHistogramPeaks(vals []float64, bins int) int {
	if len(vals) == 0 || bins < 3 {
		return 0
	}
	minV, maxV := vals[0], vals[0]
	for _, v := range vals {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	span := maxV - minV
	if span < 1e-10 {
		return 1
	}
	hist := make([]int, bins)
	for _, v := range vals {
		idx := int(float64(bins-1) * (v - minV) / span)
		if idx >= bins {
			idx = bins - 1
		}
		if idx < 0 {
			idx = 0
		}
		hist[idx]++
	}
	smooth := make([]int, bins)
	maxSmooth := 0
	for i := range hist {
		s := hist[i]
		if i > 0 {
			s += hist[i-1]
		}
		if i < bins-1 {
			s += hist[i+1]
		}
		smooth[i] = s
		if s > maxSmooth {
			maxSmooth = s
		}
	}
	peaks := 0
	for i := 1; i < bins-1; i++ {
		if smooth[i] > smooth[i-1] && smooth[i] > smooth[i+1] {
			if float64(smooth[i]) > 0.1*float64(maxSmooth) {
				peaks++
			}
		}
	}
	if peaks == 0 {
		peaks = 1
	}
	return peaks
}

func MathClassify(mf MathFeatures, bw float64, centerHz float64, snrDb float64) Classification {
	scores := map[SignalClass]float64{}
	if bw < 500 && mf.InstFreqStd < 0.15 {
		scores[ClassCW] += 3.0
	}
	if mf.AMIndex > 3.0 {
		scores[ClassAM] += 2.0
	} else if mf.AMIndex > 1.5 {
		scores[ClassAM] += 1.0
	}
	if mf.FMIndex > 5.0 && mf.EnvCoV < 0.1 {
		if bw >= 80e3 {
			scores[ClassWFM] += 2.5
		} else if bw >= 6e3 {
			scores[ClassNFM] += 2.5
		} else {
			scores[ClassNFM] += 1.5
		}
	} else if mf.FMIndex > 2.0 && mf.EnvCoV < 0.15 {
		if bw >= 50e3 {
			scores[ClassWFM] += 1.5
		} else {
			scores[ClassNFM] += 1.5
		}
	}
	if mf.AMIndex > 0.5 && mf.AMIndex < 3.0 && mf.FMIndex > 0.5 && mf.FMIndex < 3.0 {
		if bw >= 2000 && bw <= 4000 {
			scores[ClassSSBUSB] += 1.5
			scores[ClassSSBLSB] += 1.5
		}
	}
	if bw < 500 && mf.EnvKurtosis > 5.0 && mf.InstFreqStd < 0.1 {
		scores[ClassCW] += 2.5
	} else if bw < 200 && mf.InstFreqStd < 0.15 {
		scores[ClassCW] += 1.5
	}
	if bw < 500 {
		scores[ClassAM] *= 0.4
	}
	if mf.EnvCoV < 0.05 && mf.InstFreqModes >= 2 {
		if bw >= 10000 && bw <= 14000 {
			scores[ClassDMR] += 2.0
		} else if bw >= 5000 && bw <= 8000 {
			scores[ClassDStar] += 1.8
		} else {
			scores[ClassFSK] += 1.5
		}
	}
	if mf.EnvCoV < 0.08 && mf.InstFreqModes <= 1 && mf.InstFreqStd < 0.3 {
		if bw >= 100 && bw < 500 {
			scores[ClassWSPR] += 1.3
		}
		if bw >= 100 && bw < 3000 {
			scores[ClassPSK] += 1.0
		}
	}
	if mf.EnvCoV < 0.15 && mf.InstFreqModes >= 3 && bw >= 2000 && bw < 3500 {
		scores[ClassFT8] += 1.8
	}
	if mf.AMIndex < 0.5 && mf.FMIndex < 0.5 && bw > 2000 {
		scores[ClassNoise] += 1.0
	}
	best, _, second, _ := top2(scores)
	if best == "" {
		best = ClassUnknown
	}
	if second == "" {
		second = ClassUnknown
	}
	conf := softmaxConfidence(scores, best)
	if snrDb < 20 {
		snrFactor := clamp01((snrDb - 3) / 17.0)
		conf *= 0.3 + 0.7*snrFactor
	}
	if math.IsNaN(conf) || conf <= 0 {
		conf = 0.1
	}
	return Classification{
		ModType:      best,
		Confidence:   conf,
		BW3dB:        bw,
		SecondBest:   second,
		Scores:       scores,
		MathFeatures: &mf,
	}
}

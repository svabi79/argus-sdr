package classifier

import "math"

func RuleClassify(feat Features) Classification {
	bw := feat.BW3dB
	flat := feat.SpectralFlat
	sym := feat.Symmetry
	p2a := feat.PeakToAvg

	scores := map[SignalClass]float64{}
	add := func(c SignalClass, w float64) {
		if w <= 0 {
			return
		}
		scores[c] += w
	}

	switch {
	case bw >= 80e3:
		add(ClassWFM, 2.2)
	case bw >= 25e3 && bw < 80e3:
		add(ClassWFM, 1.4)
		add(ClassNFM, 0.8)
	case bw >= 6e3 && bw < 25e3:
		add(ClassNFM, 2.0)
	case bw >= 3e3 && bw < 6e3:
		add(ClassSSBUSB, 0.6)
		add(ClassSSBLSB, 0.6)
		if p2a > 2.5 && flat < 0.5 {
			add(ClassAM, 1.1)
		}
	case bw >= 500 && bw < 3e3:
		add(ClassSSBUSB, 0.8)
		add(ClassSSBLSB, 0.8)
		if p2a > 3 && flat < 0.4 {
			add(ClassAM, 1.0)
		}
	case bw >= 150 && bw < 500:
		add(ClassFSK, 0.5)
		add(ClassPSK, 0.5)
	case bw < 150:
		add(ClassCW, 1.6)
	}

	if sym > 0.2 {
		add(ClassSSBUSB, 1.2)
	} else if sym < -0.2 {
		add(ClassSSBLSB, 1.2)
	}
	if feat.EnvVariance < 0.6 && feat.InstFreqStd < 0.7 && bw >= 2000 && bw < 3000 {
		add(ClassFT8, 1.4)
	}
	if feat.EnvVariance < 0.4 && feat.InstFreqStd < 0.5 && bw >= 150 && bw < 500 {
		add(ClassWSPR, 1.3)
	}
	if feat.InstFreqStd > 0.9 {
		add(ClassFSK, 1.2)
	} else if feat.InstFreqStd < 0.25 {
		add(ClassPSK, 1.0)
	}
	if p2a > 2.5 && flat < 0.5 {
		add(ClassAM, 0.8)
	}
	if flat > 0.85 && bw > 2e3 {
		add(ClassNoise, 1.0)
	}
	if feat.InstFreqStd < 0.5 && feat.EnvVariance < 0.3 && bw >= 6e3 && bw < 25e3 {
		add(ClassDMR, 0.7)
	}

	best, _, second, _ := top2(scores)
	if best == "" {
		best = ClassUnknown
	}
	if second == "" {
		second = ClassUnknown
	}

	conf := softmaxConfidence(scores, best)
	if best == ClassNFM || best == ClassWFM {
		conf = conf * (0.8 + 0.2*clamp01(1-flat))
	}
	if best == ClassAM {
		conf = conf * (0.7 + 0.3*clamp01(p2a/6.0))
	}
	if math.IsNaN(conf) || conf <= 0 {
		conf = 0.1
	}

	if (best == ClassSSBUSB || best == ClassSSBLSB) && second == ClassUnknown {
		if best == ClassSSBUSB {
			second = ClassSSBLSB
		} else {
			second = ClassSSBUSB
		}
	}

	return Classification{
		ModType:    best,
		Confidence: conf,
		BW3dB:      bw,
		Features:   feat,
		SecondBest: second,
		Scores:     scores,
	}
}

func softmaxConfidence(scores map[SignalClass]float64, best SignalClass) float64 {
	if len(scores) == 0 || best == "" || best == ClassUnknown {
		return 0.1
	}
	maxScore := math.Inf(-1)
	for _, v := range scores {
		if v > maxScore {
			maxScore = v
		}
	}
	if math.IsInf(maxScore, -1) {
		return 0.1
	}
	var expSum float64
	var expBest float64
	for k, v := range scores {
		e := math.Exp(v - maxScore)
		expSum += e
		if k == best {
			expBest = e
		}
	}
	if expSum <= 0 {
		return 0.1
	}
	return expBest / expSum
}

func top2(scores map[SignalClass]float64) (SignalClass, float64, SignalClass, float64) {
	var best, second SignalClass
	bestScore := 0.0
	secondScore := 0.0
	better := func(k SignalClass, v float64, cur SignalClass, curV float64) bool {
		if v > curV {
			return true
		}
		if v < curV {
			return false
		}
		if cur == "" {
			return true
		}
		return string(k) < string(cur)
	}
	for k, v := range scores {
		if better(k, v, best, bestScore) {
			second = best
			secondScore = bestScore
			best = k
			bestScore = v
			continue
		}
		if k != best && better(k, v, second, secondScore) {
			second = k
			secondScore = v
		}
	}
	return best, bestScore, second, secondScore
}

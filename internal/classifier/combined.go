package classifier

func CombinedClassify(feat Features, mf MathFeatures, bw float64, centerHz float64, snrDb float64) Classification {
	ruleCls := RuleClassify(feat, centerHz, snrDb)
	mathCls := MathClassify(mf, bw, centerHz, snrDb)
	combined := map[SignalClass]float64{}
	for k, v := range ruleCls.Scores {
		combined[k] += v * 0.4
	}
	for k, v := range mathCls.Scores {
		combined[k] += v * 0.6
	}
	best, _, second, _ := top2(combined)
	if best == "" {
		best = ClassUnknown
	}
	if second == "" {
		second = ClassUnknown
	}
	conf := softmaxConfidence(combined, best)
	if snrDb < 20 {
		snrFactor := clamp01((snrDb - 3) / 17.0)
		conf *= 0.3 + 0.7*snrFactor
	}
	if conf <= 0 {
		conf = 0.1
	}
	return Classification{
		ModType:      best,
		Confidence:   conf,
		BW3dB:        feat.BW3dB,
		Features:     feat,
		MathFeatures: &mf,
		SecondBest:   second,
		Scores:       combined,
	}
}

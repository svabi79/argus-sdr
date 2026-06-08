package classifier

import "math"

func CombinedClassify(feat Features, mf MathFeatures, bw float64, centerHz float64, snrDb float64) Classification {
	ruleCls := RuleClassify(feat, bw, centerHz, snrDb)
	mathCls := MathClassify(mf, bw, centerHz, snrDb)

	// Carrier detection is the robust part on live HF: a sharp spectral carrier spike
	// (rule, offset/noise-tolerant) or a recovered DC carrier (CarrierDCCentered).
	// When a carrier is present, AM/CW-by-OCCUPIED-width is authoritative over the
	// noisy IQ blend, which otherwise mislabels carriers (wide AM -> CW/FSK; #82). A
	// real CW occupies <500 Hz, so only a clearly wide channel (broadcast) is AM.
	sharpCarrier := feat.BW3dB > 0 && feat.BW3dB <= 200 && feat.PeakToAvg >= 6
	carrierCue := math.Max(mf.CarrierDC, mf.CarrierDCCentered)
	if sharpCarrier || carrierCue > 0.4 {
		best := ClassCW
		second := ClassAM
		if bw >= 3000 {
			best, second = ClassAM, ClassCW
		}
		conf := 0.6
		if snrDb < 20 {
			conf *= 0.4 + 0.6*clamp01((snrDb-3)/17.0)
		}
		return Classification{
			ModType:      best,
			Confidence:   conf,
			BW3dB:        feat.BW3dB,
			Features:     feat,
			MathFeatures: &mf,
			SecondBest:   second,
			Scores:       map[SignalClass]float64{best: 6.0},
		}
	}

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

package classifier

import "math"

func RuleClassify(feat Features) Classification {
	bw := feat.BW3dB
	flat := feat.SpectralFlat
	sym := feat.Symmetry
	p2a := feat.PeakToAvg

	best := ClassUnknown
	second := ClassUnknown
	conf := 0.3

	switch {
	case bw > 100e3:
		best = ClassWFM
		conf = 0.9
	case bw >= 6e3 && bw <= 16e3:
		best = ClassNFM
		conf = 0.8
		if flat > 0.7 {
			second = ClassNoise
		}
		// digital voice rough guesses
		if feat.InstFreqStd < 0.5 && feat.EnvVariance < 0.3 {
			second = ClassDMR
		}
	case bw >= 3e3 && bw < 6e3:
		// wider SSB/AM
		if sym > 0.2 {
			best = ClassSSBUSB
			conf = 0.65
		} else if sym < -0.2 {
			best = ClassSSBLSB
			conf = 0.65
		} else if p2a > 2.5 && flat < 0.5 {
			best = ClassAM
			conf = 0.6
		}
	case bw >= 500 && bw < 3e3:
		// narrow SSB/AM + digital
		if feat.EnvVariance < 0.6 && feat.InstFreqStd < 0.7 && bw >= 2000 && bw < 3000 {
			best = ClassFT8
			conf = 0.55
		} else if sym > 0.2 {
			best = ClassSSBUSB
			conf = 0.7
		} else if sym < -0.2 {
			best = ClassSSBLSB
			conf = 0.7
		} else if p2a > 3 && flat < 0.4 {
			best = ClassAM
			conf = 0.6
		} else if feat.InstFreqStd > 0.8 {
			best = ClassFSK
			conf = 0.5
		} else if feat.InstFreqStd < 0.3 {
			best = ClassPSK
			conf = 0.5
		}
	case bw >= 150 && bw < 500:
		if feat.EnvVariance < 0.4 && feat.InstFreqStd < 0.5 {
			best = ClassWSPR
			conf = 0.55
		}
	case bw < 150:
		best = ClassCW
		conf = 0.7
	}

	// noise hint
	if best == ClassUnknown && flat > 0.85 && bw > 2e3 {
		best = ClassNoise
		conf = 0.6
	}

	// edge-case: if symmetry is strong, second best opposite side
	if (best == ClassSSBUSB || best == ClassSSBLSB) && second == ClassUnknown {
		if best == ClassSSBUSB {
			second = ClassSSBLSB
		} else {
			second = ClassSSBUSB
		}
	}

	// slightly scale confidence by feature strength
	if best == ClassNFM || best == ClassWFM {
		conf = conf * (0.8 + 0.2*clamp01(1-flat))
	}
	if best == ClassAM {
		conf = conf * (0.7 + 0.3*clamp01(p2a/6.0))
	}
	if math.IsNaN(conf) || conf <= 0 {
		conf = 0.3
	}

	return Classification{
		ModType:    best,
		Confidence: conf,
		BW3dB:      bw,
		Features:   feat,
		SecondBest: second,
	}
}

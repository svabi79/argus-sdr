package classifier

type ClassifierMode string

const (
	ModeRule     ClassifierMode = "rule"
	ModeMath     ClassifierMode = "math"
	ModeCombined ClassifierMode = "combined"
)

func Classify(input SignalInput, spectrum []float64, sampleRate int, fftSize int, iq []complex64, mode ClassifierMode) *Classification {
	if len(spectrum) == 0 || input.FirstBin < 0 || input.LastBin < 0 {
		return nil
	}
	feat := ExtractFeatures(input, spectrum, sampleRate, fftSize)
	// Use the wider of spectral BW3dB and detector's occupied BWHz for hard rules.
	// BW3dB measures only the 3dB peak width which can be much narrower than the
	// actual occupied bandwidth (e.g. FM broadcast has a peaked spectrum).
	hardBW := feat.BW3dB
	if input.BWHz > hardBW {
		hardBW = input.BWHz
	}
	if hard := TryHardRule(input.CenterHz, hardBW); hard != nil {
		hard.Features = feat
		hard.BW3dB = hardBW
		return hard
	}
	if len(iq) > 0 {
		envVar, zc, instStd, crest := ExtractTemporalFeatures(iq)
		feat.EnvVariance = envVar
		feat.ZeroCross = zc
		feat.InstFreqStd = instStd
		feat.CrestFactor = crest
	}
	var cls Classification
	switch mode {
	case ModeMath:
		if len(iq) > 0 {
			mf := ExtractMathFeatures(iq)
			cls = MathClassify(mf, feat.BW3dB, input.CenterHz, input.SNRDb)
			cls.Features = feat
		} else {
			cls = RuleClassify(feat, input.CenterHz, input.SNRDb)
		}
	case ModeCombined:
		if len(iq) > 0 {
			mf := ExtractMathFeatures(iq)
			cls = CombinedClassify(feat, mf, input.CenterHz, input.SNRDb)
		} else {
			cls = RuleClassify(feat, input.CenterHz, input.SNRDb)
		}
	default:
		cls = RuleClassify(feat, input.CenterHz, input.SNRDb)
	}
	return &cls
}

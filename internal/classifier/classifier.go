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
	if hard := TryHardRule(input.CenterHz, feat.BW3dB); hard != nil {
		hard.Features = feat
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

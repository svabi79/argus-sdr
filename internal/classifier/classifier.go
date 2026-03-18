package classifier

// Classify builds features and applies the rule-based classifier.
func Classify(input SignalInput, spectrum []float64, sampleRate int, fftSize int, iq []complex64) *Classification {
	if len(spectrum) == 0 || input.FirstBin < 0 || input.LastBin < 0 {
		return nil
	}
	feat := ExtractFeatures(input, spectrum, sampleRate, fftSize)
	if len(iq) > 0 {
		envVar, zc, instStd, crest := ExtractTemporalFeatures(iq)
		feat.EnvVariance = envVar
		feat.ZeroCross = zc
		feat.InstFreqStd = instStd
		feat.CrestFactor = crest
	}
	cls := RuleClassify(feat)
	return &cls
}

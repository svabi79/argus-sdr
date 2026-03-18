package classifier

// Classify builds features and applies the rule-based classifier.
func Classify(input SignalInput, spectrum []float64, sampleRate int, fftSize int) *Classification {
	if len(spectrum) == 0 || input.FirstBin < 0 || input.LastBin < 0 {
		return nil
	}
	feat := ExtractFeatures(input, spectrum, sampleRate, fftSize)
	cls := RuleClassify(feat)
	return &cls
}

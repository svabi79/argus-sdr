package pipeline

import (
	"sdr-wideband-suite/internal/classifier"
	"sdr-wideband-suite/internal/detector"
)

// RefineCandidates upgrades coarse detector candidates into refined signals
// by attaching local IQ-derived classification and PLL metadata.
func RefineCandidates(candidates []Candidate, spectrum []float64, sampleRate int, fftSize int, snippets [][]complex64, snippetRates []int, mode classifier.ClassifierMode) []Refinement {
	out := make([]Refinement, 0, len(candidates))
	for i, c := range candidates {
		sig := detector.Signal{
			ID:       c.ID,
			FirstBin: c.FirstBin,
			LastBin:  c.LastBin,
			CenterHz: c.CenterHz,
			BWHz:     c.BandwidthHz,
			PeakDb:   c.PeakDb,
			SNRDb:    c.SNRDb,
			NoiseDb:  c.NoiseDb,
		}
		var snip []complex64
		if i < len(snippets) {
			snip = snippets[i]
		}
		snipRate := sampleRate
		if i < len(snippetRates) && snippetRates[i] > 0 {
			snipRate = snippetRates[i]
		}
		cls := classifier.Classify(classifier.SignalInput{
			FirstBin: sig.FirstBin,
			LastBin:  sig.LastBin,
			SNRDb:    sig.SNRDb,
			CenterHz: sig.CenterHz,
			BWHz:     sig.BWHz,
		}, spectrum, sampleRate, fftSize, snip, mode)
		sig.Class = cls
		if cls != nil && snip != nil && len(snip) > 256 {
			pll := classifier.EstimateExactFrequency(snip, snipRate, sig.CenterHz, cls.ModType)
			cls.PLL = &pll
			sig.PLL = &pll
			if cls.ModType == classifier.ClassWFM && pll.Stereo {
				cls.ModType = classifier.ClassWFMStereo
			}
		}
		out = append(out, Refinement{
			Candidate:   c,
			Signal:      sig,
			SnippetRate: snipRate,
			Class:       cls,
			Stage:       "local-iq",
		})
	}
	return out
}

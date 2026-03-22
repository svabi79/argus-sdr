package pipeline

import (
	"sdr-wideband-suite/internal/classifier"
	"sdr-wideband-suite/internal/detector"
)

// RefineCandidates upgrades coarse detector candidates into refined signals
// by attaching local IQ-derived classification and PLL metadata.
func RefineCandidates(candidates []Candidate, windows []RefinementWindow, spectrum []float64, sampleRate int, fftSize int, snippets [][]complex64, snippetRates []int, mode classifier.ClassifierMode) []Refinement {
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
		if cls != nil && cls.ModType == classifier.ClassWFM {
			cls.ModType = classifier.ClassWFMStereo
			sig.PlaybackMode = string(classifier.ClassWFMStereo)
			sig.DemodName = string(classifier.ClassWFMStereo)
			if sig.PLL != nil && sig.PLL.Stereo {
				sig.StereoState = "locked"
			} else {
				sig.StereoState = "searching"
			}
		}
		if cls != nil && snip != nil && len(snip) > 256 {
			pll := classifier.EstimateExactFrequency(snip, snipRate, sig.CenterHz, cls.ModType)
			cls.PLL = &pll
			sig.PLL = &pll
			if cls.ModType == classifier.ClassWFMStereo {
				if pll.Stereo {
					sig.StereoState = "locked"
				} else if sig.StereoState == "" {
					sig.StereoState = "searching"
				}
				sig.PlaybackMode = string(classifier.ClassWFMStereo)
				sig.DemodName = string(classifier.ClassWFMStereo)
			}
		}
		var window RefinementWindow
		if i < len(windows) {
			window = windows[i]
		}
		if window.CenterHz == 0 {
			window.CenterHz = c.CenterHz
		}
		if window.SpanHz <= 0 {
			if c.BandwidthHz > 0 {
				window.SpanHz = c.BandwidthHz
			} else {
				window.SpanHz = 12000
			}
		}
		if window.Source == "" {
			window.Source = "candidate"
		}
		out = append(out, Refinement{
			Candidate:   c,
			Window:      window,
			Signal:      sig,
			SnippetRate: snipRate,
			Class:       cls,
			Stage:       "local-iq",
		})
	}
	return out
}

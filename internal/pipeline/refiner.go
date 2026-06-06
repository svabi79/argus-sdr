package pipeline

import (
	"math"

	"sdr-wideband-suite/internal/classifier"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/estimate"
)

// RefineCandidates upgrades coarse detector candidates into refined signals by
// re-estimating occupied bandwidth/center from the local spectrum (R1) and
// attaching local IQ-derived classification and PLL metadata.
//
// occupiedBwFraction selects the power-containment fraction for bandwidth
// re-estimation (e.g. 0.99); <=0 disables it and keeps the coarse detector
// bandwidth.
func RefineCandidates(candidates []Candidate, windows []RefinementWindow, spectrum []float64, sampleRate int, fftSize int, snippets [][]complex64, snippetRates []int, mode classifier.ClassifierMode, occupiedBwFraction float64) []Refinement {
	out := make([]Refinement, 0, len(candidates))
	binWidth := 0.0
	if fftSize > 0 {
		binWidth = float64(sampleRate) / float64(fftSize)
	}
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
		// R1: re-estimate occupied bandwidth + center from the local spectrum.
		// Guarded: only applied when the estimate is valid and within a sane
		// factor of the coarse bandwidth, otherwise the coarse value is kept.
		if occupiedBwFraction > 0 && binWidth > 0 && len(spectrum) > 0 &&
			c.FirstBin >= 0 && c.LastBin < len(spectrum) && c.LastBin >= c.FirstBin {
			ref := estimate.RefineFromSpectrum(spectrum, c.FirstBin, c.LastBin, binWidth, occupiedBwFraction)
			if ref.OK && ref.BandwidthHz > 0 && c.BandwidthHz > 0 &&
				ref.BandwidthHz >= 0.2*c.BandwidthHz && ref.BandwidthHz <= 6*c.BandwidthHz {
				coarseCenterBin := float64(c.FirstBin+c.LastBin) / 2
				sig.BWHz = ref.BandwidthHz
				sig.CenterHz = c.CenterHz + (ref.CenterBin-coarseCenterBin)*binWidth
				if lo := int(math.Round(ref.LowBin)); lo >= 0 && lo < len(spectrum) {
					sig.FirstBin = lo
				}
				if hi := int(math.Round(ref.HighBin)); hi >= sig.FirstBin && hi < len(spectrum) {
					sig.LastBin = hi
				}
			}
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

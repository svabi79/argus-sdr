package pipeline

import (
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
// survSpectrum is the surveillance spectrum the detector ran on; the candidate
// FirstBin/LastBin are in its coordinates, so it (not the lower-resolution detail
// spectrum) is used for occupied-bandwidth re-estimation.
func RefineCandidates(candidates []Candidate, windows []RefinementWindow, spectrum []float64, sampleRate int, fftSize int, snippets [][]complex64, snippetRates []int, mode classifier.ClassifierMode, survSpectrum []float64, occupiedBwFraction float64) []Refinement {
	out := make([]Refinement, 0, len(candidates))
	estBinWidth := 0.0
	if len(survSpectrum) > 0 {
		estBinWidth = float64(sampleRate) / float64(len(survSpectrum))
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
		// R1: re-estimate occupied bandwidth + center + SNR from the surveillance
		// spectrum (candidate bins are in its coordinates). Guarded: only applied
		// when the estimate is valid and within a sane factor of the coarse
		// bandwidth, otherwise the coarse value is kept. FirstBin/LastBin are left
		// untouched (they feed the classifier's detail-spectrum feature path).
		if occupiedBwFraction > 0 && estBinWidth > 0 && len(survSpectrum) > 0 &&
			c.FirstBin >= 0 && c.LastBin < len(survSpectrum) && c.LastBin >= c.FirstBin {
			ref := estimate.RefineFromSpectrum(survSpectrum, c.FirstBin, c.LastBin, estBinWidth, occupiedBwFraction)
			if ref.OK && ref.BandwidthHz > 0 && c.BandwidthHz > 0 &&
				ref.BandwidthHz >= 0.4*c.BandwidthHz && ref.BandwidthHz <= 2.5*c.BandwidthHz {
				// Refine bandwidth + SNR only. CenterHz is intentionally NOT
				// overridden: the detector's power-weighted centroid is carrier-
				// accurate, while the occupancy centroid can drift a few kHz on an
				// asymmetric instant — enough to detune WFM stereo's 19 kHz pilot
				// PLL and prevent stereo lock.
				sig.BWHz = ref.BandwidthHz
				if ref.SNRDb > 0 {
					sig.SNRDb = ref.SNRDb
					sig.NoiseDb = ref.NoiseFloorDb
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

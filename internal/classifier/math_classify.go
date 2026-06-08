package classifier

import "math"

type MathFeatures struct {
	EnvCoV        float64 `json:"env_cov"`
	EnvKurtosis   float64 `json:"env_kurtosis"`
	InstFreqStd   float64 `json:"inst_freq_std"`
	InstFreqRange float64 `json:"inst_freq_range"`
	AMIndex       float64 `json:"am_index"`
	FMIndex       float64 `json:"fm_index"`
	InstFreqModes int     `json:"inst_freq_modes"`
	// CarrierDC is the fraction of power in the DC component (|mean(iq)|^2 /
	// mean|iq|^2) of the centered baseband. A signal with a real carrier at its
	// center (AM, CW) has a strong DC term; a suppressed-carrier/offset signal
	// (SSB, FM, digital) has ~0. This is the AM/CW-vs-rest discriminator.
	CarrierDC float64 `json:"carrier_dc"`
	// IFMean is the signed mean instantaneous frequency (rad/sample): ~0 for a
	// centered carrier (AM/CW) and symmetric AM/FM, but net-positive for USB and
	// net-negative for LSB (energy sits to one side of the suppressed carrier).
	IFMean float64 `json:"if_mean"`
	// CarrierDCCentered is CarrierDC re-measured after de-rotating the IQ by IFMean,
	// which brings a real carrier (AM/CW) to DC even when the extraction left a
	// residual frequency offset. Plain CarrierDC needs sub-5-Hz centering and so
	// collapses on live HF (TestCarrierDCOffset); this recovers it. Suppressed-carrier
	// signals (SSB/FM/digital) have no concentrated carrier for de-rotation to expose,
	// so it stays ~0 — making this the live-robust AM/CW-vs-rest discriminator.
	CarrierDCCentered float64 `json:"carrier_dc_centered"`
}

func ExtractMathFeatures(iq []complex64) MathFeatures {
	if len(iq) < 10 {
		return MathFeatures{}
	}
	n := len(iq)
	env := make([]float64, n)
	var envMean float64
	var dcRe, dcIm, powMean float64
	for i, v := range iq {
		re, im := float64(real(v)), float64(imag(v))
		a := math.Hypot(re, im)
		env[i] = a
		envMean += a
		dcRe += re
		dcIm += im
		powMean += re*re + im*im
	}
	envMean /= float64(n)
	dcRe /= float64(n)
	dcIm /= float64(n)
	powMean /= float64(n)
	carrierDC := 0.0
	if powMean > 1e-20 {
		carrierDC = (dcRe*dcRe + dcIm*dcIm) / powMean
	}
	var envVar, envM4 float64
	for _, a := range env {
		d := a - envMean
		envVar += d * d
		envM4 += d * d * d * d
	}
	envVar /= float64(n)
	envM4 /= float64(n)
	envStd := math.Sqrt(envVar)
	envCoV := 0.0
	if envMean > 1e-12 {
		envCoV = envStd / envMean
	}
	envKurtosis := 0.0
	if envVar > 1e-20 {
		envKurtosis = envM4 / (envVar * envVar)
	}
	instFreq := make([]float64, n-1)
	var ifMean float64
	ifMin := math.Inf(1)
	ifMax := math.Inf(-1)
	for i := 1; i < n; i++ {
		p := iq[i-1]
		c := iq[i]
		num := float64(real(p))*float64(imag(c)) - float64(imag(p))*float64(real(c))
		den := float64(real(p))*float64(real(c)) + float64(imag(p))*float64(imag(c))
		f := math.Atan2(num, den)
		instFreq[i-1] = f
		ifMean += f
		if f < ifMin {
			ifMin = f
		}
		if f > ifMax {
			ifMax = f
		}
	}
	ifMean /= float64(n - 1)
	var ifVar float64
	for _, f := range instFreq {
		d := f - ifMean
		ifVar += d * d
	}
	ifVar /= float64(n - 1)
	ifStd := math.Sqrt(ifVar)
	ifRange := ifMax - ifMin
	modes := countHistogramPeaks(instFreq, 32)
	amIndex := envCoV / math.Max(ifStd, 0.001)
	fmIndex := ifStd / math.Max(envCoV, 0.001)
	// De-rotate by IFMean and re-measure the DC power fraction. powMean is invariant
	// under rotation, so reuse it. (See the CarrierDCCentered field doc.)
	var cdcRe, cdcIm float64
	for i, v := range iq {
		th := ifMean * float64(i)
		c, s := math.Cos(th), math.Sin(th)
		re, im := float64(real(v)), float64(imag(v))
		cdcRe += re*c + im*s
		cdcIm += im*c - re*s
	}
	cdcRe /= float64(n)
	cdcIm /= float64(n)
	carrierDCCentered := 0.0
	if powMean > 1e-20 {
		carrierDCCentered = (cdcRe*cdcRe + cdcIm*cdcIm) / powMean
	}
	return MathFeatures{
		EnvCoV:            envCoV,
		EnvKurtosis:       envKurtosis,
		InstFreqStd:       ifStd,
		InstFreqRange:     ifRange,
		AMIndex:           amIndex,
		FMIndex:           fmIndex,
		InstFreqModes:     modes,
		CarrierDC:         carrierDC,
		IFMean:            ifMean,
		CarrierDCCentered: carrierDCCentered,
	}
}

func countHistogramPeaks(vals []float64, bins int) int {
	if len(vals) == 0 || bins < 3 {
		return 0
	}
	minV, maxV := vals[0], vals[0]
	for _, v := range vals {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	span := maxV - minV
	if span < 1e-10 {
		return 1
	}
	hist := make([]int, bins)
	for _, v := range vals {
		idx := int(float64(bins-1) * (v - minV) / span)
		if idx >= bins {
			idx = bins - 1
		}
		if idx < 0 {
			idx = 0
		}
		hist[idx]++
	}
	smooth := make([]int, bins)
	maxSmooth := 0
	for i := range hist {
		s := hist[i]
		if i > 0 {
			s += hist[i-1]
		}
		if i < bins-1 {
			s += hist[i+1]
		}
		smooth[i] = s
		if s > maxSmooth {
			maxSmooth = s
		}
	}
	peaks := 0
	for i := 1; i < bins-1; i++ {
		if smooth[i] > smooth[i-1] && smooth[i] > smooth[i+1] {
			if float64(smooth[i]) > 0.1*float64(maxSmooth) {
				peaks++
			}
		}
	}
	if peaks == 0 {
		peaks = 1
	}
	return peaks
}

// MathClassify decides modulation from IQ-derived features. bw is the OCCUPIED
// bandwidth (detector edge-expanded), NOT the 3 dB peak width — a sharp AM/CW
// carrier has a tiny 3 dB width but a wide occupancy, and using the peak width
// here was the bug that labelled wide AM as CW.
//
// The backbone is built from features that are INVARIANT to center-estimate
// error, because the live extraction only approximately centers each signal. A
// residual frequency offset multiplies the IQ by e^{jwt}: that leaves the
// envelope magnitude unchanged (EnvCoV invariant) and only adds a constant to
// the instantaneous frequency (InstFreqStd invariant). CarrierDC and IFMean are
// NOT invariant — TestCarrierDCOffset shows CarrierDC collapsing from ~0.98 to
// ~0.04 at a mere 5 Hz offset — so they only refine, never gate.
//
// Decision backbone (each cue is a physical signature, not a frequency prior):
//   - constant envelope (EnvCoV very low) -> angle modulation: WFM/NFM by width,
//     or a clean keyed carrier (CW) if the frequency is also constant.
//   - moderate envelope modulation -> a carrier with AM: AM (occupied) vs CW
//     (narrow). CarrierDC, when centering is good, confirms the real carrier.
//   - high envelope variation, no carrier -> SSB (voice-width, one-sided, low
//     inst-freq spread; sideband from the sign of IFMean) or generic digital.
func MathClassify(mf MathFeatures, bw float64, centerHz float64, snrDb float64) Classification {
	scores := map[SignalClass]float64{}
	// Discriminators measured on labelled synth IQ at 15/30/45 dB
	// (TestClassifierFeatureDump). Two offset-invariant features form the backbone:
	//   EnvCoV (offset-invariant): suppressed-carrier SSB/digital ~0.45+; AM/CW and
	//     FM are lower, but in-band noise inflates them at low SNR (NFM 0.04@45dB ->
	//     0.12@15dB), so EnvCoV only gates the high (no-carrier) end reliably.
	//   InstFreqStd (offset-invariant): FM has genuine frequency variation
	//     (NFM ~0.034, WFM ~0.25, stable vs SNR) while a plain carrier (AM ~0.011)
	//     does not -> this is what splits FM from AM/CW.
	// Digital signals have BOTH high EnvCoV and high InstFreqStd, so the no-carrier
	// gate must be tested before the FM gate. The carrier cue is max(CarrierDC,
	// CarrierDCCentered): plain CarrierDC is offset-SENSITIVE (collapses 0.98->0.04 at
	// a 5 Hz offset), but CarrierDCCentered de-rotates by IFMean and recovers an
	// off-center carrier, so the pair gates reliably even on live HF where the
	// extraction leaves a residual offset. 0.4 separates AM/CW (synth 0.52-1.0) from
	// carrier-less SSB/FSK/PSK/digital (<=0.11, TestClassifierFeatureDump); see #82.
	// (IFMean stays offset-sensitive and is used only to refine SSB sideband, below.)
	carrierCue := math.Max(mf.CarrierDC, mf.CarrierDCCentered)
	carrier := carrierCue > 0.4      // a real carrier (AM/CW), centering-robust
	highEnv := mf.EnvCoV >= 0.20     // no carrier: large amplitude variation
	fmLike := mf.InstFreqStd >= 0.02 // genuine frequency modulation

	switch {
	case carrier:
		// A strong DC carrier component is DEFINITIVE: only AM and CW put real power
		// at the (centered) carrier; SSB/FM/digital are all suppressed-carrier
		// (CarrierDC ~0). So this is decisive and scored high enough to win the
		// rule+math blend (CombinedClassify), which otherwise mislabels a wide AM
		// carrier as CW because its 3 dB spectral peak is narrow. AM (occupied) vs
		// CW (narrow) by bandwidth. The split is 1 kHz: CW (Morse) occupies at most
		// a few hundred Hz even with fast keying, so a carrier wider than ~1 kHz is
		// AM, not CW. (The live detector under-reports AM occupancy — it locks onto
		// the carrier plus close-in sidebands, ~1.4-6 kHz, not the full channel — so
		// a higher threshold mislabelled those wide carriers as CW.) Verified live on
		// the 40m/49m broadcast bands; TestCarrierDCOffset covers the centering this
		// relies on for AM, whose carrier the extraction locks onto.
		if bw < 1000 {
			scores[ClassCW] += 6.0
		} else {
			scores[ClassAM] += 6.0
		}
	case highEnv:
		// Suppressed carrier with large envelope variation -> SSB or digital. A
		// narrow signal here is a weak carrier in in-band noise (CW at low SNR), not
		// wideband digital.
		oneSided := math.Abs(mf.IFMean) > 0.015
		switch {
		case bw < 1200:
			scores[ClassCW] += 1.5
			scores[ClassNoise] += 0.3
		case oneSided && bw <= 5000 && mf.InstFreqStd < 0.06:
			// Voice-width, energy on one side of center, low inst-freq spread -> SSB.
			if mf.IFMean >= 0 {
				scores[ClassSSBUSB] += 2.5
			} else {
				scores[ClassSSBLSB] += 2.5
			}
		default:
			// Generic digital. (The synth FSK/PSK/DIGITAL share one shape; per-kind
			// digital separation is future work.)
			scores[ClassFSK] += 2.0
			scores[ClassPSK] += 1.8
			if bw >= 10000 && bw <= 14000 {
				scores[ClassDMR] += 1.5
			}
			if bw >= 2000 && bw < 3500 && mf.InstFreqModes >= 3 {
				scores[ClassFT8] += 1.0
			}
		}
	case fmLike:
		// Constant-ish envelope with real frequency variation -> FM. WFM by occupied
		// width or large deviation, else NFM.
		if bw >= 80e3 || mf.InstFreqStd > 0.15 {
			scores[ClassWFM] += 3.0
		} else if bw >= 6000 {
			scores[ClassNFM] += 3.0
		} else {
			scores[ClassNFM] += 2.0
			scores[ClassCW] += 0.5 // very narrow: could be a keyed carrier
		}
	default:
		// Low envelope variation, no frequency modulation, weak/offset carrier -> a
		// plain carrier whose DC term washed out (centering error). AM (occupied) vs
		// CW (narrow).
		if bw < 1800 {
			scores[ClassCW] += 3.0
		} else {
			scores[ClassAM] += 3.0
			if bw < 2500 {
				scores[ClassCW] += 1.0 // ambiguous narrow carrier
			}
		}
	}
	best, _, second, _ := top2(scores)
	if best == "" {
		best = ClassUnknown
	}
	if second == "" {
		second = ClassUnknown
	}
	conf := softmaxConfidence(scores, best)
	if snrDb < 20 {
		snrFactor := clamp01((snrDb - 3) / 17.0)
		conf *= 0.3 + 0.7*snrFactor
	}
	if math.IsNaN(conf) || conf <= 0 {
		conf = 0.1
	}
	return Classification{
		ModType:      best,
		Confidence:   conf,
		BW3dB:        bw,
		SecondBest:   second,
		Scores:       scores,
		MathFeatures: &mf,
	}
}

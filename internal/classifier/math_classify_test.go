package classifier

import (
	"math"
	"testing"
)

func makeToneIQ(n int, freqNorm float64, am float64) []complex64 {
	iq := make([]complex64, n)
	for i := range iq {
		phase := 2 * math.Pi * freqNorm * float64(i)
		env := 1.0 + am*math.Sin(2*math.Pi*0.01*float64(i))
		iq[i] = complex(float32(env*math.Cos(phase)), float32(env*math.Sin(phase)))
	}
	return iq
}

func TestMathClassifyAM(t *testing.T) {
	// Centered carrier (freqNorm 0): the live extraction mixes each signal to
	// baseband, so an AM carrier sits at DC. An off-center tone washes out the DC
	// term and no longer models the live path.
	iq := makeToneIQ(4096, 0.0, 0.8)
	mf := ExtractMathFeatures(iq)
	if mf.AMIndex < 1.5 {
		t.Errorf("AM signal should have high AMIndex: got %.2f", mf.AMIndex)
	}
	cls := MathClassify(mf, 8000, 121.5e6, 25)
	if cls.ModType != ClassAM {
		t.Errorf("expected AM, got %s (scores: %v)", cls.ModType, cls.Scores)
	}
}

func TestMathClassifyFM(t *testing.T) {
	n := 4096
	iq := make([]complex64, n)
	phase := 0.0
	// Small frequency deviation -> narrowband FM. (A large deviation, ~30% of the
	// sample rate as before, is physically WIDEBAND FM and now classifies as WFM.)
	for i := range iq {
		freqDev := 0.01 * math.Sin(2*math.Pi*0.005*float64(i))
		phase += 2 * math.Pi * (0.1 + freqDev)
		iq[i] = complex(float32(math.Cos(phase)), float32(math.Sin(phase)))
	}
	mf := ExtractMathFeatures(iq)
	if mf.FMIndex < 2.0 {
		t.Errorf("FM signal should have high FMIndex: got %.2f", mf.FMIndex)
	}
	if mf.EnvCoV > 0.1 {
		t.Errorf("FM signal should have low EnvCoV: got %.3f", mf.EnvCoV)
	}
	cls := MathClassify(mf, 12000, 145.5e6, 25)
	if cls.ModType != ClassNFM {
		t.Errorf("expected NFM, got %s (scores: %v)", cls.ModType, cls.Scores)
	}
}

func TestMathClassifyCW(t *testing.T) {
	n := 4096
	iq := make([]complex64, n)
	// Centered carrier (freqNorm 0): the live extraction centers each signal on its
	// detected center, so a CW carrier sits at DC with constant envelope and
	// frequency.
	for i := range iq {
		phase := 2 * math.Pi * 0.0 * float64(i)
		iq[i] = complex(float32(math.Cos(phase)), float32(math.Sin(phase)))
	}
	mf := ExtractMathFeatures(iq)
	cls := MathClassify(mf, 100, 7.02e6, 20)
	if cls.ModType != ClassCW {
		t.Errorf("expected CW, got %s (scores: %v, kurtosis: %.1f)", cls.ModType, cls.Scores, mf.EnvKurtosis)
	}
}

func TestCombinedClassify(t *testing.T) {
	n := 4096
	iq := make([]complex64, n)
	phase := 0.0
	// Small deviation -> narrowband FM (see TestMathClassifyFM).
	for i := range iq {
		freqDev := 0.01 * math.Sin(2*math.Pi*0.003*float64(i))
		phase += 2 * math.Pi * (0.1 + freqDev)
		iq[i] = complex(float32(math.Cos(phase)), float32(math.Sin(phase)))
	}
	feat := Features{BW3dB: 12000, SpectralFlat: 0.3, PeakToAvg: 1.5, EnvVariance: 0.01, InstFreqStd: 0.8}
	mf := ExtractMathFeatures(iq)
	cls := CombinedClassify(feat, mf, 12000, 145.5e6, 25)
	if cls.ModType != ClassNFM {
		t.Errorf("expected NFM, got %s (scores: %v)", cls.ModType, cls.Scores)
	}
}

// makeChirpIQ is a constant-envelope frequency sweep: it has spread (not
// concentrated) energy and no carrier, so de-rotating by its mean instantaneous
// frequency must NOT manufacture a DC term.
func makeChirpIQ(n int) []complex64 {
	iq := make([]complex64, n)
	phase := 0.0
	for i := range iq {
		f := -0.1 + 0.2*float64(i)/float64(n) // sweep across the band
		phase += 2 * math.Pi * f
		iq[i] = complex(float32(math.Cos(phase)), float32(math.Sin(phase)))
	}
	return iq
}

// TestCarrierDCCenteredRecoversOffset is the live-HF AM fix (#82): a real carrier
// offset from center collapses plain CarrierDC (it needs sub-bin centering) but
// CarrierDCCentered de-rotates by IFMean and recovers it — without manufacturing a
// carrier for genuinely carrier-less signals.
func TestCarrierDCCenteredRecoversOffset(t *testing.T) {
	centered := makeToneIQ(4096, 0.0, 0.8) // AM carrier at DC
	offset := makeToneIQ(4096, 0.02, 0.8)  // same AM, carrier offset 2% of fs
	mc := ExtractMathFeatures(centered)
	mo := ExtractMathFeatures(offset)
	if mc.CarrierDC < 0.5 {
		t.Fatalf("sanity: centered AM should have high CarrierDC, got %.3f", mc.CarrierDC)
	}
	if mo.CarrierDC > 0.2 {
		t.Errorf("offset AM: plain CarrierDC should collapse, got %.3f", mo.CarrierDC)
	}
	if mo.CarrierDCCentered < 0.5 {
		t.Errorf("offset AM: CarrierDCCentered should recover the carrier, got %.3f", mo.CarrierDCCentered)
	}
	if ms := ExtractMathFeatures(makeChirpIQ(4096)); ms.CarrierDCCentered > 0.2 {
		t.Errorf("chirp (no carrier): CarrierDCCentered should stay low, got %.3f", ms.CarrierDCCentered)
	}
}

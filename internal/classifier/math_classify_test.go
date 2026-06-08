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

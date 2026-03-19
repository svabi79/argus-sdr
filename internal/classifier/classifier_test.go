package classifier

import "testing"

func TestRuleClassifyWFM(t *testing.T) {
	sampleRate := 1_000_000
	fftSize := 1024
	spectrum := make([]float64, fftSize)
	for i := range spectrum {
		spectrum[i] = -100
	}
	start := 100
	end := 350 // ~244 bins -> ~238 kHz
	for i := start; i <= end; i++ {
		spectrum[i] = -10
	}
	cls := Classify(SignalInput{FirstBin: start, LastBin: end}, spectrum, sampleRate, fftSize, nil)
	if cls == nil || cls.ModType != ClassWFM {
		t.Fatalf("expected WFM, got %+v", cls)
	}
}

func TestSoftmaxConfidence(t *testing.T) {
	scores1 := map[SignalClass]float64{ClassNFM: 2.0, ClassAM: 0.3, ClassNoise: 0.1}
	c1 := softmaxConfidence(scores1, ClassNFM)
	if c1 < 0.7 {
		t.Fatalf("clear winner should have high confidence: %f", c1)
	}

	scores2 := map[SignalClass]float64{ClassSSBUSB: 1.0, ClassSSBLSB: 0.9, ClassAM: 0.8}
	c2 := softmaxConfidence(scores2, ClassSSBUSB)
	if c2 > 0.5 {
		t.Fatalf("ambiguous should have low confidence: %f", c2)
	}

	c3 := softmaxConfidence(map[SignalClass]float64{}, ClassNFM)
	if c3 != 0.1 {
		t.Fatalf("empty should return 0.1: %f", c3)
	}
}

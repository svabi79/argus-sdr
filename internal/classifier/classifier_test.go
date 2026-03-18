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
	cls := Classify(SignalInput{FirstBin: start, LastBin: end}, spectrum, sampleRate, fftSize)
	if cls == nil || cls.ModType != ClassWFM {
		t.Fatalf("expected WFM, got %+v", cls)
	}
}

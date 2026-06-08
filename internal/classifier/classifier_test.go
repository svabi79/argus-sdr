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
	end := 350
	for i := start; i <= end; i++ {
		spectrum[i] = -10
	}
	cls := Classify(SignalInput{FirstBin: start, LastBin: end, CenterHz: 100e6, SNRDb: 30}, spectrum, sampleRate, fftSize, nil, ModeCombined)
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

func TestClassifierProfiles(t *testing.T) {
	tests := []struct {
		name     string
		feat     Features
		centerHz float64
		snrDb    float64
		wantBest SignalClass
	}{
		{
			name: "FM Broadcast 100 MHz",
			feat: Features{BW3dB: 120000, SpectralFlat: 0.3, PeakToAvg: 1.5, Symmetry: 0.05,
				RolloffLeft: 20, RolloffRight: 22, EnvVariance: 0.01, InstFreqStd: 0.8},
			centerHz: 100.0e6, snrDb: 40,
			wantBest: ClassWFM,
		},
		{
			name: "FT8 auf 7.074 MHz",
			feat: Features{BW3dB: 2500, SpectralFlat: 0.6, PeakToAvg: 1.8, Symmetry: 0.1,
				EnvVariance: 0.03, InstFreqStd: 0.4},
			centerHz: 7.074e6, snrDb: 15,
			wantBest: ClassFT8,
		},
		{
			name: "USB Voice 14.230 MHz",
			feat: Features{BW3dB: 2800, SpectralFlat: 0.35, PeakToAvg: 3.5, Symmetry: 0.4,
				RolloffLeft: 5, RolloffRight: 18, EnvVariance: 0.25, InstFreqStd: 0.6},
			centerHz: 14.230e6, snrDb: 25,
			wantBest: ClassSSBUSB,
		},
		{
			name: "DMR auf 438 MHz",
			feat: Features{BW3dB: 12500, SpectralFlat: 0.7, PeakToAvg: 1.2, Symmetry: 0.02,
				RolloffLeft: 25, RolloffRight: 24, EnvVariance: 0.01, InstFreqStd: 0.35},
			centerHz: 438.5e6, snrDb: 20,
			wantBest: ClassDMR,
		},
		{
			name: "Airband AM 121.5 MHz",
			feat: Features{BW3dB: 7000, SpectralFlat: 0.25, PeakToAvg: 4.0, Symmetry: 0.05,
				RolloffLeft: 15, RolloffRight: 16, EnvVariance: 0.2, InstFreqStd: 0.7},
			centerHz: 121.5e6, snrDb: 30,
			wantBest: ClassAM,
		},
		{
			name: "CW auf 7.020 MHz",
			feat: Features{BW3dB: 80, SpectralFlat: 0.15, PeakToAvg: 8.0, Symmetry: 0.0,
				EnvVariance: 0.9, InstFreqStd: 0.05, CrestFactor: 3.5},
			centerHz: 7.020e6, snrDb: 20,
			wantBest: ClassCW,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cls := RuleClassify(tt.feat, tt.feat.BW3dB, tt.centerHz, tt.snrDb)
			if cls.ModType != tt.wantBest {
				t.Errorf("got %s (conf=%.2f), want %s. Scores: %v", cls.ModType, cls.Confidence, tt.wantBest, cls.Scores)
			}
		})
	}
}

func TestLowSNRConfidence(t *testing.T) {
	feat := Features{BW3dB: 3000, SpectralFlat: 0.5, PeakToAvg: 1.5}
	cls := RuleClassify(feat, feat.BW3dB, 14.2e6, 5)
	if cls.Confidence > 0.5 {
		t.Errorf("low SNR should have low confidence: got %.2f", cls.Confidence)
	}
}

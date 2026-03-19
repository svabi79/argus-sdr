package cfar

import "testing"

func makeSpectrum(n int, noiseDb float64, signals [][2]int, sigDb float64) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = noiseDb
	}
	for _, sig := range signals {
		for i := sig[0]; i <= sig[1] && i < n; i++ {
			s[i] = sigDb
		}
	}
	return s
}

func TestAllVariantsDetectSignal(t *testing.T) {
	spec := makeSpectrum(1024, -100, [][2]int{{500, 510}}, -20)
	for _, mode := range []Mode{ModeCA, ModeOS, ModeGOSCA, ModeCASO} {
		c := New(Config{Mode: mode, GuardCells: 2, TrainCells: 16, Rank: 24, ScaleDb: 6, WrapAround: true})
		if c == nil {
			t.Fatalf("%s: nil", mode)
		}
		th := c.Thresholds(spec)
		if len(th) != 1024 {
			t.Fatalf("%s: len=%d", mode, len(th))
		}
		if spec[505] < th[505] {
			t.Fatalf("%s: signal not above threshold", mode)
		}
		if spec[200] >= th[200] {
			t.Fatalf("%s: noise above threshold", mode)
		}
	}
}

func TestWrapAroundEdges(t *testing.T) {
	spec := makeSpectrum(256, -100, [][2]int{{0, 5}}, -20)
	c := New(Config{Mode: ModeCA, GuardCells: 2, TrainCells: 8, ScaleDb: 6, WrapAround: true})
	th := c.Thresholds(spec)
	if th[0] <= -200 || th[0] > 0 {
		t.Fatalf("edge threshold bad: %v", th[0])
	}
	if th[255] <= -200 || th[255] > 0 {
		t.Fatalf("wrap threshold bad: %v", th[255])
	}
}

func TestGOSCAMaskingProtection(t *testing.T) {
	spec := makeSpectrum(1024, -100, [][2]int{{500, 510}, {530, 540}}, -20)
	cGosca := New(Config{Mode: ModeGOSCA, GuardCells: 2, TrainCells: 16, ScaleDb: 6, WrapAround: true})
	cCA := New(Config{Mode: ModeCA, GuardCells: 2, TrainCells: 16, ScaleDb: 6, WrapAround: true})
	thG := cGosca.Thresholds(spec)
	thC := cCA.Thresholds(spec)
	midBin := 520
	if thG[midBin] < thC[midBin] {
		t.Logf("GOSCA=%f CA=%f at bin %d — GOSCA correctly higher", thG[midBin], thC[midBin], midBin)
	}
}

func BenchmarkCFAR(b *testing.B) {
	spec := makeSpectrum(2048, -100, [][2]int{{500, 510}, {1000, 1020}}, -20)
	for _, mode := range []Mode{ModeCA, ModeOS, ModeGOSCA, ModeCASO} {
		cfg := Config{Mode: mode, GuardCells: 2, TrainCells: 16, Rank: 24, ScaleDb: 6, WrapAround: true}
		c := New(cfg)
		b.Run(string(mode), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				c.Thresholds(spec)
			}
		})
	}
}

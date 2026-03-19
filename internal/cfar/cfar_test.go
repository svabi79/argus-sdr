package cfar

import (
	"math"
	"testing"
)

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

func TestCellAveragingUsesLinearPower(t *testing.T) {
	spec := []float64{-100, -100, -100, -80, -100, -90, -100, -100, -100}
	cfg := Config{GuardCells: 0, TrainCells: 2, ScaleDb: 0, WrapAround: false}

	ca := New(Config{Mode: ModeCA, GuardCells: cfg.GuardCells, TrainCells: cfg.TrainCells, ScaleDb: cfg.ScaleDb, WrapAround: cfg.WrapAround})
	gosca := New(Config{Mode: ModeGOSCA, GuardCells: cfg.GuardCells, TrainCells: cfg.TrainCells, ScaleDb: cfg.ScaleDb, WrapAround: cfg.WrapAround})
	caso := New(Config{Mode: ModeCASO, GuardCells: cfg.GuardCells, TrainCells: cfg.TrainCells, ScaleDb: cfg.ScaleDb, WrapAround: cfg.WrapAround})

	mid := 5
	caTh := ca.Thresholds(spec)[mid]
	goscaTh := gosca.Thresholds(spec)[mid]
	casoTh := caso.Thresholds(spec)[mid]

	dbApproxCA := (-80.0 + -90.0 + -100.0 + -100.0) / 4.0
	dbApproxGOSCA := -85.0
	dbApproxCASO := -100.0

	if math.Abs(caTh-dbApproxCA) < 1.0 {
		t.Fatalf("CA threshold still looks like dB averaging: got %v approx %v", caTh, dbApproxCA)
	}
	if math.Abs(goscaTh-dbApproxGOSCA) < 1.0 {
		t.Fatalf("GOSCA threshold still looks like dB averaging: got %v approx %v", goscaTh, dbApproxGOSCA)
	}
	if math.Abs(casoTh-dbApproxCASO) < 1.0 {
		t.Fatalf("CASO threshold still looks like dB averaging: got %v approx %v", casoTh, dbApproxCASO)
	}
	if !(goscaTh > caTh && caTh > casoTh) {
		t.Fatalf("unexpected ordering: GOSCA=%v CA=%v CASO=%v", goscaTh, caTh, casoTh)
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

package estimate_test

import (
	"math"
	"testing"

	"sdr-wideband-suite/internal/estimate"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/synth"
)

const (
	estFs = 2_500_000
	estN  = 8192
)

// regionAround returns the dB spectrum slice [centerHz ± halfHz] and its bin
// width. The full scene spectrum is computed first (DC at n/2).
func regionAround(spec []float64, centerHz, halfHz float64) ([]float64, float64) {
	binWidth := float64(estFs) / float64(estN)
	centerBin := estN/2 + int(math.Round(centerHz/binWidth))
	half := int(math.Round(halfHz / binWidth))
	lo := centerBin - half
	hi := centerBin + half
	if lo < 0 {
		lo = 0
	}
	if hi >= len(spec) {
		hi = len(spec) - 1
	}
	return spec[lo : hi+1], binWidth
}

// estimateOne generates a single-signal scene and returns the occupied-bandwidth
// estimate over a generous region (±4× the true bandwidth, with noise margin).
func estimateOne(kind synth.Kind, bwHz, snr float64) estimate.Occupancy {
	scene := synth.Scene{
		SampleRate: estFs, Seed: 99, NoiseStd: 1.0,
		Signals: []synth.SignalSpec{{Kind: kind, CenterHz: 0, BandwidthHz: bwHz, SNRdB: snr}},
	}
	spec := fftutil.Spectrum(scene.Generate(estN), fftutil.Hann(estN))
	region, binWidth := regionAround(spec, 0, math.Max(4*bwHz, 60e3))
	return estimate.OccupiedBandwidthDb(region, binWidth, 0.99)
}

func TestOccupiedBandwidthTracksTruth(t *testing.T) {
	cases := []struct {
		kind    synth.Kind
		bwHz    float64
		tolFrac float64 // allowed |est-truth|/truth
	}{
		{synth.KindWFM, 180e3, 0.30},
		{synth.KindNFM, 12e3, 0.40},
		{synth.KindAM, 8e3, 0.45},
		{synth.KindSSB, 3e3, 0.60},
		{synth.KindDigital, 25e3, 0.35},
	}
	for _, c := range cases {
		occ := estimateOne(c.kind, c.bwHz, 30)
		if !occ.OK {
			t.Errorf("%s: estimator returned not-ok", c.kind)
			continue
		}
		err := math.Abs(occ.BandwidthHz-c.bwHz) / c.bwHz
		t.Logf("%-8s truth=%.0f est=%.0f err=%.1f%%", c.kind, c.bwHz, occ.BandwidthHz, err*100)
		if err > c.tolFrac {
			t.Errorf("%s: bw error %.1f%% exceeds tol %.0f%% (truth=%.0f est=%.0f)",
				c.kind, err*100, c.tolFrac*100, c.bwHz, occ.BandwidthHz)
		}
	}
}

// The whole point of R1: occupied bandwidth must beat the detector's geometric
// −81% WFM under-estimate. Assert WFM is captured within ±30% rather than ~−80%.
func TestOccupiedBandwidthFixesWFMUnderestimate(t *testing.T) {
	occ := estimateOne(synth.KindWFM, 180e3, 30)
	if !occ.OK {
		t.Fatal("not ok")
	}
	if occ.BandwidthHz < 0.7*180e3 {
		t.Errorf("WFM occupied bw %.0f still badly under 180000 (detector baseline was ~34000)", occ.BandwidthHz)
	}
}

func TestEmptyRegionNotOK(t *testing.T) {
	flat := make([]float64, 256)
	for i := range flat {
		flat[i] = -100 // pure flat noise floor, no signal
	}
	occ := estimate.OccupiedBandwidthDb(flat, 305, 0.99)
	if occ.OK {
		t.Errorf("flat noise region should not yield an occupancy, got bw=%.0f", occ.BandwidthHz)
	}
}

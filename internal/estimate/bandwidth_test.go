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

var nominalBW = []struct {
	kind synth.Kind
	bw   float64
}{
	{synth.KindWFM, 180e3},
	{synth.KindDigital, 25e3},
	{synth.KindNFM, 12e3},
	{synth.KindAM, 8e3},
	{synth.KindSSB, 3e3},
}

// The occupied bandwidth is not the nominal (Carson) bandwidth — for FM the
// 99%-occupied band is genuinely narrower than Carson. So instead of pinning to
// the nominal value, assert the estimate is in a sane range of it and that the
// ordering across kinds matches their true relative widths.
func TestOccupiedBandwidthSaneAndOrdered(t *testing.T) {
	got := map[synth.Kind]float64{}
	for _, c := range nominalBW {
		occ := estimateOne(c.kind, c.bw, 40)
		if !occ.OK {
			t.Fatalf("%s: estimator returned not-ok", c.kind)
		}
		ratio := occ.BandwidthHz / c.bw
		t.Logf("%-8s nominal=%.0f occupied=%.0f ratio=%.2f", c.kind, c.bw, occ.BandwidthHz, ratio)
		if ratio < 0.5 || ratio > 1.6 {
			t.Errorf("%s: occupied bw %.0f out of sane range of nominal %.0f (ratio %.2f)",
				c.kind, occ.BandwidthHz, c.bw, ratio)
		}
		got[c.kind] = occ.BandwidthHz
	}
	order := []synth.Kind{synth.KindWFM, synth.KindDigital, synth.KindNFM, synth.KindSSB}
	for i := 1; i < len(order); i++ {
		if got[order[i-1]] <= got[order[i]] {
			t.Errorf("ordering violated: %s (%.0f) should be wider than %s (%.0f)",
				order[i-1], got[order[i-1]], order[i], got[order[i]])
		}
	}
	if got[synth.KindAM] <= got[synth.KindSSB] {
		t.Errorf("AM (%.0f) should be wider than SSB (%.0f)", got[synth.KindAM], got[synth.KindSSB])
	}
}

// The estimate must be stable at the SNRs real signals present (broadcast FM is
// 30 dB+). At 25 dB the estimate should agree with the near-clean one.
//
// Note (R2 target): at very low SNR (~15 dB) the weak skirts of skirt-heavy
// modulations (FM, AM) drop below the noise floor, so the measurable occupied
// bandwidth genuinely shrinks. Raising the effective noise floor via Welch
// averaging (Phase R, step R2) is what extends usable estimation downward.
func TestOccupiedBandwidthNoiseRobust(t *testing.T) {
	for _, c := range nominalBW {
		clean := estimateOne(c.kind, c.bw, 40)
		noisy := estimateOne(c.kind, c.bw, 25)
		if !clean.OK || !noisy.OK {
			t.Fatalf("%s: not-ok (clean=%v noisy=%v)", c.kind, clean.OK, noisy.OK)
		}
		err := math.Abs(noisy.BandwidthHz-clean.BandwidthHz) / clean.BandwidthHz
		t.Logf("%-8s clean=%.0f noisy(25dB)=%.0f drift=%.1f%%", c.kind, clean.BandwidthHz, noisy.BandwidthHz, err*100)
		if err > 0.40 {
			t.Errorf("%s: estimate drifts %.1f%% from clean to 25 dB SNR (>40%%)", c.kind, err*100)
		}
	}
}

// Refined peak-over-noise SNR must track the configured signal SNR: a 10 dB
// increase in signal power raises the peak ~10 dB while the noise floor is fixed.
func TestOccupiedSNRTracksConfigured(t *testing.T) {
	for _, kind := range []synth.Kind{synth.KindNFM, synth.KindDigital} {
		s10 := estimateOne(kind, 12e3, 10)
		s20 := estimateOne(kind, 12e3, 20)
		s30 := estimateOne(kind, 12e3, 30)
		if !s10.OK || !s20.OK || !s30.OK {
			t.Fatalf("%s: not-ok", kind)
		}
		d1 := s20.SNRDb() - s10.SNRDb()
		d2 := s30.SNRDb() - s20.SNRDb()
		t.Logf("%-8s SNRdB: 10dB->%.1f 20dB->%.1f 30dB->%.1f (steps %.1f, %.1f)",
			kind, s10.SNRDb(), s20.SNRDb(), s30.SNRDb(), d1, d2)
		if d1 < 5 || d1 > 15 || d2 < 5 || d2 > 15 {
			t.Errorf("%s: SNR steps (%.1f, %.1f) should be ~10 dB per 10 dB", kind, d1, d2)
		}
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

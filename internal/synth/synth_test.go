package synth

import (
	"math"
	"sort"
	"testing"

	fftutil "sdr-wideband-suite/internal/fft"
)

const (
	testFs = 2_500_000
	testN  = 8192
)

// freqToBin maps a baseband offset (Hz) to a bin index in the fftshifted
// spectrum produced by fftutil.Spectrum (DC at n/2).
func freqToBin(hz, fs float64, n int) int {
	binWidth := fs / float64(n)
	return n/2 + int(math.Round(hz/binWidth))
}

func median(v []float64) float64 {
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	return c[len(c)/2]
}

func maxInBand(spec []float64, centerBin, halfWidthBins int) (float64, int) {
	lo := centerBin - halfWidthBins
	hi := centerBin + halfWidthBins
	if lo < 0 {
		lo = 0
	}
	if hi >= len(spec) {
		hi = len(spec) - 1
	}
	best := math.Inf(-1)
	bestBin := lo
	for i := lo; i <= hi; i++ {
		if spec[i] > best {
			best = spec[i]
			bestBin = i
		}
	}
	return best, bestBin
}

func TestGeneratePlacesSignalsAtExpectedFrequencies(t *testing.T) {
	scene := Scene{
		SampleRate: testFs,
		Seed:       42,
		NoiseStd:   1.0,
		Signals: []SignalSpec{
			{Kind: KindWFM, CenterHz: 400e3, BandwidthHz: 180e3, SNRdB: 30},
			{Kind: KindNFM, CenterHz: 100e3, BandwidthHz: 12e3, SNRdB: 30},
			{Kind: KindCW, CenterHz: -200e3, BandwidthHz: 100, SNRdB: 30},
			{Kind: KindAM, CenterHz: -600e3, BandwidthHz: 8e3, SNRdB: 30},
		},
	}
	iq := scene.Generate(testN)
	if len(iq) != testN {
		t.Fatalf("expected %d samples, got %d", testN, len(iq))
	}
	spec := fftutil.Spectrum(iq, fftutil.Hann(testN))
	noiseFloor := median(spec)
	binWidth := float64(testFs) / float64(testN)

	for _, s := range scene.Signals {
		centerBin := freqToBin(s.CenterHz, float64(testFs), testN)
		halfWidth := int(s.BandwidthHz/binWidth)/2 + 3
		peak, peakBin := maxInBand(spec, centerBin, halfWidth)

		if peak-noiseFloor < 12 {
			t.Errorf("%s @ %.0f kHz: peak only %.1f dB above noise floor (want >12)",
				s.Kind, s.CenterHz/1e3, peak-noiseFloor)
		}
		// Peak must sit within the occupied band (tolerance scales with bandwidth).
		tolBins := int(s.BandwidthHz/binWidth)/2 + 2
		if d := peakBin - centerBin; d < -tolBins || d > tolBins {
			t.Errorf("%s @ %.0f kHz: peak bin %d off center bin %d by %d (tol %d)",
				s.Kind, s.CenterHz/1e3, peakBin, centerBin, d, tolBins)
		}
	}

	// A guard region far from every signal must stay near the noise floor.
	guardBin := freqToBin(900e3, float64(testFs), testN)
	guardPeak, _ := maxInBand(spec, guardBin, 20)
	if guardPeak-noiseFloor > 8 {
		t.Errorf("guard region not quiet: %.1f dB above noise floor", guardPeak-noiseFloor)
	}
}

func TestSNRControlsPeakHeight(t *testing.T) {
	mk := func(snr float64) float64 {
		scene := Scene{
			SampleRate: testFs, Seed: 7, NoiseStd: 1.0,
			Signals: []SignalSpec{{Kind: KindCW, CenterHz: 0, BandwidthHz: 100, SNRdB: snr}},
		}
		spec := fftutil.Spectrum(scene.Generate(testN), fftutil.Hann(testN))
		peak, _ := maxInBand(spec, testN/2, 4)
		return peak - median(spec)
	}
	low := mk(10)
	high := mk(40)
	if high <= low+10 {
		t.Errorf("higher SNR should raise the peak markedly: low=%.1f high=%.1f", low, high)
	}
}

func TestGenerateDeterministic(t *testing.T) {
	scene := Scene{
		SampleRate: testFs, Seed: 123, NoiseStd: 1.0,
		Signals: []SignalSpec{{Kind: KindNFM, CenterHz: 50e3, BandwidthHz: 12e3, SNRdB: 20}},
	}
	a := scene.Generate(testN)
	b := scene.Generate(testN)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic output at sample %d: %v vs %v", i, a[i], b[i])
		}
	}
}

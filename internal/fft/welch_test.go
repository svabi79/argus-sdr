package fftutil_test

import (
	"math"
	"testing"

	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/synth"
)

func std(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var m float64
	for _, x := range v {
		m += x
	}
	m /= float64(len(v))
	var s float64
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(v)))
}

// Welch averaging must markedly reduce the per-bin noise variance versus a single
// periodogram — this is what lowers the effective noise floor fluctuation and
// makes weak signals visible at low SNR (Phase R, step R2).
func TestWelchReducesNoiseVariance(t *testing.T) {
	const n = 4096
	const k = 16
	noise := synth.Scene{SampleRate: 2_500_000, Seed: 5, NoiseStd: 1.0}.Generate(n * k)

	single := fftutil.Spectrum(noise[:n], fftutil.Hann(n))
	welch := fftutil.WelchPSD(noise, n, 0.5, fftutil.Hann(n))

	sSingle := std(single)
	sWelch := std(welch)
	t.Logf("noise-floor std: single=%.2f dB  welch(%d-seg)=%.2f dB", sSingle, k, sWelch)
	if sWelch > 0.6*sSingle {
		t.Errorf("Welch did not reduce noise variance enough: single=%.2f welch=%.2f", sSingle, sWelch)
	}
}

// A real signal peak must survive Welch averaging (it averages noise, not the
// stationary signal).
func TestWelchPreservesSignalPeak(t *testing.T) {
	const n = 4096
	const k = 16
	scene := synth.Scene{
		SampleRate: 2_500_000, Seed: 9, NoiseStd: 1.0,
		Signals: []synth.SignalSpec{{Kind: synth.KindNFM, CenterHz: 0, BandwidthHz: 12e3, SNRdB: 20}},
	}
	iq := scene.Generate(n * k)
	welch := fftutil.WelchPSD(iq, n, 0.5, fftutil.Hann(n))
	// peak near DC (center bin) should stand well above the median noise floor.
	med := median4(welch)
	peak := math.Inf(-1)
	for i := n/2 - 20; i <= n/2+20; i++ {
		if welch[i] > peak {
			peak = welch[i]
		}
	}
	if peak-med < 10 {
		t.Errorf("signal peak only %.1f dB above noise after Welch", peak-med)
	}
}

func median4(v []float64) float64 {
	c := append([]float64(nil), v...)
	for i := 1; i < len(c); i++ {
		for j := i; j > 0 && c[j-1] > c[j]; j-- {
			c[j-1], c[j] = c[j], c[j-1]
		}
	}
	return c[len(c)/2]
}

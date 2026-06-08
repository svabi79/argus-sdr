//go:build bench

// IQ-classifier evaluation (Problem #3, classifier rework). The spectrum-only
// baseline (TestClassificationBaseline, ModeRule) scores 0-12%; the IQ modes
// (math/combined) were deferred. This generates per-kind baseband IQ (what the
// per-signal extraction feeds the classifier live), runs ModeCombined, and prints
// a confusion matrix + per-kind accuracy so the rework is measured, not guessed.
//
//	go test -tags bench -run TestMathClassifyConfusion ./internal/synth/ -v
package synth_test

import (
	"fmt"
	"sort"
	"testing"

	"sdr-wideband-suite/internal/classifier"
	"sdr-wideband-suite/internal/dsp"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/synth"
)

// extractToBW mimics the live per-signal extraction: low-pass the baseband signal
// to its occupied bandwidth so the classifier sees the signal, not the full-band
// noise (a 150 Hz CW in a 256 kHz window is otherwise pure noise).
func extractToBW(iq []complex64, fs int, bw float64) []complex64 {
	cutoff := bw / 2
	if cutoff < 100 {
		cutoff = 100
	}
	if cutoff > float64(fs)/2-1 {
		cutoff = float64(fs)/2 - 1
	}
	taps := dsp.LowpassFIR(cutoff, fs, 101)
	return dsp.ApplyFIR(iq, taps)
}

type kindCase struct {
	kind synth.Kind
	bw   float64
	fs   int
}

func classifierCases() []kindCase {
	return []kindCase{
		{synth.KindCW, 150, 256000},
		{synth.KindAM, 8000, 256000},
		{synth.KindSSB, 2700, 256000},
		{synth.KindNFM, 12000, 256000},
		{synth.KindWFM, 180000, 512000},
		{synth.KindFSK, 10000, 256000},
		{synth.KindPSK, 2400, 256000},
		{synth.KindDigital, 20000, 256000},
	}
}

// classifyKindIQ generates one signal of the kind at baseband (center 0) and runs
// the classifier the way the live per-signal path does (occupied IQ + spectrum).
func classifyKindIQ(kc kindCase, snr float64, seed int64, mode classifier.ClassifierMode) classifier.SignalClass {
	const n = 65536
	sc := synth.Scene{SampleRate: kc.fs, Seed: seed, NoiseStd: 1.0, Signals: []synth.SignalSpec{
		{Kind: kc.kind, CenterHz: 0, BandwidthHz: kc.bw, SNRdB: snr},
	}}
	iq := extractToBW(sc.Generate(n), kc.fs, kc.bw)
	win := fftutil.Hann(n)
	spec := fftutil.Spectrum(iq, win)
	binWidth := float64(kc.fs) / float64(n)
	hw := int(kc.bw/2/binWidth) + 1
	first := n/2 - hw
	last := n/2 + hw
	if first < 0 {
		first = 0
	}
	if last >= n {
		last = n - 1
	}
	in := classifier.SignalInput{FirstBin: first, LastBin: last, CenterHz: 0, BWHz: kc.bw, SNRDb: snr}
	cls := classifier.Classify(in, spec, kc.fs, n, iq, mode)
	if cls == nil {
		return classifier.ClassUnknown
	}
	return cls.ModType
}

// TestCarrierDCOffset measures how robust the DC-mean carrier metric is to a
// center-estimate error: the live extraction centers each signal on the detected
// center, which for a wide AM broadcast can be off by tens of Hz. CarrierDC
// (|mean(iq)|^2/pow) washes the carrier toward zero once the residual offset
// exceeds ~fs/N, so we generate AM/CW off-center and check the metric survives.
func TestCarrierDCOffset(t *testing.T) {
	const n = 65536
	offsets := []float64{0, 5, 20, 50, 100, 300, 1000}
	for _, kc := range []kindCase{{synth.KindAM, 8000, 256000}, {synth.KindCW, 150, 256000}} {
		t.Logf("--- %s bw=%.0f fs=%d  (fs/N=%.1f Hz) ---", kc.kind, kc.bw, kc.fs, float64(kc.fs)/n)
		for _, off := range offsets {
			sc := synth.Scene{SampleRate: kc.fs, Seed: 1, NoiseStd: 1.0, Signals: []synth.SignalSpec{
				{Kind: kc.kind, CenterHz: off, BandwidthHz: kc.bw, SNRdB: 30},
			}}
			iq := extractToBW(sc.Generate(n), kc.fs, kc.bw)
			mf := classifier.ExtractMathFeatures(iq)
			cls := classifier.MathClassify(mf, kc.bw, 0, 30)
			t.Logf("  off=%6.0f Hz  carrDC=%.3f  envCoV=%.3f  -> %s", off, mf.CarrierDC, mf.EnvCoV, cls.ModType)
		}
	}
}

func TestMathClassifyConfusion(t *testing.T) {
	snrs := []float64{15, 25, 35, 45}
	seeds := []int64{1, 2, 3, 4, 5}
	mode := classifier.ModeCombined

	// confusion[true][predicted]
	confusion := map[synth.Kind]map[classifier.SignalClass]int{}
	correct := map[synth.Kind]int{}
	total := map[synth.Kind]int{}
	allCorrect, allTotal := 0, 0

	for _, kc := range classifierCases() {
		confusion[kc.kind] = map[classifier.SignalClass]int{}
		for _, snr := range snrs {
			for _, seed := range seeds {
				got := classifyKindIQ(kc, snr, seed, mode)
				confusion[kc.kind][got]++
				total[kc.kind]++
				allTotal++
				if expectedClasses(kc.kind)[got] {
					correct[kc.kind]++
					allCorrect++
				}
			}
		}
	}

	t.Logf("=== IQ classifier (%s) confusion, %d SNRs x %d seeds per kind ===", mode, len(snrs), len(seeds))
	for _, kc := range classifierCases() {
		acc := 100.0 * float64(correct[kc.kind]) / float64(total[kc.kind])
		// top predictions for this true kind
		type pc struct {
			c classifier.SignalClass
			n int
		}
		var preds []pc
		for c, cnt := range confusion[kc.kind] {
			preds = append(preds, pc{c, cnt})
		}
		sort.Slice(preds, func(a, b int) bool { return preds[a].n > preds[b].n })
		parts := ""
		for i, p := range preds {
			if i >= 4 {
				break
			}
			parts += fmt.Sprintf(" %s:%d", p.c, p.n)
		}
		t.Logf("  %-8s acc=%5.1f%%  ->%s", kc.kind, acc, parts)
	}
	t.Logf("OVERALL accuracy = %.1f%% (%d/%d)", 100.0*float64(allCorrect)/float64(allTotal), allCorrect, allTotal)
}

// TestClassifierFeatureDump prints mean IQ features per kind so thresholds in
// MathClassify are set from data, not guessed (Constitution V).
func TestClassifierFeatureDump(t *testing.T) {
	const n = 65536
	seeds := []int64{1, 2, 3}
	for _, snr := range []float64{15, 30, 45} {
		t.Logf("=== SNR %.0f dB ===", snr)
		t.Logf("%-8s %6s %8s %8s %9s %8s", "kind", "envCoV", "ifStd", "carrDC", "ifMean", "ifModes")
		for _, kc := range classifierCases() {
			var envCoV, ifStd, carr, ifMean float64
			var modes int
			for _, seed := range seeds {
				sc := synth.Scene{SampleRate: kc.fs, Seed: seed, NoiseStd: 1.0, Signals: []synth.SignalSpec{
					{Kind: kc.kind, CenterHz: 0, BandwidthHz: kc.bw, SNRdB: snr},
				}}
				mf := classifier.ExtractMathFeatures(extractToBW(sc.Generate(n), kc.fs, kc.bw))
				envCoV += mf.EnvCoV
				ifStd += mf.InstFreqStd
				carr += mf.CarrierDC
				ifMean += mf.IFMean
				modes += mf.InstFreqModes
			}
			k := float64(len(seeds))
			t.Logf("%-8s %6.3f %8.4f %8.3f %9.4f %6d", kc.kind, envCoV/k, ifStd/k, carr/k, ifMean/k, modes/len(seeds))
		}
	}
}

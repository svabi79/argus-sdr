//go:build bench

// Ground-truth detection/estimation benchmark for the current pipeline.
// Build-tagged so it does not run in the normal `go test ./...` sweep.
//
//	go test -tags bench -run TestDetectionBaseline ./internal/synth/ -v
//
// See docs/detection-rework-plan-2026-06-06.md (Phase R, steps R0.3-R0.5).
package synth_test

import (
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	"os"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/dsp"
	"sdr-wideband-suite/internal/estimate"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/iqfile"
	"sdr-wideband-suite/internal/synth"
)

// TestRealTargetPilot extracts a target WFM station from the capture, FM-
// demodulates it, and checks for the 19 kHz stereo pilot (and 57 kHz RDS
// subcarrier). This isolates extraction QUALITY from the PLL/decoder: if the
// pilot is clearly present here, the no-lock is a downstream wiring/PLL issue,
// not the DSP chain.
func TestRealTargetPilot(t *testing.T) {
	const path = "../../data/snapshots/fm_bc.cf32"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("snapshot not present (%s)", path)
	}
	iq, meta, err := iqfile.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tgtMHz := range []float64{100.6, 102.5} {
		offset := tgtMHz*1e6 - meta.CenterHz
		// Use ~1.5 s of the capture.
		n := meta.SampleRate * 3 / 2
		if n > len(iq) {
			n = len(iq)
		}
		shifted := dsp.FreqShift(iq[:n], meta.SampleRate, offset)
		// Decimate to ~256 kHz in one stage (integer factor).
		decim := meta.SampleRate / 256000
		if decim < 1 {
			decim = 1
		}
		lp := dsp.LowpassFIR(float64(meta.SampleRate/decim)/2.0*0.8, meta.SampleRate, 101)
		base := dsp.Decimate(dsp.ApplyFIR(shifted, lp), decim)
		baseRate := meta.SampleRate / decim
		// FM discriminator: instantaneous frequency = angle(y[n]·conj(y[n-1])).
		disc := make([]complex64, len(base))
		for i := 1; i < len(base); i++ {
			p := base[i] * conj64(base[i-1])
			disc[i] = complex(float32(math.Atan2(float64(imag(p)), float64(real(p)))), 0)
		}
		// Spectrum of the discriminator; look for the 19 kHz pilot.
		fftN := 16384
		if fftN > len(disc) {
			fftN = len(disc)
		}
		spec := fftutil.Spectrum(disc[len(disc)-fftN:], fftutil.Hann(fftN))
		bw := float64(baseRate) / float64(fftN)
		binAt := func(hz float64) int { return fftN/2 + int(hz/bw) }
		floor := medianf(append([]float64(nil), spec...))
		pilot := spec[binAt(19000)] - floor
		rds := spec[binAt(57000)] - floor
		audio := spec[binAt(3000)] - floor
		t.Logf("%.1f MHz: audio(3k)=%.1f dB  PILOT(19k)=%.1f dB  RDS(57k)=%.1f dB above floor (baseRate=%d)",
			tgtMHz, audio, pilot, rds, baseRate)
	}
}

func conj64(c complex64) complex64 { return complex(real(c), -imag(c)) }

// liveDetectorConfig mirrors the user's aggressive live CFAR (from autosave):
// high scale + wide guard, which over-detects strong signals.
func liveDetectorConfig() config.DetectorConfig {
	c := detectorConfig()
	c.CFARScaleDb = 15
	c.CFARGuardHz = 250000
	c.CFARTrainHz = 100000
	return c
}

// TestRealJitter measures center/bandwidth jitter on a real FM capture (the
// 100.6 + 102.5 MHz targets), to compare against the synthetic baseline and to
// give the tracker fix a real-world target. Skips if the snapshot is absent.
//
//	go test -tags bench -run TestRealJitter ./internal/synth/ -v
func TestRealJitter(t *testing.T) {
	const path = "../../data/snapshots/fm_bc.cf32"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("snapshot not present (%s); capture with: sdrd -capture %s -capture-center 101.5", path, path)
	}
	iq, meta, err := iqfile.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	fft := 16384
	binWidth := float64(meta.SampleRate) / float64(fft)
	win := fftutil.Hann(fft)
	d := detector.New(liveDetectorConfig(), meta.SampleRate, fft)

	// Targets as offsets from the capture center (101.5 MHz).
	targets := []struct {
		name   string
		offset float64
	}{
		{"100.6", 100.6e6 - meta.CenterHz},
		{"102.5", 102.5e6 - meta.CenterHz},
	}
	type acc struct{ centers, bws []float64 }
	accs := make([]acc, len(targets))

	stride := meta.SampleRate / 12 // ~12 fps
	now := time.Unix(0, 0)
	frame := 0
	const warmup = 8
	for start := 0; start+fft <= len(iq); start += stride {
		spec := fftutil.Spectrum(iq[start:start+fft], win)
		now = now.Add(83 * time.Millisecond)
		d.Process(now, spec, 0)
		frame++
		if frame < warmup {
			continue
		}
		stable := d.StableSignals()
		for ti, tg := range targets {
			best, bestD := -1, math.MaxFloat64
			for j := range stable {
				if dd := math.Abs(stable[j].CenterHz - tg.offset); dd < bestD {
					best, bestD = j, dd
				}
			}
			if best >= 0 && bestD < 120e3 {
				accs[ti].centers = append(accs[ti].centers, stable[best].CenterHz)
				accs[ti].bws = append(accs[ti].bws, stable[best].BWHz)
			}
		}
	}

	t.Logf("Real FM capture jitter (%.3f MHz, %.3f MS/s, binWidth=%.0f Hz, %d frames):",
		meta.CenterHz/1e6, float64(meta.SampleRate)/1e6, binWidth, frame)
	t.Logf("%-8s %14s %12s %14s %6s", "target", "ctrJitterHz", "bwMeanHz", "bwJitterHz", "seen")
	for ti, tg := range targets {
		t.Logf("%-8s %14.0f %12.0f %14.0f %6d",
			tg.name, stddev(accs[ti].centers), mean(accs[ti].bws), stddev(accs[ti].bws), len(accs[ti].centers))
	}
}

const (
	benchFs     = 2_500_000
	benchN      = 8192
	benchFrames = 6
)

// detectorConfig mirrors the shipped config.yaml detector defaults.
func detectorConfig() config.DetectorConfig {
	return config.DetectorConfig{
		ThresholdDb: -60, MinDurationMs: 120, HoldMs: 1500, EmaAlpha: 0.35,
		HysteresisDb: 6, MinStableFrames: 3, GapToleranceMs: 1500,
		CFARMode: "GOSCA", CFARGuardHz: 15000, CFARTrainHz: 120000,
		CFARGuardCells: 3, CFARTrainCells: 24, CFARRank: 36, CFARScaleDb: 7,
		CFARWrapAround: true, EdgeMarginDb: 4, MaxSignalBwHz: 260000,
		MergeGapHz: 20000, ClassHistorySize: 10, ClassSwitchRatio: 0.6,
	}
}

// baseScene is the fixed signal layout; SNR is applied per run.
func baseScene(snrDb float64, seed int64) synth.Scene {
	mk := func(k synth.Kind, c, bw float64) synth.SignalSpec {
		return synth.SignalSpec{Kind: k, CenterHz: c, BandwidthHz: bw, SNRdB: snrDb}
	}
	return synth.Scene{
		SampleRate: benchFs, Seed: seed, NoiseStd: 1.0,
		Signals: []synth.SignalSpec{
			mk(synth.KindWFM, 800e3, 180e3),
			mk(synth.KindWFM, -800e3, 180e3),
			mk(synth.KindNFM, 200e3, 12e3),
			mk(synth.KindNFM, -300e3, 12e3),
			mk(synth.KindAM, 400e3, 8e3),
			mk(synth.KindSSB, -500e3, 3e3),
			mk(synth.KindCW, 50e3, 100),
			mk(synth.KindDigital, -100e3, 25e3),
		},
	}
}

// runDetector feeds benchFrames noisy realizations of the scene through the
// detector (advancing time so the EMA settles) and returns the last frame's raw
// signals.
func runDetector(d *detector.Detector, scene synth.Scene) []detector.Signal {
	win := fftutil.Hann(benchN)
	var raw []detector.Signal
	now := time.Unix(0, 0)
	for f := 0; f < benchFrames; f++ {
		s := scene
		s.Seed = scene.Seed + int64(f)
		spec := fftutil.Spectrum(s.Generate(benchN), win)
		now = now.Add(66 * time.Millisecond)
		_, raw = d.Process(now, spec, 0)
	}
	return raw
}

type metrics struct {
	tp, fp, fn int
	bwErrPct   []float64
	centerErr  []float64
}

func score(truth []synth.SignalSpec, got []detector.Signal, binWidth float64) metrics {
	used := make([]bool, len(got))
	var m metrics
	for _, t := range truth {
		bestIdx, bestDist := -1, math.MaxFloat64
		for i, g := range got {
			if used[i] {
				continue
			}
			tol := math.Max(t.BandwidthHz, g.BWHz)/2 + 2*binWidth
			d := math.Abs(g.CenterHz - t.CenterHz)
			if d <= tol && d < bestDist {
				bestIdx, bestDist = i, d
			}
		}
		if bestIdx < 0 {
			m.fn++
			continue
		}
		used[bestIdx] = true
		m.tp++
		g := got[bestIdx]
		if t.BandwidthHz > 0 {
			m.bwErrPct = append(m.bwErrPct, math.Abs(g.BWHz-t.BandwidthHz)/t.BandwidthHz*100)
		}
		m.centerErr = append(m.centerErr, math.Abs(g.CenterHz-t.CenterHz))
	}
	for i := range got {
		if !used[i] {
			m.fp++
		}
	}
	return m
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func medianf(v []float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	return c[len(c)/2]
}

func TestDetectionBaseline(t *testing.T) {
	binWidth := float64(benchFs) / float64(benchN)
	snrs := []float64{5, 10, 20, 30}

	t.Logf("Detection baseline — current pipeline (binWidth=%.0f Hz, %d truth signals/scene)", binWidth, len(baseScene(0, 0).Signals))
	t.Logf("%-6s %8s %8s %8s %10s %10s %8s", "SNRdB", "detect", "precis", "recall", "bwMAE%", "bwMed%", "ctrHz")

	for _, snr := range snrs {
		d := detector.New(detectorConfig(), benchFs, benchN)
		scene := baseScene(snr, 1000)
		got := runDetector(d, scene)
		m := score(scene.Signals, got, binWidth)

		prec := safeDiv(m.tp, m.tp+m.fp)
		rec := safeDiv(m.tp, m.tp+m.fn)
		t.Logf("%-6.0f %8d %8.2f %8.2f %10.1f %10.1f %8.0f",
			snr, len(got), prec, rec, mean(m.bwErrPct), medianf(m.bwErrPct), mean(m.centerErr))
	}

	// Per-kind bandwidth error at high SNR (where detection is easy) to expose
	// the geometric-bandwidth weakness regardless of detection misses.
	t.Logf("")
	t.Logf("Per-kind bandwidth error @ 30 dB SNR:")
	t.Logf("%-8s %10s %10s %10s", "kind", "truthBW", "detBW", "err%")
	d := detector.New(detectorConfig(), benchFs, benchN)
	scene := baseScene(30, 2000)
	got := runDetector(d, scene)
	for _, tr := range scene.Signals {
		best, bestD := -1, math.MaxFloat64
		for i, g := range got {
			dd := math.Abs(g.CenterHz - tr.CenterHz)
			if dd < bestD {
				best, bestD = i, dd
			}
		}
		if best < 0 || bestD > math.Max(tr.BandwidthHz, got[best].BWHz)/2+4*binWidth {
			t.Logf("%-8s %10.0f %10s %10s", tr.Kind, tr.BandwidthHz, "MISS", "-")
			continue
		}
		g := got[best]
		t.Logf("%-8s %10.0f %10.0f %10.1f", tr.Kind, tr.BandwidthHz, g.BWHz,
			math.Abs(g.BWHz-tr.BandwidthHz)/tr.BandwidthHz*100)
	}
}

// refTruthBW measures a signal's true occupied bandwidth from a near-noiseless,
// wide-region generation. This is the fair ground truth (the nominal Carson
// value overstates the 99%-occupied band for FM), used to score both the
// geometric and the refined estimate on equal footing.
func refTruthBW(kind synth.Kind, nominalBw float64) float64 {
	binWidth := float64(benchFs) / float64(benchN)
	scene := synth.Scene{
		SampleRate: benchFs, Seed: 7, NoiseStd: 1.0,
		Signals: []synth.SignalSpec{{Kind: kind, CenterHz: 0, BandwidthHz: nominalBw, SNRdB: 50}},
	}
	spec := fftutil.Spectrum(scene.Generate(benchN), fftutil.Hann(benchN))
	center := benchN / 2
	half := int(math.Max(4*nominalBw, 60e3) / binWidth)
	lo, hi := center-half, center+half
	if lo < 0 {
		lo = 0
	}
	if hi >= len(spec) {
		hi = len(spec) - 1
	}
	occ := estimate.OccupiedBandwidthDb(spec[lo:hi+1], binWidth, 0.99)
	return occ.BandwidthHz
}

// TestRefinedBandwidthVsGeometric proves R1: the occupied-bandwidth estimator
// recovers the true occupied bandwidth far better than the detector's geometric
// width, on the same scenes, scored against a noiseless reference truth.
func TestRefinedBandwidthVsGeometric(t *testing.T) {
	binWidth := float64(benchFs) / float64(benchN)
	win := fftutil.Hann(benchN)
	d := detector.New(detectorConfig(), benchFs, benchN)
	scene := baseScene(30, 2000)

	// Run frames; keep the final realization's spectrum for refinement.
	var raw []detector.Signal
	var spec []float64
	now := time.Unix(0, 0)
	for f := 0; f < benchFrames; f++ {
		s := scene
		s.Seed = scene.Seed + int64(f)
		spec = fftutil.Spectrum(s.Generate(benchN), win)
		now = now.Add(66 * time.Millisecond)
		_, raw = d.Process(now, spec, 0)
	}

	t.Logf("Refined vs geometric bandwidth @ 30 dB SNR (vs noiseless reference truth):")
	t.Logf("%-8s %9s %9s %9s %9s %9s", "kind", "refTruth", "geomBW", "geom%", "refBW", "ref%")
	var geomErrs, refErrs []float64
	var wfmRefErr float64
	for _, tr := range scene.Signals {
		best, bestD := -1, math.MaxFloat64
		for i, g := range raw {
			dd := math.Abs(g.CenterHz - tr.CenterHz)
			if dd < bestD {
				best, bestD = i, dd
			}
		}
		if best < 0 || bestD > math.Max(tr.BandwidthHz, raw[best].BWHz)/2+4*binWidth {
			t.Logf("%-8s %9s", tr.Kind, "MISS")
			continue
		}
		truth := refTruthBW(tr.Kind, tr.BandwidthHz)
		if truth <= 0 {
			continue
		}
		g := raw[best]
		ref := estimate.RefineFromSpectrum(spec, g.FirstBin, g.LastBin, binWidth, 0.99)
		geomErr := math.Abs(g.BWHz-truth) / truth * 100
		refErr := math.NaN()
		refBW := math.NaN()
		if ref.OK {
			refBW = ref.BandwidthHz
			refErr = math.Abs(ref.BandwidthHz-truth) / truth * 100
		}
		t.Logf("%-8s %9.0f %9.0f %9.1f %9.0f %9.1f", tr.Kind, truth, g.BWHz, geomErr, refBW, refErr)
		// Exclude the sub-bin CW signal from aggregate bandwidth stats.
		if tr.Kind != synth.KindCW {
			geomErrs = append(geomErrs, geomErr)
			if ref.OK {
				refErrs = append(refErrs, refErr)
			}
		}
		if tr.Kind == synth.KindWFM && ref.OK {
			wfmRefErr = refErr
		}
	}

	geomMed := medianf(geomErrs)
	refMed := medianf(refErrs)
	t.Logf("median bw error vs reference truth (excl. CW): geometric=%.1f%%  refined=%.1f%%", geomMed, refMed)

	if refMed > 0.7*geomMed {
		t.Errorf("refined median bw error %.1f%% should be substantially better than geometric %.1f%%", refMed, geomMed)
	}
	if refMed > 30 {
		t.Errorf("refined median bw error %.1f%% should be under 30%%", refMed)
	}
	if wfmRefErr > 35 {
		t.Errorf("refined WFM bw error %.1f%% should be under 35%% (geometric was ~48%%)", wfmRefErr)
	}
}

// bwFromSpec runs the occupied-bandwidth estimator on a wide region of spec
// around DC (the single signal sits at center 0).
func bwFromSpec(spec []float64, nominalBw float64) float64 {
	binWidth := float64(benchFs) / float64(benchN)
	center := benchN / 2
	half := int(math.Max(4*nominalBw, 60e3) / binWidth)
	lo, hi := center-half, center+half
	if lo < 0 {
		lo = 0
	}
	if hi >= len(spec) {
		hi = len(spec) - 1
	}
	return estimate.OccupiedBandwidthDb(spec[lo:hi+1], binWidth, 0.99).BandwidthHz
}

// TestWelchStabilizesLowSNRBandwidth proves R2: at low SNR a Welch-averaged PSD
// gives a more accurate occupied bandwidth than a single FFT, because the stable
// noise floor keeps the weak skirts of FM/AM visible. This directly fixes the
// low-SNR bandwidth shrink seen in R1.
func TestWelchStabilizesLowSNRBandwidth(t *testing.T) {
	win := fftutil.Hann(benchN)
	const snr = 15.0
	var sumS, sumW float64
	kinds := []synth.Kind{synth.KindWFM, synth.KindAM}
	for _, kind := range kinds {
		nominal := map[synth.Kind]float64{synth.KindWFM: 180e3, synth.KindAM: 8e3}[kind]
		ref := refTruthBW(kind, nominal)

		single := fftutil.Spectrum(sceneOne(kind, nominal, snr, 11).Generate(benchN), win)
		welch := fftutil.WelchPSD(sceneOne(kind, nominal, snr, 11).Generate(benchN*8), benchN, 0.5, win)

		errS := math.Abs(bwFromSpec(single, nominal)-ref) / ref * 100
		errW := math.Abs(bwFromSpec(welch, nominal)-ref) / ref * 100
		t.Logf("%-4s @ %.0f dB: refTruth=%.0f  single err=%.1f%%  welch err=%.1f%%", kind, snr, ref, errS, errW)
		sumS += errS
		sumW += errW
	}
	avgS, avgW := sumS/float64(len(kinds)), sumW/float64(len(kinds))
	t.Logf("mean bw error @ %.0f dB: single-FFT=%.1f%%  welch=%.1f%%", snr, avgS, avgW)
	if avgW > avgS {
		t.Errorf("Welch mean bw error %.1f%% should be better than single-FFT %.1f%%", avgW, avgS)
	}
}

func sceneOne(kind synth.Kind, bw, snr float64, seed int64) synth.Scene {
	return synth.Scene{
		SampleRate: benchFs, Seed: seed, NoiseStd: 1.0,
		Signals: []synth.SignalSpec{{Kind: kind, CenterHz: 0, BandwidthHz: bw, SNRdB: snr}},
	}
}

// TestCenterTrackingFollowsDrift validates the switchable center filter: a
// drifting carrier (LEO Doppler) should be followed in "tracking" mode but
// heavily lagged in the default "quiet" mode.
func TestCenterTrackingFollowsDrift(t *testing.T) {
	fft := benchN
	win := fftutil.Hann(fft)
	const driftPerFrame = 1500.0 // Hz/frame (~18 kHz/s at 12 fps)

	run := func(mode string) float64 {
		cfg := detectorConfig()
		cfg.CenterTrackMode = mode
		d := detector.New(cfg, benchFs, fft)
		now := time.Unix(0, 0)
		var lag []float64
		for f := 0; f < 45; f++ {
			trueCenter := -300e3 + driftPerFrame*float64(f)
			scene := synth.Scene{
				SampleRate: benchFs, Seed: 7000 + int64(f), NoiseStd: 1.0,
				Signals: []synth.SignalSpec{{Kind: synth.KindWFM, CenterHz: trueCenter, BandwidthHz: 180e3, SNRdB: 40}},
			}
			spec := fftutil.Spectrum(scene.Generate(fft), win)
			now = now.Add(83 * time.Millisecond)
			d.Process(now, spec, 0)
			if f < 18 {
				continue
			}
			best, bestD := -1, math.MaxFloat64
			stable := d.StableSignals()
			for j := range stable {
				if dd := math.Abs(stable[j].CenterHz - trueCenter); dd < bestD {
					best, bestD = j, dd
				}
			}
			if best >= 0 {
				lag = append(lag, math.Abs(stable[best].CenterHz-trueCenter))
			}
		}
		return mean(lag)
	}

	quietLag := run("quiet")
	trackLag := run("tracking")
	t.Logf("drift %.0f Hz/frame: quiet lag=%.0f Hz, tracking lag=%.0f Hz", driftPerFrame, quietLag, trackLag)
	if trackLag > 0.4*quietLag {
		t.Errorf("tracking mode lag %.0f Hz should be far below quiet %.0f Hz", trackLag, quietLag)
	}
}

func stddev(v []float64) float64 {
	if len(v) < 2 {
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

// denseFMScene places several strong WFM stations at realistic UKW spacing,
// including a very strong one (to reproduce the over-wide detection seen live).
func denseFMScene(snrSeed int64) synth.Scene {
	mk := func(c, snr float64) synth.SignalSpec {
		return synth.SignalSpec{Kind: synth.KindWFM, CenterHz: c, BandwidthHz: 180e3, SNRdB: snr}
	}
	return synth.Scene{
		SampleRate: benchFs, Seed: snrSeed, NoiseStd: 1.0,
		Signals: []synth.SignalSpec{
			mk(-900e3, 50), mk(-500e3, 35), mk(-150e3, 55), mk(250e3, 40), mk(650e3, 45),
		},
	}
}

// TestDetectionJitter measures, per station, how much the tracked center and
// bandwidth wobble frame to frame on a dense/strong FM band. This is the
// stability the WFM stereo pilot PLL needs; the std-dev is the number a stable
// tracker must drive down (foundation for the lock fix).
func TestDetectionJitter(t *testing.T) {
	binWidth := float64(benchFs) / float64(benchN)
	d := detector.New(detectorConfig(), benchFs, benchN)
	win := fftutil.Hann(benchN)
	scene := denseFMScene(5000)

	type acc struct{ centers, bws []float64 }
	accs := make([]acc, len(scene.Signals))
	now := time.Unix(0, 0)
	const frames, warmup = 50, 12
	for f := 0; f < frames; f++ {
		s := scene
		s.Seed = scene.Seed + int64(f)
		spec := fftutil.Spectrum(s.Generate(benchN), win)
		now = now.Add(83 * time.Millisecond)
		d.Process(now, spec, 0)
		if f < warmup {
			continue
		}
		stable := d.StableSignals()
		for i, tr := range scene.Signals {
			best, bestD := -1, math.MaxFloat64
			for j := range stable {
				if dd := math.Abs(stable[j].CenterHz - tr.CenterHz); dd < bestD {
					best, bestD = j, dd
				}
			}
			if best >= 0 && bestD < 120e3 {
				accs[i].centers = append(accs[i].centers, stable[best].CenterHz)
				accs[i].bws = append(accs[i].bws, stable[best].BWHz)
			}
		}
	}

	t.Logf("Detection jitter on dense FM band (binWidth=%.0f Hz, %d frames):", binWidth, frames-warmup)
	t.Logf("%-10s %5s %14s %14s %14s %5s", "station", "snr", "ctrJitterHz", "bwMeanHz", "bwJitterHz", "seen")
	for i, tr := range scene.Signals {
		t.Logf("%-10.0f %5.0f %14.0f %14.0f %14.0f %5d",
			tr.CenterHz/1e3, tr.SNRdB, stddev(accs[i].centers), mean(accs[i].bws), stddev(accs[i].bws), len(accs[i].centers))
	}
}

func safeDiv(a, b int) float64 {
	if b == 0 {
		return math.NaN()
	}
	return float64(a) / float64(b)
}

var _ = fmt.Sprintf

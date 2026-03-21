package runtime

import (
	"testing"

	"sdr-wideband-suite/internal/config"
)

func TestApplyConfigUpdate(t *testing.T) {
	cfg := config.Default()
	mgr := New(cfg)

	center := 7.2e6
	sampleRate := 1_024_000
	fftSize := 4096
	threshold := -35.0
	bw := 1536
	cfarMode := "OS"
	cfarWrap := true
	cfarGuard := 2
	cfarTrain := 12
	cfarRank := 18
	cfarScale := 5.5

	mode := "wideband-balanced"
	survFPS := 12
	maxRefJobs := 24
	updated, err := mgr.ApplyConfig(ConfigUpdate{
		CenterHz:   &center,
		SampleRate: &sampleRate,
		FFTSize:    &fftSize,
		TunerBwKHz: &bw,
		Pipeline:   &PipelineUpdate{Mode: &mode},
		Surveillance: &SurveillanceUpdate{FrameRate: &survFPS},
		Resources: &ResourcesUpdate{MaxRefinementJobs: &maxRefJobs},
		Detector: &DetectorUpdate{
			ThresholdDb:    &threshold,
			CFARMode:       &cfarMode,
			CFARWrapAround: &cfarWrap,
			CFARGuardCells: &cfarGuard,
			CFARTrainCells: &cfarTrain,
			CFARRank:       &cfarRank,
			CFARScaleDb:    &cfarScale,
		},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if updated.CenterHz != center {
		t.Fatalf("center hz: %v", updated.CenterHz)
	}
	if updated.SampleRate != sampleRate {
		t.Fatalf("sample rate: %v", updated.SampleRate)
	}
	if updated.FFTSize != fftSize {
		t.Fatalf("fft size: %v", updated.FFTSize)
	}
	if updated.Surveillance.AnalysisFFTSize != fftSize {
		t.Fatalf("analysis fft size: %v", updated.Surveillance.AnalysisFFTSize)
	}
	if updated.Detector.ThresholdDb != threshold {
		t.Fatalf("threshold: %v", updated.Detector.ThresholdDb)
	}
	if updated.Detector.CFARMode != cfarMode {
		t.Fatalf("cfar mode: %v", updated.Detector.CFARMode)
	}
	if updated.Detector.CFARWrapAround != cfarWrap {
		t.Fatalf("cfar wrap: %v", updated.Detector.CFARWrapAround)
	}
	if updated.Detector.CFARGuardCells != cfarGuard {
		t.Fatalf("cfar guard: %v", updated.Detector.CFARGuardCells)
	}
	if updated.Detector.CFARTrainCells != cfarTrain {
		t.Fatalf("cfar train: %v", updated.Detector.CFARTrainCells)
	}
	if updated.Detector.CFARRank != cfarRank {
		t.Fatalf("cfar rank: %v", updated.Detector.CFARRank)
	}
	if updated.Detector.CFARScaleDb != cfarScale {
		t.Fatalf("cfar scale: %v", updated.Detector.CFARScaleDb)
	}
	if updated.TunerBwKHz != bw {
		t.Fatalf("tuner bw: %v", updated.TunerBwKHz)
	}
	if updated.Pipeline.Mode != mode {
		t.Fatalf("pipeline mode: %v", updated.Pipeline.Mode)
	}
	if updated.Surveillance.FrameRate != survFPS || updated.FrameRate != survFPS {
		t.Fatalf("surveillance frame rate: %v / %v", updated.Surveillance.FrameRate, updated.FrameRate)
	}
	if updated.Resources.MaxRefinementJobs != maxRefJobs {
		t.Fatalf("max refinement jobs: %v", updated.Resources.MaxRefinementJobs)
	}
}

func TestApplyConfigRejectsInvalid(t *testing.T) {
	cfg := config.Default()

	{
		mgr := New(cfg)
		bad := 0
		if _, err := mgr.ApplyConfig(ConfigUpdate{SampleRate: &bad}); err == nil {
			t.Fatalf("expected error")
		}
		snap := mgr.Snapshot()
		if snap.SampleRate != cfg.SampleRate {
			t.Fatalf("sample rate changed on error")
		}
	}

	{
		mgr := New(cfg)
		badAlpha := -0.5
		if _, err := mgr.ApplyConfig(ConfigUpdate{Detector: &DetectorUpdate{EmaAlpha: &badAlpha}}); err == nil {
			t.Fatalf("expected ema_alpha error")
		}
		if mgr.Snapshot().Detector.EmaAlpha != cfg.Detector.EmaAlpha {
			t.Fatalf("ema_alpha changed on error")
		}
	}

	{
		mgr := New(cfg)
		badAlpha := 1.5
		if _, err := mgr.ApplyConfig(ConfigUpdate{Detector: &DetectorUpdate{EmaAlpha: &badAlpha}}); err == nil {
			t.Fatalf("expected ema_alpha upper bound error")
		}
		if mgr.Snapshot().Detector.EmaAlpha != cfg.Detector.EmaAlpha {
			t.Fatalf("ema_alpha changed on error")
		}
	}

	{
		mgr := New(cfg)
		badHyst := -1.0
		if _, err := mgr.ApplyConfig(ConfigUpdate{Detector: &DetectorUpdate{HysteresisDb: &badHyst}}); err == nil {
			t.Fatalf("expected hysteresis_db error")
		}
		if mgr.Snapshot().Detector.HysteresisDb != cfg.Detector.HysteresisDb {
			t.Fatalf("hysteresis_db changed on error")
		}
	}

	{
		mgr := New(cfg)
		badStable := 0
		if _, err := mgr.ApplyConfig(ConfigUpdate{Detector: &DetectorUpdate{MinStableFrames: &badStable}}); err == nil {
			t.Fatalf("expected min_stable_frames error")
		}
		if mgr.Snapshot().Detector.MinStableFrames != cfg.Detector.MinStableFrames {
			t.Fatalf("min_stable_frames changed on error")
		}
	}

	{
		mgr := New(cfg)
		badGap := -10
		if _, err := mgr.ApplyConfig(ConfigUpdate{Detector: &DetectorUpdate{GapToleranceMs: &badGap}}); err == nil {
			t.Fatalf("expected gap_tolerance_ms error")
		}
		if mgr.Snapshot().Detector.GapToleranceMs != cfg.Detector.GapToleranceMs {
			t.Fatalf("gap_tolerance_ms changed on error")
		}
	}
}

func TestApplySettings(t *testing.T) {
	cfg := config.Default()
	mgr := New(cfg)
	agc := true
	dc := true
	iq := true
	updated, err := mgr.ApplySettings(SettingsUpdate{
		AGC:       &agc,
		DCBlock:   &dc,
		IQBalance: &iq,
	})
	if err != nil {
		t.Fatalf("apply settings: %v", err)
	}
	if !updated.AGC || !updated.DCBlock || !updated.IQBalance {
		t.Fatalf("settings not applied: %+v", updated)
	}
}

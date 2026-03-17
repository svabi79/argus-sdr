package runtime

import (
	"testing"

	"sdr-visual-suite/internal/config"
)

func TestApplyConfigUpdate(t *testing.T) {
	cfg := config.Default()
	mgr := New(cfg)

	center := 7.2e6
	sampleRate := 1_024_000
	fftSize := 4096
	threshold := -35.0

	updated, err := mgr.ApplyConfig(ConfigUpdate{
		CenterHz:   &center,
		SampleRate: &sampleRate,
		FFTSize:    &fftSize,
		Detector: &DetectorUpdate{
			ThresholdDb: &threshold,
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
	if updated.Detector.ThresholdDb != threshold {
		t.Fatalf("threshold: %v", updated.Detector.ThresholdDb)
	}
}

func TestApplyConfigRejectsInvalid(t *testing.T) {
	cfg := config.Default()
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

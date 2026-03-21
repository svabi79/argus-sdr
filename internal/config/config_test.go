package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	data := []byte("center_hz: 100.0e6\nfft_size: 1024\n")
	f, err := os.CreateTemp(t.TempDir(), "cfg*.yaml")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.CenterHz != 100.0e6 {
		t.Fatalf("center hz: %v", cfg.CenterHz)
	}
	if cfg.FFTSize != 2048 {
		t.Fatalf("fft size: %v", cfg.FFTSize)
	}
	if cfg.Surveillance.AnalysisFFTSize != 2048 {
		t.Fatalf("analysis fft size: %v", cfg.Surveillance.AnalysisFFTSize)
	}
	if cfg.FrameRate <= 0 {
		t.Fatalf("frame rate default not applied")
	}
	if cfg.Surveillance.AnalysisFFTSize != cfg.FFTSize {
		t.Fatalf("analysis fft size not aligned: %d vs %d", cfg.Surveillance.AnalysisFFTSize, cfg.FFTSize)
	}
	if cfg.Pipeline.Mode == "" {
		t.Fatalf("pipeline mode default not applied")
	}
	if !cfg.Refinement.Enabled {
		t.Fatalf("refinement default not applied")
	}
	if cfg.Refinement.AutoSpan == nil || !*cfg.Refinement.AutoSpan {
		t.Fatalf("refinement auto_span default not applied")
	}
	if cfg.EventPath == "" {
		t.Fatalf("event path default not applied")
	}
}

func TestProfileDefaultsPresent(t *testing.T) {
	cfg := Default()
	if len(cfg.Profiles) < 2 {
		t.Fatalf("expected built-in profiles")
	}
	found := false
	for _, p := range cfg.Profiles {
		if p.Name == "wideband-balanced" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing wideband-balanced profile")
	}
}

func TestRefinementSpanDefaults(t *testing.T) {
	cfg := Default()
	cfg.Refinement.MinSpanHz = 20000
	cfg.Refinement.MaxSpanHz = 10000
	cfg = applyDefaults(cfg)
	if cfg.Refinement.MaxSpanHz != cfg.Refinement.MinSpanHz {
		t.Fatalf("expected max span to clamp to min when inverted: %+v", cfg.Refinement)
	}
}

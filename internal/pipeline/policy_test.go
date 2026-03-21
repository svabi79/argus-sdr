package pipeline

import (
	"testing"

	"sdr-wideband-suite/internal/config"
)

func TestApplyNamedProfile(t *testing.T) {
	cfg := config.Default()
	ApplyNamedProfile(&cfg, "wideband-balanced")
	if cfg.Pipeline.Mode != "wideband-balanced" {
		t.Fatalf("mode not applied: %s", cfg.Pipeline.Mode)
	}
	if cfg.Surveillance.AnalysisFFTSize < 4096 {
		t.Fatalf("analysis fft too small: %d", cfg.Surveillance.AnalysisFFTSize)
	}
	if !cfg.Refinement.Enabled {
		t.Fatalf("refinement should stay enabled")
	}
	if cfg.Resources.MaxRefinementJobs < 16 {
		t.Fatalf("refinement jobs too small: %d", cfg.Resources.MaxRefinementJobs)
	}
}

func TestPolicyFromConfig(t *testing.T) {
	cfg := config.Default()
	cfg.Pipeline.Mode = "archive"
	cfg.Surveillance.AnalysisFFTSize = 8192
	cfg.Surveillance.FrameRate = 9
	cfg.Refinement.Enabled = true
	cfg.Resources.MaxRefinementJobs = 5
	cfg.Refinement.MinCandidateSNRDb = 2.5
	cfg.Resources.PreferGPU = true
	p := PolicyFromConfig(cfg)
	if p.Mode != "archive" || p.SurveillanceFFTSize != 8192 || p.SurveillanceFPS != 9 {
		t.Fatalf("unexpected policy: %+v", p)
	}
	if !p.RefinementEnabled || p.MaxRefinementJobs != 5 || p.MinCandidateSNRDb != 2.5 || !p.PreferGPU {
		t.Fatalf("unexpected policy details: %+v", p)
	}
}

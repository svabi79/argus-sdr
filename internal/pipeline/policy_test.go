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
	if cfg.Pipeline.Goals.Intent != "wideband-surveillance" {
		t.Fatalf("intent not applied: %s", cfg.Pipeline.Goals.Intent)
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
	cfg.Pipeline.Goals.Intent = "archive-and-triage"
	cfg.Pipeline.Goals.MonitorStartHz = 88e6
	cfg.Pipeline.Goals.MonitorEndHz = 108e6
	cfg.Pipeline.Goals.MonitorSpanHz = 20e6
	cfg.Pipeline.Goals.SignalPriorities = []string{"broadcast-fm", "rds"}
	cfg.Surveillance.AnalysisFFTSize = 8192
	cfg.Surveillance.FrameRate = 9
	cfg.Surveillance.DisplayBins = 1200
	cfg.Surveillance.DisplayFPS = 6
	cfg.Refinement.Enabled = true
	cfg.Resources.MaxRefinementJobs = 5
	cfg.Refinement.MinCandidateSNRDb = 2.5
	cfg.Resources.PreferGPU = true
	p := PolicyFromConfig(cfg)
	if p.Mode != "archive" || p.Intent != "archive-and-triage" || p.SurveillanceFFTSize != 8192 || p.SurveillanceFPS != 9 || p.DisplayBins != 1200 || p.DisplayFPS != 6 {
		t.Fatalf("unexpected policy: %+v", p)
	}
	if p.MonitorSpanHz != 20e6 || len(p.SignalPriorities) != 2 {
		t.Fatalf("unexpected policy goals: %+v", p)
	}
	if p.MonitorCenterHz != cfg.CenterHz {
		t.Fatalf("unexpected monitor center: %+v", p.MonitorCenterHz)
	}
	if !p.RefinementEnabled || p.MaxRefinementJobs != 5 || p.MinCandidateSNRDb != 2.5 || !p.PreferGPU {
		t.Fatalf("unexpected policy details: %+v", p)
	}
}

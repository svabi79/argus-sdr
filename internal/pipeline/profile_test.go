package pipeline

import (
	"testing"

	"sdr-wideband-suite/internal/config"
)

func TestResolveAndMergeProfile(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles = append(cfg.Profiles, config.ProfileConfig{
		Name:        "custom-test",
		Description: "test profile",
		Pipeline: &config.PipelineConfig{Mode: "custom", Goals: config.PipelineGoalConfig{Intent: "custom-intent", MonitorSpanHz: 12.5e6}},
		Surveillance: &config.SurveillanceConfig{AnalysisFFTSize: 16384, FrameRate: 8, Strategy: "single-resolution"},
		Refinement: &config.RefinementConfig{Enabled: true, MaxConcurrent: 20, MinCandidateSNRDb: 4},
		Resources: &config.ResourceConfig{PreferGPU: true, MaxRefinementJobs: 20, MaxRecordingStreams: 32},
	})
	p, ok := ResolveProfile(cfg, "custom-test")
	if !ok {
		t.Fatalf("expected profile")
	}
	MergeProfile(&cfg, p)
	if cfg.Pipeline.Mode != "custom" || cfg.Pipeline.Goals.Intent != "custom-intent" {
		t.Fatalf("pipeline not merged: %+v", cfg.Pipeline)
	}
	if cfg.FFTSize != 16384 || cfg.FrameRate != 8 {
		t.Fatalf("surveillance not merged into legacy fields: fft=%d fps=%d", cfg.FFTSize, cfg.FrameRate)
	}
	if cfg.Resources.MaxRefinementJobs != 20 {
		t.Fatalf("resources not merged: %+v", cfg.Resources)
	}
}

package main

import (
	"testing"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/detector"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/pipeline"
)

func TestNewDSPRuntime(t *testing.T) {
	cfg := config.Default()
	det := detector.New(cfg.Detector, cfg.SampleRate, cfg.FFTSize)
	window := fftutil.Hann(cfg.FFTSize)
	rt := newDSPRuntime(cfg, det, window, &gpuStatus{}, nil)
	if rt == nil {
		t.Fatalf("runtime is nil")
	}
	if rt.plan == nil {
		t.Fatalf("fft plan is nil")
	}
	if rt.cfg.FFTSize != cfg.FFTSize {
		t.Fatalf("unexpected fft size: %d", rt.cfg.FFTSize)
	}
}

func TestScheduledCandidateSelectionUsesPolicy(t *testing.T) {
	cfg := config.Default()
	cfg.Resources.MaxRefinementJobs = 1
	cfg.Refinement.MinCandidateSNRDb = 6
	policy := pipeline.PolicyFromConfig(cfg)
	got := pipeline.ScheduleCandidates([]pipeline.Candidate{
		{ID: 1, SNRDb: 3, BandwidthHz: 1000},
		{ID: 2, SNRDb: 12, BandwidthHz: 5000},
		{ID: 3, SNRDb: 8, BandwidthHz: 7000},
	}, policy)
	if len(got) != 2 {
		t.Fatalf("expected 2 scheduled candidates after gating, got %d", len(got))
	}
	if got[0].Candidate.ID != 2 {
		t.Fatalf("expected highest priority candidate, got %d", got[0].Candidate.ID)
	}
}

func TestSurveillanceLevelsRespectStrategy(t *testing.T) {
	cfg := config.Default()
	det := detector.New(cfg.Detector, cfg.SampleRate, cfg.FFTSize)
	window := fftutil.Hann(cfg.FFTSize)
	rt := newDSPRuntime(cfg, det, window, &gpuStatus{}, nil)
	policy := pipeline.Policy{SurveillanceStrategy: "single-resolution"}
	plan := rt.buildSurveillancePlan(policy)
	if len(plan.Levels) != 1 {
		t.Fatalf("expected single level for single-resolution, got %d", len(plan.Levels))
	}
	if plan.Levels[0].Role != pipeline.RoleSurveillancePrimary {
		t.Fatalf("expected primary role, got %q", plan.Levels[0].Role)
	}
	policy.SurveillanceStrategy = "multi-res"
	policy.Intent = "wideband-surveillance"
	plan = rt.buildSurveillancePlan(policy)
	if len(plan.Levels) != 2 {
		t.Fatalf("expected secondary level for multi-res, got %d", len(plan.Levels))
	}
	if plan.Levels[1].Decimation != 2 {
		t.Fatalf("expected decimation factor 2, got %d", plan.Levels[1].Decimation)
	}
	if plan.Levels[1].Role != pipeline.RoleSurveillanceDerived {
		t.Fatalf("expected derived role, got %q", plan.Levels[1].Role)
	}
}

func TestWindowSpanBounds(t *testing.T) {
	windows := []pipeline.RefinementWindow{
		{SpanHz: 8000},
		{SpanHz: 16000},
		{SpanHz: 12000},
	}
	minSpan, maxSpan, ok := windowSpanBounds(windows)
	if !ok {
		t.Fatalf("expected spans to be found")
	}
	if minSpan != 8000 || maxSpan != 16000 {
		t.Fatalf("unexpected span bounds: min %.0f max %.0f", minSpan, maxSpan)
	}
}

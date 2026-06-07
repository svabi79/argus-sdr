package main

import (
	"math"
	"testing"
	"time"

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

// TestScaleAwareFusionWiring verifies the L1-B live wiring (tagless, no GPU):
// with Detector.ScaleAwareFusion on, a sharp detector is created and
// buildSurveillanceResult routes through pipeline.FuseScaleAware — a bridged
// coarse candidate is split into two emissions that adopt the sharp pass's
// stable, offset IDs (Principle II); off, no sharp detector is created.
func TestScaleAwareFusionWiring(t *testing.T) {
	cfg := config.Default()
	if rt := newDSPRuntime(cfg, detector.New(cfg.Detector, cfg.SampleRate, cfg.FFTSize), fftutil.Hann(cfg.FFTSize), &gpuStatus{}, nil); rt.detSharp != nil {
		t.Fatalf("scale-aware off by default: detSharp should be nil")
	}

	cfg.Detector.ScaleAwareFusion = true
	det := detector.New(cfg.Detector, cfg.SampleRate, cfg.FFTSize)
	rt := newDSPRuntime(cfg, det, fftutil.Hann(cfg.FFTSize), &gpuStatus{}, nil)
	if rt.detSharp == nil {
		t.Fatalf("scale-aware on: detSharp should be created")
	}

	c := cfg.CenterHz
	plan := rt.buildSurveillancePlan(pipeline.PolicyFromConfig(cfg))
	art := &spectrumArtifacts{
		now:              time.Now(),
		surveillancePlan: plan,
		// Coarse: one bridged blob spanning two stations.
		detected: []detector.Signal{{ID: 1, CenterHz: c, BWHz: 400e3, SNRDb: 55}},
		// Sharp: the two stations resolved.
		sharpDetected: []detector.Signal{
			{ID: 10, CenterHz: c - 100e3, BWHz: 120e3, SNRDb: 50},
			{ID: 11, CenterHz: c + 100e3, BWHz: 120e3, SNRDb: 50},
		},
	}
	res := rt.buildSurveillanceResult(art)
	if len(res.Candidates) != 2 {
		t.Fatalf("expected the bridged candidate split into 2, got %d", len(res.Candidates))
	}
	// Each child sits near a sharp center and carries the offset sharp ID.
	wantIDs := map[int64]float64{rt.sharpIDBase - 10: c - 100e3, rt.sharpIDBase - 11: c + 100e3}
	for _, cd := range res.Candidates {
		wantCenter, ok := wantIDs[cd.ID]
		if !ok {
			t.Errorf("child has unexpected ID %d (want offset sharp IDs %d/%d)", cd.ID, rt.sharpIDBase-10, rt.sharpIDBase-11)
			continue
		}
		if math.Abs(cd.CenterHz-wantCenter) > 1 {
			t.Errorf("child ID %d center %.0f, want %.0f", cd.ID, cd.CenterHz, wantCenter)
		}
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

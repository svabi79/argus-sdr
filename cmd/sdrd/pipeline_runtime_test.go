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
	rt := newDSPRuntime(cfg, det, window, &gpuStatus{})
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
	if len(got) != 1 {
		t.Fatalf("expected 1 scheduled candidate, got %d", len(got))
	}
	if got[0].Candidate.ID != 2 {
		t.Fatalf("expected highest priority candidate, got %d", got[0].Candidate.ID)
	}
}

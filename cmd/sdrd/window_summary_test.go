package main

import (
	"testing"

	"sdr-wideband-suite/internal/pipeline"
)

func TestBuildMonitorWindowSummaryCountsCandidates(t *testing.T) {
	windows := []pipeline.MonitorWindow{
		{Index: 0, Label: "primary", StartHz: 100, EndHz: 200, CenterHz: 150, SpanHz: 100},
		{Index: 1, Label: "secondary", StartHz: 300, EndHz: 400, CenterHz: 350, SpanHz: 100},
	}
	candidates := []pipeline.Candidate{
		{ID: 1, CenterHz: 150, BandwidthHz: 20},
		{ID: 2, CenterHz: 320, BandwidthHz: 10},
	}
	summary := buildMonitorWindowSummary(windows, nil, candidates)
	if len(summary) != 2 {
		t.Fatalf("expected 2 window summaries, got %d", len(summary))
	}
	if summary[0].Candidates != 1 || summary[1].Candidates != 1 {
		t.Fatalf("unexpected candidate counts: %+v", summary)
	}
}

func TestBuildMonitorWindowSummaryPreservesStatsCounts(t *testing.T) {
	stats := []pipeline.MonitorWindowStats{
		{Index: 0, Label: "primary", StartHz: 100, EndHz: 200, CenterHz: 150, SpanHz: 100, Candidates: 2, Planned: 1},
	}
	windows := []pipeline.MonitorWindow{
		{Index: 0, Label: "primary", StartHz: 100, EndHz: 200, CenterHz: 150, SpanHz: 100},
	}
	candidates := []pipeline.Candidate{
		{ID: 1, CenterHz: 150, BandwidthHz: 20},
	}
	summary := buildMonitorWindowSummary(windows, stats, candidates)
	if len(summary) != 1 {
		t.Fatalf("expected 1 window summary, got %d", len(summary))
	}
	if summary[0].Candidates != 2 {
		t.Fatalf("expected candidates to stay at 2, got %d", summary[0].Candidates)
	}
	if summary[0].Planned != 1 {
		t.Fatalf("expected planned to stay at 1, got %d", summary[0].Planned)
	}
}

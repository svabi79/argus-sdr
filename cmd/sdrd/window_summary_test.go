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

func TestBuildWindowOutcomeSummaryTracksPressureByWindowAndZone(t *testing.T) {
	windows := []pipeline.MonitorWindow{
		{Index: 0, Label: "alpha", Zone: "north", StartHz: 100, EndHz: 200},
		{Index: 1, Label: "beta", Zone: "south", StartHz: 300, EndHz: 400},
	}
	match0 := pipeline.MonitorWindowMatch{Index: 0, Label: "alpha", Zone: "north"}
	match1 := pipeline.MonitorWindowMatch{Index: 1, Label: "beta", Zone: "south"}
	workItems := []pipeline.RefinementWorkItem{
		{
			Candidate: pipeline.Candidate{ID: 1, MonitorMatches: []pipeline.MonitorWindowMatch{match0}},
			Admission: &pipeline.PriorityAdmission{Class: pipeline.AdmissionClassAdmit},
		},
		{
			Candidate: pipeline.Candidate{ID: 2, MonitorMatches: []pipeline.MonitorWindowMatch{match0}},
			Admission: &pipeline.PriorityAdmission{Class: pipeline.AdmissionClassHold},
		},
		{
			Candidate: pipeline.Candidate{ID: 3, MonitorMatches: []pipeline.MonitorWindowMatch{match1}},
			Admission: &pipeline.PriorityAdmission{Class: pipeline.AdmissionClassDisplace},
		},
		{
			Candidate: pipeline.Candidate{ID: 4, MonitorMatches: []pipeline.MonitorWindowMatch{match1}},
			Admission: &pipeline.PriorityAdmission{Class: pipeline.AdmissionClassDefer},
		},
	}
	decisions := []pipeline.SignalDecision{
		{
			Candidate:       pipeline.Candidate{ID: 1},
			ShouldRecord:    true,
			RecordWindow:    &match0,
			RecordAdmission: &pipeline.PriorityAdmission{Class: pipeline.AdmissionClassAdmit},
		},
		{
			Candidate:       pipeline.Candidate{ID: 2},
			ShouldRecord:    false,
			RecordWindow:    &match0,
			RecordAdmission: &pipeline.PriorityAdmission{Class: pipeline.AdmissionClassDisplace},
		},
		{
			Candidate:        pipeline.Candidate{ID: 3},
			ShouldAutoDecode: true,
			DecodeWindow:     &match1,
			DecodeAdmission:  &pipeline.PriorityAdmission{Class: pipeline.AdmissionClassHold},
		},
		{
			Candidate:        pipeline.Candidate{ID: 4},
			ShouldAutoDecode: false,
			DecodeWindow:     &match1,
			DecodeAdmission:  &pipeline.PriorityAdmission{Class: pipeline.AdmissionClassDefer},
		},
	}
	summary := buildWindowSummary(pipeline.RefinementPlan{MonitorWindows: windows}, nil, nil, workItems, decisions)
	if summary == nil || summary.Outcomes == nil {
		t.Fatalf("expected outcome summary to be populated")
	}
	if len(summary.Outcomes.Windows) != 2 {
		t.Fatalf("expected 2 window outcomes, got %d", len(summary.Outcomes.Windows))
	}
	win0 := summary.Outcomes.Windows[0]
	win1 := summary.Outcomes.Windows[1]
	if win0.Refinement.Admit != 1 || win0.Refinement.Hold != 1 {
		t.Fatalf("unexpected refinement outcomes for window 0: %+v", win0.Refinement)
	}
	if win0.Record.Admit != 1 || win0.Record.Displace != 1 || win0.Record.Enabled != 1 {
		t.Fatalf("unexpected record outcomes for window 0: %+v", win0.Record)
	}
	if win1.Refinement.Displace != 1 || win1.Refinement.Defer != 1 {
		t.Fatalf("unexpected refinement outcomes for window 1: %+v", win1.Refinement)
	}
	if win1.Decode.Hold != 1 || win1.Decode.Defer != 1 || win1.Decode.Enabled != 1 {
		t.Fatalf("unexpected decode outcomes for window 1: %+v", win1.Decode)
	}
	if len(summary.Outcomes.Zones) != 2 {
		t.Fatalf("expected 2 zone outcomes, got %d", len(summary.Outcomes.Zones))
	}
	for _, zone := range summary.Outcomes.Zones {
		switch zone.Zone {
		case "north":
			if zone.Refinement.Admit != 1 || zone.Refinement.Hold != 1 {
				t.Fatalf("unexpected north zone refinement: %+v", zone.Refinement)
			}
		case "south":
			if zone.Refinement.Displace != 1 || zone.Refinement.Defer != 1 {
				t.Fatalf("unexpected south zone refinement: %+v", zone.Refinement)
			}
		default:
			t.Fatalf("unexpected zone %q", zone.Zone)
		}
	}
}

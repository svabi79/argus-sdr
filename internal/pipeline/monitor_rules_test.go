package pipeline

import (
	"testing"

	"sdr-wideband-suite/internal/config"
)

func TestNormalizeMonitorWindows(t *testing.T) {
	goals := config.PipelineGoalConfig{
		MonitorWindows: []config.MonitorWindow{
			{Label: "a", StartHz: 100, EndHz: 200},
			{Label: "b", CenterHz: 500, SpanHz: 50},
		},
	}
	windows := NormalizeMonitorWindows(goals, 0)
	if len(windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(windows))
	}
	if windows[0].StartHz != 100 || windows[0].EndHz != 200 {
		t.Fatalf("unexpected first window: %+v", windows[0])
	}
	if windows[1].CenterHz != 500 || windows[1].SpanHz != 50 {
		t.Fatalf("unexpected second window: %+v", windows[1])
	}
}

func TestMonitorWindowBounds(t *testing.T) {
	windows := []MonitorWindow{
		{StartHz: 100, EndHz: 200},
		{StartHz: 50, EndHz: 90},
		{StartHz: 500, EndHz: 800},
	}
	start, end, ok := MonitorWindowBounds(windows)
	if !ok {
		t.Fatalf("expected bounds")
	}
	if start != 50 || end != 800 {
		t.Fatalf("unexpected bounds: %.0f %.0f", start, end)
	}
}

func TestCandidateInMonitorWindows(t *testing.T) {
	policy := Policy{
		MonitorWindows: []MonitorWindow{
			{StartHz: 100, EndHz: 200},
			{StartHz: 300, EndHz: 400},
		},
	}
	if !candidateInMonitor(policy, Candidate{CenterHz: 150}) {
		t.Fatalf("expected candidate inside window")
	}
	if candidateInMonitor(policy, Candidate{CenterHz: 250}) {
		t.Fatalf("expected candidate outside windows")
	}
}

func TestMonitorWindowMatchesOverlap(t *testing.T) {
	policy := Policy{
		MonitorWindows: finalizeMonitorWindows([]MonitorWindow{
			{Label: "wide", StartHz: 100, EndHz: 300, SpanHz: 200},
			{Label: "narrow", StartHz: 150, EndHz: 220, SpanHz: 70},
		}),
	}
	matches := MonitorWindowMatches(policy, Candidate{CenterHz: 180, BandwidthHz: 20})
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Index == matches[1].Index {
		t.Fatalf("expected distinct window matches")
	}
}

func TestMonitorWindowBiasPrefersNarrowWindow(t *testing.T) {
	goals := config.PipelineGoalConfig{
		MonitorWindows: []config.MonitorWindow{
			{Label: "wide", StartHz: 100, EndHz: 300},
			{Label: "narrow", StartHz: 150, EndHz: 200},
		},
	}
	policy := Policy{MonitorWindows: NormalizeMonitorWindows(goals, 0)}
	bias, detail := MonitorWindowBias(policy, Candidate{CenterHz: 175, BandwidthHz: 10})
	if detail == nil {
		t.Fatalf("expected monitor match detail")
	}
	if detail.Label != "narrow" {
		t.Fatalf("expected narrow window to be preferred, got %q", detail.Label)
	}
	if bias <= 0 {
		t.Fatalf("expected positive bias, got %.3f", bias)
	}
}

func TestMonitorWindowPriorityBiasUsesPriority(t *testing.T) {
	goals := config.PipelineGoalConfig{
		MonitorWindows: []config.MonitorWindow{
			{Label: "low", StartHz: 100, EndHz: 200, Priority: -1},
			{Label: "high", StartHz: 300, EndHz: 400, Priority: 1},
		},
	}
	policy := Policy{MonitorWindows: NormalizeMonitorWindows(goals, 0)}
	var low, high *MonitorWindow
	for i := range policy.MonitorWindows {
		win := &policy.MonitorWindows[i]
		switch win.Label {
		case "low":
			low = win
		case "high":
			high = win
		}
	}
	if low == nil || high == nil {
		t.Fatalf("expected both windows")
	}
	if low.Priority != -1 || high.Priority != 1 {
		t.Fatalf("unexpected priority values: low=%.2f high=%.2f", low.Priority, high.Priority)
	}
	if high.PriorityBias <= low.PriorityBias {
		t.Fatalf("expected high priority bias > low priority bias, got %.3f vs %.3f", high.PriorityBias, low.PriorityBias)
	}
}

func TestMonitorWindowZoneBiases(t *testing.T) {
	goals := config.PipelineGoalConfig{
		MonitorWindows: []config.MonitorWindow{
			{Label: "record", StartHz: 100, EndHz: 200, Zone: "record"},
			{Label: "decode", StartHz: 300, EndHz: 400, Zone: "decode"},
		},
	}
	policy := Policy{MonitorWindows: NormalizeMonitorWindows(goals, 0)}
	if len(policy.MonitorWindows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(policy.MonitorWindows))
	}
	var recordWin, decodeWin *MonitorWindow
	for i := range policy.MonitorWindows {
		win := &policy.MonitorWindows[i]
		switch win.Label {
		case "record":
			recordWin = win
		case "decode":
			decodeWin = win
		}
	}
	if recordWin == nil || decodeWin == nil {
		t.Fatalf("expected both window entries")
	}
	if recordWin.RecordBias <= 0 || recordWin.DecodeBias != 0 {
		t.Fatalf("unexpected record window biases: %+v", recordWin)
	}
	if decodeWin.DecodeBias <= 0 || decodeWin.RecordBias != 0 {
		t.Fatalf("unexpected decode window biases: %+v", decodeWin)
	}
	matches := MonitorWindowMatches(policy, Candidate{CenterHz: 150, BandwidthHz: 0})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].RecordBias <= 0 || matches[0].DecodeBias != 0 {
		t.Fatalf("unexpected match biases: %+v", matches[0])
	}
}

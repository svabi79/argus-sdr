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

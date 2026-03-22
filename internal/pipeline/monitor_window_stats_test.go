package pipeline

import "testing"

func TestMonitorWindowStatsAttribution(t *testing.T) {
	policy := Policy{
		MonitorWindows: finalizeMonitorWindows([]MonitorWindow{
			{Label: "wide", StartHz: 100, EndHz: 300, SpanHz: 200},
			{Label: "narrow", StartHz: 150, EndHz: 250, SpanHz: 100},
		}),
		MinCandidateSNRDb: 5,
		MaxRefinementJobs: 5,
	}
	candidates := []Candidate{
		{ID: 1, CenterHz: 160, BandwidthHz: 10, SNRDb: 8},
		{ID: 2, CenterHz: 260, BandwidthHz: 10, SNRDb: 2},
		{ID: 3, CenterHz: 500, BandwidthHz: 10, SNRDb: 12},
	}
	plan := BuildRefinementPlan(candidates, policy)
	if plan.DroppedByMonitor != 1 {
		t.Fatalf("expected 1 dropped by monitor, got %d", plan.DroppedByMonitor)
	}
	if len(plan.MonitorWindowStats) != 2 {
		t.Fatalf("expected 2 window stats, got %d", len(plan.MonitorWindowStats))
	}
	var wide, narrow *MonitorWindowStats
	for i := range plan.MonitorWindowStats {
		stat := &plan.MonitorWindowStats[i]
		switch stat.Label {
		case "wide":
			wide = stat
		case "narrow":
			narrow = stat
		}
	}
	if wide == nil || narrow == nil {
		t.Fatalf("expected both window stats to be present")
	}
	if wide.Candidates != 2 || wide.Planned != 1 || wide.Dropped != 1 {
		t.Fatalf("unexpected wide stats: %+v", *wide)
	}
	if narrow.Candidates != 1 || narrow.Planned != 1 || narrow.Dropped != 0 {
		t.Fatalf("unexpected narrow stats: %+v", *narrow)
	}
}

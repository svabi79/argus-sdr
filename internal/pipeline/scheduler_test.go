package pipeline

import "testing"

func TestScheduleCandidates(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 2, MinCandidateSNRDb: 5}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, SNRDb: 4, BandwidthHz: 10000, PeakDb: 1},
		{ID: 2, CenterHz: 200, SNRDb: 12, BandwidthHz: 50000, PeakDb: 3},
		{ID: 3, CenterHz: 300, SNRDb: 10, BandwidthHz: 25000, PeakDb: 2},
		{ID: 4, CenterHz: 400, SNRDb: 20, BandwidthHz: 100000, PeakDb: 5},
	}
	got := ScheduleCandidates(cands, policy)
	if len(got) != 2 {
		t.Fatalf("expected 2 scheduled candidates, got %d", len(got))
	}
	if got[0].Candidate.ID != 4 {
		t.Fatalf("expected strongest candidate first, got id=%d", got[0].Candidate.ID)
	}
	if got[1].Candidate.ID != 2 {
		t.Fatalf("expected next strongest candidate second, got id=%d", got[1].Candidate.ID)
	}
}

func TestBuildRefinementPlanTracksDrops(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 1, MinCandidateSNRDb: 10}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, SNRDb: 4, BandwidthHz: 10000, PeakDb: 1},
		{ID: 2, CenterHz: 200, SNRDb: 12, BandwidthHz: 50000, PeakDb: 3},
		{ID: 3, CenterHz: 300, SNRDb: 11, BandwidthHz: 25000, PeakDb: 2},
	}
	plan := BuildRefinementPlan(cands, policy)
	if plan.TotalCandidates != 3 {
		t.Fatalf("expected total candidates 3, got %d", plan.TotalCandidates)
	}
	if plan.DroppedBySNR != 1 {
		t.Fatalf("expected 1 dropped by SNR, got %d", plan.DroppedBySNR)
	}
	if plan.DroppedByBudget != 1 {
		t.Fatalf("expected 1 dropped by budget, got %d", plan.DroppedByBudget)
	}
	if len(plan.Selected) != 1 || plan.Selected[0].Candidate.ID != 2 {
		t.Fatalf("unexpected plan selection: %+v", plan.Selected)
	}
}

func TestScheduleCandidatesPriorityBoost(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 1, MinCandidateSNRDb: 0, SignalPriorities: []string{"digital"}}
	got := ScheduleCandidates([]Candidate{
		{ID: 1, SNRDb: 15, Hint: "voice"},
		{ID: 2, SNRDb: 14, Hint: "digital-burst"},
	}, policy)
	if len(got) != 1 || got[0].Candidate.ID != 2 {
		t.Fatalf("expected priority boost to favor digital candidate, got %+v", got)
	}
}

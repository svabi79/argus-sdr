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

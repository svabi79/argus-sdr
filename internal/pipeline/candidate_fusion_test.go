package pipeline

import "testing"

func TestFuseCandidatesDedup(t *testing.T) {
	primary := []Candidate{
		{
			ID:          1,
			CenterHz:    100.0e6,
			BandwidthHz: 20000,
			Evidence: []LevelEvidence{
				{Level: AnalysisLevel{Name: "surveillance"}},
			},
		},
	}
	derived := []Candidate{
		{
			ID:          -1,
			CenterHz:    100.0e6 + 2000,
			BandwidthHz: 25000,
			Evidence: []LevelEvidence{
				{Level: AnalysisLevel{Name: "surveillance-lowres"}},
			},
		},
	}
	fused := FuseCandidates(primary, derived)
	if len(fused) != 1 {
		t.Fatalf("expected 1 fused candidate, got %d", len(fused))
	}
	if got := CandidateEvidenceLevelCount(fused[0]); got != 2 {
		t.Fatalf("expected 2 evidence levels after fuse, got %d", got)
	}
	if fused[0].EvidenceState == nil || !fused[0].EvidenceState.Fused || !fused[0].EvidenceState.MultiLevelConfirmed {
		t.Fatalf("expected fused multi-level evidence state, got %+v", fused[0].EvidenceState)
	}
}

func TestFuseCandidatesSingleVsMultiResolution(t *testing.T) {
	primary := []Candidate{
		{
			ID:          1,
			CenterHz:    101.0e6,
			BandwidthHz: 12000,
			Evidence: []LevelEvidence{
				{Level: AnalysisLevel{Name: "surveillance"}},
			},
		},
	}
	single := FuseCandidates(primary, nil)
	if len(single) != 1 {
		t.Fatalf("expected single-resolution to keep 1 candidate, got %d", len(single))
	}
	derived := []Candidate{
		{
			ID:          -2,
			CenterHz:    101.3e6,
			BandwidthHz: 15000,
			Evidence: []LevelEvidence{
				{Level: AnalysisLevel{Name: "surveillance-lowres"}},
			},
		},
	}
	multi := FuseCandidates(primary, derived)
	if len(multi) != 2 {
		t.Fatalf("expected multi-resolution to keep 2 candidates, got %d", len(multi))
	}
}

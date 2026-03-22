package pipeline

import (
	"testing"
	"time"
)

func TestHoldPolicyArchiveBiasesRecord(t *testing.T) {
	policy := Policy{DecisionHoldMs: 1000, Profile: "archive", RefinementStrategy: "archive-oriented"}
	hold := HoldPolicyFromPolicy(policy)
	if hold.RecordMs <= hold.BaseMs {
		t.Fatalf("expected archive profile to extend record hold, got %d vs %d", hold.RecordMs, hold.BaseMs)
	}
	if hold.RefinementMs <= hold.BaseMs {
		t.Fatalf("expected archive profile to extend refinement hold, got %d vs %d", hold.RefinementMs, hold.BaseMs)
	}
	if !containsReason(hold.Reasons, HoldReasonProfileArchive) {
		t.Fatalf("expected profile archive reason, got %+v", hold.Reasons)
	}
}

func TestHoldPolicyDigitalBiasesDecode(t *testing.T) {
	policy := Policy{DecisionHoldMs: 1000, Profile: "digital-hunting", RefinementStrategy: "digital-hunting"}
	hold := HoldPolicyFromPolicy(policy)
	if hold.DecodeMs <= hold.RecordMs {
		t.Fatalf("expected digital profile to favor decode hold, got decode=%d record=%d", hold.DecodeMs, hold.RecordMs)
	}
	if !containsReason(hold.Reasons, HoldReasonProfileDigital) {
		t.Fatalf("expected profile digital reason, got %+v", hold.Reasons)
	}
}

func TestAdmitRefinementPlanNoCandidatesReason(t *testing.T) {
	res := AdmitRefinementPlan(RefinementPlan{}, Policy{}, time.Now(), &RefinementHold{Active: map[int64]time.Time{}})
	if res.Admission.Reason != ReasonAdmissionNoCandidates {
		t.Fatalf("expected no-candidates reason, got %s", res.Admission.Reason)
	}
}

func containsReason(reasons []string, target string) bool {
	for _, r := range reasons {
		if r == target {
			return true
		}
	}
	return false
}

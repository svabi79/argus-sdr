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

func TestHoldPolicyIntentOverrides(t *testing.T) {
	policy := Policy{DecisionHoldMs: 1000, Intent: "archive-and-triage"}
	hold := HoldPolicyFromPolicy(policy)
	if hold.RecordMs <= hold.BaseMs {
		t.Fatalf("expected archive intent to extend record hold, got %d vs %d", hold.RecordMs, hold.BaseMs)
	}
	if !containsReason(hold.Reasons, HoldReasonIntentArchive) {
		t.Fatalf("expected intent archive reason, got %+v", hold.Reasons)
	}

	policy = Policy{DecisionHoldMs: 1000, Intent: "decode-digital"}
	hold = HoldPolicyFromPolicy(policy)
	if hold.DecodeMs <= hold.BaseMs {
		t.Fatalf("expected decode intent to extend decode hold, got %d vs %d", hold.DecodeMs, hold.BaseMs)
	}
	if !containsReason(hold.Reasons, HoldReasonIntentDecode) {
		t.Fatalf("expected intent decode reason, got %+v", hold.Reasons)
	}
}

func TestAdmitRefinementPlanNoCandidatesReason(t *testing.T) {
	res := AdmitRefinementPlan(RefinementPlan{}, Policy{}, time.Now(), &RefinementHold{Active: map[int64]time.Time{}})
	if res.Admission.Reason != ReasonAdmissionNoCandidates {
		t.Fatalf("expected no-candidates reason, got %s", res.Admission.Reason)
	}
}

func TestAdmitRefinementPlanUnlimitedBudget(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 0, MinCandidateSNRDb: 0}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, SNRDb: 5},
		{ID: 2, CenterHz: 200, SNRDb: 6},
	}
	plan := BuildRefinementPlan(cands, policy)
	res := AdmitRefinementPlan(plan, policy, time.Now(), &RefinementHold{Active: map[int64]time.Time{}})
	if len(res.Plan.Selected) != len(cands) {
		t.Fatalf("expected all candidates admitted, got %d", len(res.Plan.Selected))
	}
	if res.Admission.Skipped != 0 || res.Plan.DroppedByBudget != 0 {
		t.Fatalf("expected no budget drops, got admission=%+v plan=%+v", res.Admission, res.Plan)
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

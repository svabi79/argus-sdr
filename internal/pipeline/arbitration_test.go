package pipeline

import "testing"

func TestHoldPolicyArchiveBiasesRecord(t *testing.T) {
	policy := Policy{DecisionHoldMs: 1000, Profile: "archive", RefinementStrategy: "archive-oriented"}
	hold := HoldPolicyFromPolicy(policy)
	if hold.RecordMs <= hold.BaseMs {
		t.Fatalf("expected archive profile to extend record hold, got %d vs %d", hold.RecordMs, hold.BaseMs)
	}
	if hold.RefinementMs <= hold.BaseMs {
		t.Fatalf("expected archive profile to extend refinement hold, got %d vs %d", hold.RefinementMs, hold.BaseMs)
	}
}

func TestHoldPolicyDigitalBiasesDecode(t *testing.T) {
	policy := Policy{DecisionHoldMs: 1000, Profile: "digital-hunting", RefinementStrategy: "digital-hunting"}
	hold := HoldPolicyFromPolicy(policy)
	if hold.DecodeMs <= hold.RecordMs {
		t.Fatalf("expected digital profile to favor decode hold, got decode=%d record=%d", hold.DecodeMs, hold.RecordMs)
	}
}

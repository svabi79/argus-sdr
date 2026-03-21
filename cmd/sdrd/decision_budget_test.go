package main

import (
	"testing"

	"sdr-wideband-suite/internal/pipeline"
)

func TestEnforceDecisionBudgets(t *testing.T) {
	decisions := []pipeline.SignalDecision{
		{ShouldRecord: true, ShouldAutoDecode: true},
		{ShouldRecord: true, ShouldAutoDecode: true},
		{ShouldRecord: true, ShouldAutoDecode: false},
	}
	recorded, decoded := enforceDecisionBudgets(decisions, 1, 1)
	if recorded != 1 || decoded != 1 {
		t.Fatalf("unexpected counts: record=%d decode=%d", recorded, decoded)
	}
	if decisions[0].ShouldRecord != true || decisions[0].ShouldAutoDecode != true {
		t.Fatalf("expected first decision to remain allowed")
	}
	if decisions[1].ShouldRecord || decisions[1].ShouldAutoDecode {
		t.Fatalf("expected second decision to be budgeted off")
	}
	if decisions[2].ShouldRecord {
		t.Fatalf("expected third decision to be budgeted off")
	}
}

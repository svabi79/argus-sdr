package main

import (
	"testing"

	"sdr-wideband-suite/internal/pipeline"
)

func TestEnforceDecisionBudgets(t *testing.T) {
	decisions := []pipeline.SignalDecision{
		{Candidate: pipeline.Candidate{SNRDb: 5}, ShouldRecord: true, ShouldAutoDecode: true},
		{Candidate: pipeline.Candidate{SNRDb: 15}, ShouldRecord: true, ShouldAutoDecode: true},
		{Candidate: pipeline.Candidate{SNRDb: 10}, ShouldRecord: true, ShouldAutoDecode: false},
	}
	recorded, decoded := enforceDecisionBudgets(decisions, 1, 1)
	if recorded != 1 || decoded != 1 {
		t.Fatalf("unexpected counts: record=%d decode=%d", recorded, decoded)
	}
	if !decisions[1].ShouldRecord || !decisions[1].ShouldAutoDecode {
		t.Fatalf("expected highest SNR decision to remain allowed")
	}
	if decisions[0].ShouldRecord || decisions[0].ShouldAutoDecode {
		t.Fatalf("expected lowest SNR decision to be budgeted off")
	}
	if decisions[2].ShouldRecord {
		t.Fatalf("expected mid SNR decision to be budgeted off by record budget")
	}
}

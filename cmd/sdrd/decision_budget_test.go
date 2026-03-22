package main

import (
	"testing"
	"time"

	"sdr-wideband-suite/internal/pipeline"
)

func TestEnforceDecisionBudgets(t *testing.T) {
	decisions := []pipeline.SignalDecision{
		{Candidate: pipeline.Candidate{ID: 1, SNRDb: 5}, ShouldRecord: true, ShouldAutoDecode: true},
		{Candidate: pipeline.Candidate{ID: 2, SNRDb: 15}, ShouldRecord: true, ShouldAutoDecode: true},
		{Candidate: pipeline.Candidate{ID: 3, SNRDb: 10}, ShouldRecord: true, ShouldAutoDecode: false},
	}
	q := newDecisionQueues()
	policy := pipeline.Policy{SignalPriorities: []string{"digital"}, MaxRecordingStreams: 1, MaxDecodeJobs: 1}
	budget := pipeline.BudgetModelFromPolicy(policy)
	stats := q.Apply(decisions, budget, time.Now(), policy)
	if stats.RecordSelected != 1 || stats.DecodeSelected != 1 {
		t.Fatalf("unexpected counts: record=%d decode=%d", stats.RecordSelected, stats.DecodeSelected)
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

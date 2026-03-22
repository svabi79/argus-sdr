package pipeline

import (
	"testing"
	"time"
)

func TestDecisionQueueDropsByBudget(t *testing.T) {
	arbiter := NewArbiter()
	decisions := []SignalDecision{
		{Candidate: Candidate{ID: 1, SNRDb: 12}, ShouldRecord: true, ShouldAutoDecode: true},
		{Candidate: Candidate{ID: 2, SNRDb: 10}, ShouldRecord: true, ShouldAutoDecode: true},
	}
	budget := BudgetModel{
		Record: BudgetQueue{Max: 1},
		Decode: BudgetQueue{Max: 1},
	}
	stats := arbiter.ApplyDecisions(decisions, budget, time.Now(), Policy{DecisionHoldMs: 250})
	if stats.RecordDropped == 0 || stats.DecodeDropped == 0 {
		t.Fatalf("expected drops by budget, got %+v", stats)
	}
	allowed := 0
	for _, d := range decisions {
		if d.ShouldRecord || d.ShouldAutoDecode {
			allowed++
			continue
		}
		if d.Reason != DecisionReasonQueueRecord && d.Reason != DecisionReasonQueueDecode {
			t.Fatalf("unexpected decision reason: %s", d.Reason)
		}
	}
	if allowed != 1 {
		t.Fatalf("expected 1 decision allowed, got %d", allowed)
	}
}

func TestDecisionQueueEnforcesBudgets(t *testing.T) {
	decisions := []SignalDecision{
		{Candidate: Candidate{ID: 1, SNRDb: 5}, ShouldRecord: true, ShouldAutoDecode: true},
		{Candidate: Candidate{ID: 2, SNRDb: 15}, ShouldRecord: true, ShouldAutoDecode: true},
		{Candidate: Candidate{ID: 3, SNRDb: 10}, ShouldRecord: true, ShouldAutoDecode: false},
	}
	arbiter := NewArbiter()
	policy := Policy{SignalPriorities: []string{"digital"}, MaxRecordingStreams: 1, MaxDecodeJobs: 1}
	budget := BudgetModelFromPolicy(policy)
	stats := arbiter.ApplyDecisions(decisions, budget, time.Now(), policy)
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

func TestDecisionQueueHoldKeepsSelection(t *testing.T) {
	arbiter := NewArbiter()
	policy := Policy{DecisionHoldMs: 500}
	budget := BudgetModel{Record: BudgetQueue{Max: 1}, Decode: BudgetQueue{Max: 1}}
	now := time.Now()

	decisions := []SignalDecision{
		{Candidate: Candidate{ID: 1, SNRDb: 5}, ShouldRecord: true, ShouldAutoDecode: true},
		{Candidate: Candidate{ID: 2, SNRDb: 15}, ShouldRecord: true, ShouldAutoDecode: true},
	}
	arbiter.ApplyDecisions(decisions, budget, now, policy)
	if !decisions[1].ShouldRecord || !decisions[1].ShouldAutoDecode {
		t.Fatalf("expected candidate 2 to be selected initially")
	}

	decisions = []SignalDecision{
		{Candidate: Candidate{ID: 1, SNRDb: 25}, ShouldRecord: true, ShouldAutoDecode: true},
		{Candidate: Candidate{ID: 2, SNRDb: 2}, ShouldRecord: true, ShouldAutoDecode: true},
	}
	arbiter.ApplyDecisions(decisions, budget, now.Add(100*time.Millisecond), policy)
	if !decisions[1].ShouldRecord || !decisions[1].ShouldAutoDecode {
		t.Fatalf("expected held candidate 2 to remain selected")
	}
	if decisions[0].ShouldRecord || decisions[0].ShouldAutoDecode {
		t.Fatalf("expected candidate 1 to remain queued behind hold")
	}
}

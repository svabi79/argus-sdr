package pipeline

import (
	"strings"
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
		if !strings.HasPrefix(d.Reason, DecisionReasonQueueRecord) && !strings.HasPrefix(d.Reason, DecisionReasonQueueDecode) {
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
	if decisions[1].RecordAdmission == nil || decisions[1].RecordAdmission.Class != AdmissionClassAdmit {
		t.Fatalf("expected admitted record admission, got %+v", decisions[1].RecordAdmission)
	}
	if decisions[0].RecordAdmission == nil || decisions[0].RecordAdmission.Class != AdmissionClassDefer {
		t.Fatalf("expected deferred record admission, got %+v", decisions[0].RecordAdmission)
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
		{Candidate: Candidate{ID: 1, SNRDb: 32}, ShouldRecord: true, ShouldAutoDecode: true},
		{Candidate: Candidate{ID: 2, SNRDb: 30}, ShouldRecord: true, ShouldAutoDecode: true},
		{Candidate: Candidate{ID: 3, SNRDb: 10}, ShouldRecord: true, ShouldAutoDecode: true},
	}
	stats := arbiter.ApplyDecisions(decisions, budget, now.Add(100*time.Millisecond), policy)
	if !decisions[1].ShouldRecord || !decisions[1].ShouldAutoDecode {
		t.Fatalf("expected held candidate 2 to remain selected")
	}
	if decisions[0].ShouldRecord || decisions[0].ShouldAutoDecode {
		t.Fatalf("expected candidate 1 to remain queued behind hold")
	}
	if decisions[1].RecordAdmission == nil || decisions[1].RecordAdmission.Class != AdmissionClassHold {
		t.Fatalf("expected record admission hold class, got %+v", decisions[1].RecordAdmission)
	}
	if stats.DecisionHoldMs != policy.DecisionHoldMs {
		t.Fatalf("expected decision hold ms %d, got %d", policy.DecisionHoldMs, stats.DecisionHoldMs)
	}
	if stats.RecordDisplacedByHold != 1 || stats.RecordDisplaced != 1 {
		t.Fatalf("expected displaced-by-hold count 1, got %+v", stats)
	}
}

func TestDecisionQueueHighTierHoldProtected(t *testing.T) {
	arbiter := NewArbiter()
	policy := Policy{DecisionHoldMs: 500}
	budget := BudgetModel{Record: BudgetQueue{Max: 1}}
	now := time.Now()

	decisions := []SignalDecision{
		{Candidate: Candidate{ID: 1, SNRDb: 30}, ShouldRecord: true},
		{Candidate: Candidate{ID: 2, SNRDb: 10}, ShouldRecord: true},
	}
	arbiter.ApplyDecisions(decisions, budget, now, policy)
	if !decisions[0].ShouldRecord {
		t.Fatalf("expected candidate 1 to be selected initially")
	}

	decisions = []SignalDecision{
		{Candidate: Candidate{ID: 1, SNRDb: 30}, ShouldRecord: true},
		{Candidate: Candidate{ID: 2, SNRDb: 10}, ShouldRecord: true},
		{Candidate: Candidate{ID: 3, SNRDb: 32}, ShouldRecord: true},
	}
	arbiter.ApplyDecisions(decisions, budget, now.Add(100*time.Millisecond), policy)
	if !decisions[0].ShouldRecord {
		t.Fatalf("expected protected hold to keep candidate 1")
	}
	if decisions[2].ShouldRecord {
		t.Fatalf("expected candidate 3 to remain deferred behind protected hold")
	}
	if decisions[0].RecordAdmission == nil || decisions[0].RecordAdmission.Class != AdmissionClassHold {
		t.Fatalf("expected hold admission for candidate 1, got %+v", decisions[0].RecordAdmission)
	}
	if decisions[2].RecordAdmission == nil || decisions[2].RecordAdmission.Class != AdmissionClassDisplace {
		t.Fatalf("expected displacement admission for candidate 3, got %+v", decisions[2].RecordAdmission)
	}
}

func TestDecisionQueueOpportunisticDisplacement(t *testing.T) {
	arbiter := NewArbiter()
	policy := Policy{DecisionHoldMs: 500}
	budget := BudgetModel{Record: BudgetQueue{Max: 1}}
	now := time.Now()

	decisions := []SignalDecision{
		{Candidate: Candidate{ID: 1, SNRDb: 15}, ShouldRecord: true},
		{Candidate: Candidate{ID: 2, SNRDb: 10}, ShouldRecord: true},
	}
	arbiter.ApplyDecisions(decisions, budget, now, policy)
	if !decisions[0].ShouldRecord {
		t.Fatalf("expected candidate 1 to be selected initially")
	}

	decisions = []SignalDecision{
		{Candidate: Candidate{ID: 1, SNRDb: 5}, ShouldRecord: true},
		{Candidate: Candidate{ID: 2, SNRDb: 4}, ShouldRecord: true},
		{Candidate: Candidate{ID: 3, SNRDb: 30}, ShouldRecord: true},
	}
	arbiter.ApplyDecisions(decisions, budget, now.Add(100*time.Millisecond), policy)
	if decisions[0].ShouldRecord {
		t.Fatalf("expected candidate 1 to be displaced")
	}
	if !decisions[2].ShouldRecord {
		t.Fatalf("expected candidate 3 to opportunistically displace hold")
	}
	if decisions[0].RecordAdmission == nil || decisions[0].RecordAdmission.Class != AdmissionClassDisplace {
		t.Fatalf("expected displacement admission for candidate 1, got %+v", decisions[0].RecordAdmission)
	}
	if decisions[2].RecordAdmission == nil || decisions[2].RecordAdmission.Class != AdmissionClassAdmit {
		t.Fatalf("expected admit admission for candidate 3, got %+v", decisions[2].RecordAdmission)
	}
	if decisions[2].RecordAdmission == nil || !strings.Contains(decisions[2].RecordAdmission.Reason, ReasonTagDisplaceOpportunist) {
		t.Fatalf("expected opportunistic displacement reason, got %+v", decisions[2].RecordAdmission)
	}
}

func TestDecisionQueueHoldExpiryChurn(t *testing.T) {
	arbiter := NewArbiter()
	policy := Policy{DecisionHoldMs: 100}
	budget := BudgetModel{Record: BudgetQueue{Max: 1}}
	now := time.Now()

	decisions := []SignalDecision{
		{Candidate: Candidate{ID: 1, SNRDb: 12}, ShouldRecord: true},
		{Candidate: Candidate{ID: 2, SNRDb: 10}, ShouldRecord: true},
	}
	arbiter.ApplyDecisions(decisions, budget, now, policy)
	if !decisions[0].ShouldRecord {
		t.Fatalf("expected candidate 1 to be selected initially")
	}

	decisions = []SignalDecision{
		{Candidate: Candidate{ID: 1, SNRDb: 30}, ShouldRecord: true},
		{Candidate: Candidate{ID: 2, SNRDb: 32}, ShouldRecord: true},
		{Candidate: Candidate{ID: 3, SNRDb: 5}, ShouldRecord: true},
	}
	arbiter.ApplyDecisions(decisions, budget, now.Add(50*time.Millisecond), policy)
	if !decisions[0].ShouldRecord {
		t.Fatalf("expected hold to keep candidate 1 before expiry")
	}

	decisions = []SignalDecision{
		{Candidate: Candidate{ID: 1, SNRDb: 30}, ShouldRecord: true},
		{Candidate: Candidate{ID: 2, SNRDb: 32}, ShouldRecord: true},
		{Candidate: Candidate{ID: 3, SNRDb: 5}, ShouldRecord: true},
	}
	arbiter.ApplyDecisions(decisions, budget, now.Add(200*time.Millisecond), policy)
	if decisions[0].ShouldRecord {
		t.Fatalf("expected candidate 1 to be released after hold expiry")
	}
	if !decisions[1].ShouldRecord {
		t.Fatalf("expected candidate 2 to be selected after hold expiry")
	}
	if decisions[0].RecordAdmission == nil || !strings.Contains(decisions[0].RecordAdmission.Reason, ReasonTagHoldExpired) {
		t.Fatalf("expected hold expiry reason, got %+v", decisions[0].RecordAdmission)
	}
}

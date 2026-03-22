package pipeline

import (
	"strings"
	"testing"
	"time"
)

func TestScheduleCandidates(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 2, MinCandidateSNRDb: 5}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, SNRDb: 4, BandwidthHz: 10000, PeakDb: 1},
		{ID: 2, CenterHz: 200, SNRDb: 12, BandwidthHz: 50000, PeakDb: 3},
		{ID: 3, CenterHz: 300, SNRDb: 10, BandwidthHz: 25000, PeakDb: 2},
		{ID: 4, CenterHz: 400, SNRDb: 20, BandwidthHz: 100000, PeakDb: 5},
	}
	got := ScheduleCandidates(cands, policy)
	if len(got) != 3 {
		t.Fatalf("expected 3 scheduled candidates, got %d", len(got))
	}
	if got[0].Candidate.ID != 4 {
		t.Fatalf("expected strongest candidate first, got id=%d", got[0].Candidate.ID)
	}
	if got[1].Candidate.ID != 2 {
		t.Fatalf("expected next strongest candidate second, got id=%d", got[1].Candidate.ID)
	}
}

func TestBuildRefinementPlanTracksDrops(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 1, MinCandidateSNRDb: 10}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, SNRDb: 4, BandwidthHz: 10000, PeakDb: 1},
		{ID: 2, CenterHz: 200, SNRDb: 12, BandwidthHz: 50000, PeakDb: 3},
		{ID: 3, CenterHz: 300, SNRDb: 11, BandwidthHz: 25000, PeakDb: 2},
	}
	plan := BuildRefinementPlan(cands, policy)
	if plan.TotalCandidates != 3 {
		t.Fatalf("expected total candidates 3, got %d", plan.TotalCandidates)
	}
	if plan.DroppedBySNR != 1 {
		t.Fatalf("expected 1 dropped by SNR, got %d", plan.DroppedBySNR)
	}
	if plan.DroppedByBudget != 0 {
		t.Fatalf("expected 0 dropped by budget in plan stage, got %d", plan.DroppedByBudget)
	}
	if len(plan.Selected) != 0 {
		t.Fatalf("expected no admitted selection in plan stage, got %+v", plan.Selected)
	}
	if len(plan.Ranked) != 2 {
		t.Fatalf("expected ranked candidates after gating, got %d", len(plan.Ranked))
	}
	if len(plan.WorkItems) != len(cands) {
		t.Fatalf("expected work items for all candidates, got %d", len(plan.WorkItems))
	}
	item2 := findWorkItem(plan.WorkItems, 2)
	if item2 == nil || item2.Status != RefinementStatusPlanned || item2.Reason != RefinementReasonPlanned {
		t.Fatalf("expected candidate 2 planned with reason, got %+v", item2)
	}
	item1 := findWorkItem(plan.WorkItems, 1)
	if item1 == nil || item1.Reason != RefinementReasonBelowSNR {
		t.Fatalf("expected candidate 1 dropped by snr, got %+v", item1)
	}
	item3 := findWorkItem(plan.WorkItems, 3)
	if item3 == nil || item3.Status != RefinementStatusPlanned {
		t.Fatalf("expected candidate 3 planned pre-admission, got %+v", item3)
	}
}

func TestBuildRefinementPlanRespectsMaxConcurrent(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 5, RefinementMaxConcurrent: 2, MinCandidateSNRDb: 0}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, SNRDb: 9},
		{ID: 2, CenterHz: 200, SNRDb: 8},
		{ID: 3, CenterHz: 300, SNRDb: 7},
	}
	plan := BuildRefinementPlan(cands, policy)
	if plan.Budget != 2 {
		t.Fatalf("expected budget 2, got %d", plan.Budget)
	}
	if plan.BudgetSource != "refinement.max_concurrent" {
		t.Fatalf("expected budget source refinement.max_concurrent, got %s", plan.BudgetSource)
	}
	if len(plan.Selected) != 0 {
		t.Fatalf("expected no selected until admission, got %d", len(plan.Selected))
	}
}

func TestBuildRefinementPlanAppliesMonitorSpan(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 5, MinCandidateSNRDb: 0, MonitorStartHz: 150, MonitorEndHz: 350}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, BandwidthHz: 20},
		{ID: 2, CenterHz: 200, BandwidthHz: 50},
		{ID: 3, CenterHz: 300, BandwidthHz: 100},
		{ID: 4, CenterHz: 500, BandwidthHz: 50},
	}
	plan := BuildRefinementPlan(cands, policy)
	if plan.DroppedByMonitor != 2 {
		t.Fatalf("expected 2 dropped by monitor, got %d", plan.DroppedByMonitor)
	}
	if len(plan.Ranked) != 2 {
		t.Fatalf("expected 2 ranked within monitor, got %d", len(plan.Ranked))
	}
}

func TestBuildRefinementPlanAppliesMonitorSpanCentered(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 5, MinCandidateSNRDb: 0, MonitorCenterHz: 300, MonitorSpanHz: 200}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, BandwidthHz: 20},
		{ID: 2, CenterHz: 250, BandwidthHz: 50},
		{ID: 3, CenterHz: 300, BandwidthHz: 100},
		{ID: 4, CenterHz: 420, BandwidthHz: 50},
	}
	plan := BuildRefinementPlan(cands, policy)
	if plan.DroppedByMonitor != 1 {
		t.Fatalf("expected 1 dropped by monitor, got %d", plan.DroppedByMonitor)
	}
	if len(plan.Ranked) != 3 {
		t.Fatalf("expected 3 ranked within monitor, got %d", len(plan.Ranked))
	}
}

func TestAutoSpanForHint(t *testing.T) {
	span, source := AutoSpanForHint("WFM_STEREO")
	if span < 150000 || source == "" {
		t.Fatalf("expected WFM span, got %.0f (%s)", span, source)
	}
	span, source = AutoSpanForHint("CW")
	if span != 500 || source == "" {
		t.Fatalf("expected CW span, got %.0f (%s)", span, source)
	}
	span, source = AutoSpanForHint("")
	if span != 0 || source != "" {
		t.Fatalf("expected empty span for unknown hint, got %.0f (%s)", span, source)
	}
}

func TestScheduleCandidatesPriorityBoost(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 1, MinCandidateSNRDb: 0, SignalPriorities: []string{"digital"}}
	got := ScheduleCandidates([]Candidate{
		{ID: 1, SNRDb: 15, Hint: "voice"},
		{ID: 2, SNRDb: 14, Hint: "digital-burst"},
	}, policy)
	if len(got) != 2 || got[0].Candidate.ID != 2 {
		t.Fatalf("expected priority boost to favor digital candidate, got %+v", got)
	}
}

func TestScheduleCandidatesFamilyTierFloor(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 2, MinCandidateSNRDb: 0, SignalPriorities: []string{"digital", "wfm"}}
	cands := []Candidate{
		{ID: 1, SNRDb: 1, Hint: "digital-burst"},
		{ID: 2, SNRDb: 20, Hint: "voice"},
	}
	plan := BuildRefinementPlan(cands, policy)
	item := findScheduled(plan.Ranked, 1)
	if item == nil {
		t.Fatalf("expected ranked candidate 1")
	}
	if item.Family != "digital" || item.FamilyRank != 1 {
		t.Fatalf("expected digital family rank 1, got family=%s rank=%d", item.Family, item.FamilyRank)
	}
	if item.TierFloor != PriorityTierHigh {
		t.Fatalf("expected tier floor high, got %s", item.TierFloor)
	}
	if priorityTierRank(item.Tier) < priorityTierRank(PriorityTierHigh) {
		t.Fatalf("expected tier to be raised by family floor, got %s", item.Tier)
	}
}

func TestScheduleCandidatesEvidenceBoost(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 2, MinCandidateSNRDb: 0}
	single := Candidate{
		ID:          1,
		SNRDb:       8,
		BandwidthHz: 12000,
		Evidence: []LevelEvidence{
			{Level: AnalysisLevel{Name: "surveillance"}},
		},
	}
	multi := Candidate{
		ID:          2,
		SNRDb:       8,
		BandwidthHz: 12000,
		Evidence: []LevelEvidence{
			{Level: AnalysisLevel{Name: "surveillance"}},
			{Level: AnalysisLevel{Name: "surveillance-lowres"}},
		},
	}
	plan := BuildRefinementPlan([]Candidate{single, multi}, policy)
	if len(plan.Ranked) < 2 {
		t.Fatalf("expected ranked candidates, got %d", len(plan.Ranked))
	}
	if plan.Ranked[0].Candidate.ID != multi.ID {
		t.Fatalf("expected multi-level candidate to rank first, got %+v", plan.Ranked[0])
	}
	if plan.Ranked[0].Breakdown == nil || plan.Ranked[0].Breakdown.EvidenceScore <= 0 {
		t.Fatalf("expected evidence score to be populated, got %+v", plan.Ranked[0].Breakdown)
	}
	if plan.Ranked[0].Breakdown.EvidenceDetail == nil || !plan.Ranked[0].Breakdown.EvidenceDetail.MultiLevelConfirmed {
		t.Fatalf("expected evidence detail for multi-level candidate, got %+v", plan.Ranked[0].Breakdown)
	}
}

func TestScheduleCandidatesDerivedOnlyPenalty(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 2, MinCandidateSNRDb: 0}
	primary := Candidate{
		ID:          1,
		SNRDb:       10,
		BandwidthHz: 12000,
		Evidence: []LevelEvidence{
			{Level: AnalysisLevel{Name: "surveillance", Role: RoleSurveillancePrimary, Truth: "surveillance"}},
		},
	}
	derived := Candidate{
		ID:          2,
		SNRDb:       10,
		BandwidthHz: 12000,
		Evidence: []LevelEvidence{
			{Level: AnalysisLevel{Name: "surveillance-lowres", Role: RoleSurveillanceDerived, Truth: "surveillance"}},
		},
	}
	plan := BuildRefinementPlan([]Candidate{derived, primary}, policy)
	if len(plan.Ranked) != 2 {
		t.Fatalf("expected ranked candidates, got %d", len(plan.Ranked))
	}
	if plan.Ranked[0].Candidate.ID != primary.ID {
		t.Fatalf("expected primary evidence to outrank derived-only, got %+v", plan.Ranked[0])
	}
}

func TestScheduleCandidatesDerivedOnlyStrategyBias(t *testing.T) {
	cand := Candidate{
		ID:          1,
		SNRDb:       9,
		BandwidthHz: 12000,
		Evidence: []LevelEvidence{
			{Level: AnalysisLevel{Name: "surveillance-lowres", Role: RoleSurveillanceDerived, Truth: "surveillance"}},
		},
	}
	singlePlan := BuildRefinementPlan([]Candidate{cand}, Policy{MinCandidateSNRDb: 0})
	multiPlan := BuildRefinementPlan([]Candidate{cand}, Policy{MinCandidateSNRDb: 0, SurveillanceStrategy: "multi-resolution"})
	if len(singlePlan.Ranked) == 0 || len(multiPlan.Ranked) == 0 {
		t.Fatalf("expected ranked candidates in both plans")
	}
	singleScore := singlePlan.Ranked[0].Breakdown.EvidenceScore
	multiScore := multiPlan.Ranked[0].Breakdown.EvidenceScore
	if multiScore <= singleScore {
		t.Fatalf("expected multi-resolution strategy to improve derived-only evidence score, got %.3f vs %.3f", multiScore, singleScore)
	}
	if multiPlan.Ranked[0].Breakdown.EvidenceDetail == nil || multiPlan.Ranked[0].Breakdown.EvidenceDetail.StrategyBias <= 0 {
		t.Fatalf("expected strategy bias detail for multi-resolution, got %+v", multiPlan.Ranked[0].Breakdown.EvidenceDetail)
	}
}

func TestBuildRefinementPlanPriorityStats(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 1, MinCandidateSNRDb: 0}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, SNRDb: 8, BandwidthHz: 10000, PeakDb: 2},
		{ID: 2, CenterHz: 200, SNRDb: 12, BandwidthHz: 20000, PeakDb: 4},
	}
	plan := BuildRefinementPlan(cands, policy)
	if plan.PriorityMax < plan.PriorityMin {
		t.Fatalf("priority bounds invalid: %+v", plan)
	}
	res := AdmitRefinementPlan(plan, policy, time.Now(), &RefinementHold{Active: map[int64]time.Time{}})
	if len(res.Plan.Selected) != 1 {
		t.Fatalf("expected 1 admitted, got %d", len(res.Plan.Selected))
	}
	if res.Plan.PriorityCutoff != res.Plan.Selected[0].Priority {
		t.Fatalf("expected cutoff to match selection, got %.2f vs %.2f", res.Plan.PriorityCutoff, res.Plan.Selected[0].Priority)
	}
	if res.Plan.Selected[0].Breakdown == nil {
		t.Fatalf("expected breakdown on selected candidate")
	}
	if res.Plan.Selected[0].Score == nil || res.Plan.Selected[0].Score.Total == 0 {
		t.Fatalf("expected score on selected candidate")
	}
}

func TestBuildRefinementPlanStrategyBias(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 1, MinCandidateSNRDb: 0, Intent: "archive-and-triage"}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, SNRDb: 12, BandwidthHz: 5000, PeakDb: 1},
		{ID: 2, CenterHz: 200, SNRDb: 11, BandwidthHz: 100000, PeakDb: 1},
	}
	plan := BuildRefinementPlan(cands, policy)
	if len(plan.Ranked) != 2 {
		t.Fatalf("expected ranked candidates, got %d", len(plan.Ranked))
	}
	if plan.Ranked[0].Candidate.ID != 2 {
		t.Fatalf("expected archive-oriented strategy to favor wider candidate, got %+v", plan.Ranked[0])
	}
}

func TestAdmitRefinementPlanAppliesBudget(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 1, MinCandidateSNRDb: 10}
	cands := []Candidate{
		{ID: 2, CenterHz: 200, SNRDb: 12, BandwidthHz: 50000, PeakDb: 3},
		{ID: 3, CenterHz: 300, SNRDb: 11, BandwidthHz: 25000, PeakDb: 2},
	}
	plan := BuildRefinementPlan(cands, policy)
	res := AdmitRefinementPlan(plan, policy, time.Now(), &RefinementHold{Active: map[int64]time.Time{}})
	if len(res.Plan.Selected) != 1 || res.Plan.Selected[0].Candidate.ID != 2 {
		t.Fatalf("unexpected admission selection: %+v", res.Plan.Selected)
	}
	if res.Plan.DroppedByBudget != 1 {
		t.Fatalf("expected 1 dropped by budget, got %d", res.Plan.DroppedByBudget)
	}
	item2 := findWorkItem(res.WorkItems, 2)
	if item2 == nil || item2.Status != RefinementStatusAdmitted {
		t.Fatalf("expected candidate 2 admitted, got %+v", item2)
	}
	if item2.Admission == nil || item2.Admission.Class != AdmissionClassAdmit || item2.Admission.Tier == "" {
		t.Fatalf("expected admission class/tier on admitted item, got %+v", item2.Admission)
	}
	item3 := findWorkItem(res.WorkItems, 3)
	if item3 == nil || item3.Status != RefinementStatusSkipped {
		t.Fatalf("expected candidate 3 skipped, got %+v", item3)
	}
	if item3.Admission == nil || item3.Admission.Class != AdmissionClassDefer {
		t.Fatalf("expected deferred admission class on skipped item, got %+v", item3.Admission)
	}
}

func TestAdmitRefinementPlanDisplacedByHold(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 1, MinCandidateSNRDb: 0, DecisionHoldMs: 500}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, SNRDb: 9},
		{ID: 2, CenterHz: 200, SNRDb: 12},
		{ID: 3, CenterHz: 300, SNRDb: 2},
	}
	plan := BuildRefinementPlan(cands, policy)
	hold := &RefinementHold{Active: map[int64]time.Time{1: time.Now().Add(2 * time.Second)}}
	res := AdmitRefinementPlan(plan, policy, time.Now(), hold)
	if len(res.Plan.Selected) != 1 || res.Plan.Selected[0].Candidate.ID != 1 {
		t.Fatalf("expected held candidate to remain admitted, got %+v", res.Plan.Selected)
	}
	item2 := findWorkItem(res.WorkItems, 2)
	if item2 == nil || item2.Status != RefinementStatusDisplaced {
		t.Fatalf("expected higher priority candidate displaced, got %+v", item2)
	}
	if item2.Admission == nil || item2.Admission.Class != AdmissionClassDisplace {
		t.Fatalf("expected displaced admission class, got %+v", item2.Admission)
	}
	if res.Admission.DisplacedByHold != 1 || res.Admission.Displaced != 1 {
		t.Fatalf("expected displaced-by-hold count 1, got %+v", res.Admission)
	}
	if res.Admission.DecisionHoldMs != policy.DecisionHoldMs {
		t.Fatalf("expected decision hold ms %d, got %d", policy.DecisionHoldMs, res.Admission.DecisionHoldMs)
	}
}

func TestAdmitRefinementPlanOpportunisticDisplacement(t *testing.T) {
	policy := Policy{MaxRefinementJobs: 1, MinCandidateSNRDb: 0, DecisionHoldMs: 500}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, SNRDb: 5},
		{ID: 2, CenterHz: 200, SNRDb: 25},
	}
	plan := BuildRefinementPlan(cands, policy)
	hold := &RefinementHold{Active: map[int64]time.Time{1: time.Now().Add(2 * time.Second)}}
	res := AdmitRefinementPlan(plan, policy, time.Now(), hold)
	if len(res.Plan.Selected) != 1 || res.Plan.Selected[0].Candidate.ID != 2 {
		t.Fatalf("expected opportunistic displacement to admit candidate 2, got %+v", res.Plan.Selected)
	}
	item1 := findWorkItem(res.WorkItems, 1)
	if item1 == nil || item1.Status != RefinementStatusDisplaced {
		t.Fatalf("expected candidate 1 displaced, got %+v", item1)
	}
	if item1.Admission == nil || item1.Admission.Class != AdmissionClassDisplace {
		t.Fatalf("expected displaced admission class, got %+v", item1.Admission)
	}
	if item1.Admission == nil || !strings.Contains(item1.Admission.Reason, ReasonTagDisplaceOpportunist) {
		t.Fatalf("expected opportunistic displacement reason, got %+v", item1.Admission)
	}
}

func TestRefinementStrategyUsesProfile(t *testing.T) {
	strategy, reason := refinementStrategy(Policy{Profile: "digital-hunting"})
	if strategy != "digital-hunting" || reason != "profile" {
		t.Fatalf("expected digital profile to set strategy, got %s (%s)", strategy, reason)
	}
	strategy, reason = refinementStrategy(Policy{Profile: "archive"})
	if strategy != "archive-oriented" || reason != "profile" {
		t.Fatalf("expected archive profile to set strategy, got %s (%s)", strategy, reason)
	}
}

func TestRefinementStrategyUsesIntentAndSurveillance(t *testing.T) {
	strategy, reason := refinementStrategy(Policy{Intent: "decode-digital"})
	if strategy != "digital-hunting" || reason != "intent" {
		t.Fatalf("expected intent to set digital strategy, got %s (%s)", strategy, reason)
	}
	strategy, reason = refinementStrategy(Policy{Intent: "archive-and-triage"})
	if strategy != "archive-oriented" || reason != "intent" {
		t.Fatalf("expected intent to set archive strategy, got %s (%s)", strategy, reason)
	}
	strategy, reason = refinementStrategy(Policy{SurveillanceStrategy: "multi-resolution"})
	if strategy != "multi-resolution" || reason != "surveillance-strategy" {
		t.Fatalf("expected surveillance strategy to set multi-resolution, got %s (%s)", strategy, reason)
	}
}

func findWorkItem(items []RefinementWorkItem, id int64) *RefinementWorkItem {
	for i := range items {
		if items[i].Candidate.ID == id {
			return &items[i]
		}
	}
	return nil
}

func findScheduled(items []ScheduledCandidate, id int64) *ScheduledCandidate {
	for i := range items {
		if items[i].Candidate.ID == id {
			return &items[i]
		}
	}
	return nil
}

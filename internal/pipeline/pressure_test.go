package pipeline

import (
	"strings"
	"testing"
	"time"
)

func TestBudgetPreferenceAffectsEffectiveBudgets(t *testing.T) {
	archivePolicy := Policy{
		Profile:             "archive",
		Intent:              "archive-and-triage",
		MaxRecordingStreams: 10,
		MaxDecodeJobs:       10,
		MaxRefinementJobs:   6,
	}
	archiveBudget := BudgetModelFromPolicy(archivePolicy)
	if archiveBudget.Record.EffectiveMax <= archiveBudget.Decode.EffectiveMax {
		t.Fatalf("expected archive preference to favor record, got record=%.2f decode=%.2f", archiveBudget.Record.EffectiveMax, archiveBudget.Decode.EffectiveMax)
	}
	if len(archiveBudget.Preference.Reasons) == 0 {
		t.Fatalf("expected archive preference reasons to be populated")
	}

	digitalPolicy := Policy{
		Profile:             "digital-hunting",
		Intent:              "decode-digital",
		MaxRecordingStreams: 10,
		MaxDecodeJobs:       10,
		MaxRefinementJobs:   6,
	}
	digitalBudget := BudgetModelFromPolicy(digitalPolicy)
	if digitalBudget.Decode.EffectiveMax <= digitalBudget.Record.EffectiveMax {
		t.Fatalf("expected digital preference to favor decode, got record=%.2f decode=%.2f", digitalBudget.Record.EffectiveMax, digitalBudget.Decode.EffectiveMax)
	}
}

func TestPressureSummaryReflectsPreference(t *testing.T) {
	policy := Policy{
		Profile:             "digital-hunting",
		Intent:              "decode-digital",
		MaxRecordingStreams: 4,
		MaxDecodeJobs:       4,
		MaxRefinementJobs:   2,
	}
	budget := BudgetModelFromPolicy(policy)
	queue := DecisionQueueStats{
		RecordQueued:   4,
		DecodeQueued:   4,
		RecordSelected: 2,
		DecodeSelected: 2,
		RecordActive:   1,
		DecodeActive:   1,
	}
	pressure := BuildBudgetPressureSummary(budget, RefinementAdmission{}, queue)
	if pressure.Record.Pressure <= 0 || pressure.Decode.Pressure <= 0 {
		t.Fatalf("expected non-zero pressure ratios, got record=%.2f decode=%.2f", pressure.Record.Pressure, pressure.Decode.Pressure)
	}
	if pressure.Record.Pressure <= pressure.Decode.Pressure {
		t.Fatalf("expected record pressure to be higher than decode under digital preference, got record=%.2f decode=%.2f", pressure.Record.Pressure, pressure.Decode.Pressure)
	}
}

func TestRefinementPressureTagsAdmission(t *testing.T) {
	policy := Policy{Profile: "archive", MaxRefinementJobs: 1, MinCandidateSNRDb: 0}
	cands := []Candidate{
		{ID: 1, CenterHz: 100, SNRDb: 10},
		{ID: 2, CenterHz: 200, SNRDb: 9},
	}
	plan := BuildRefinementPlan(cands, policy)
	res := AdmitRefinementPlan(plan, policy, time.Now(), &RefinementHold{Active: map[int64]time.Time{}})
	if res.Admission.Pressure.Level == "" || res.Admission.Pressure.Level == "idle" {
		t.Fatalf("expected pressure level to be set, got %+v", res.Admission.Pressure)
	}
	if res.Admission.Reason == "" || !strings.Contains(res.Admission.Reason, "pressure:") {
		t.Fatalf("expected admission reason to include pressure tag, got %s", res.Admission.Reason)
	}
}

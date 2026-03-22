package pipeline

import "testing"

func TestRebalanceArchiveProtectsRecord(t *testing.T) {
	policy := Policy{
		Profile:             "archive",
		Intent:              "archive-and-triage",
		MaxRefinementJobs:   4,
		MaxRecordingStreams: 4,
		MaxDecodeJobs:       4,
	}
	budget := BudgetModelFromPolicy(policy)
	pressure := BudgetPressureSummary{
		Refinement: pressureFor(0.6),
		Record:     pressureFor(0.6),
		Decode:     pressureFor(1.3),
	}
	rebalanced := ApplyBudgetRebalance(policy, budget, pressure)
	if rebalanced.Record.RebalanceDelta < 0 {
		t.Fatalf("expected record to be protected from donating, got delta=%d", rebalanced.Record.RebalanceDelta)
	}
	if rebalanced.Decode.RebalanceDelta <= 0 {
		t.Fatalf("expected decode to receive a slot, got delta=%d", rebalanced.Decode.RebalanceDelta)
	}
	if rebalanced.Refinement.RebalanceDelta >= 0 {
		t.Fatalf("expected refinement to donate a slot, got delta=%d", rebalanced.Refinement.RebalanceDelta)
	}
}

func TestRebalanceDigitalProtectsDecode(t *testing.T) {
	policy := Policy{
		Profile:             "digital-hunting",
		Intent:              "decode-digital",
		MaxRefinementJobs:   4,
		MaxRecordingStreams: 4,
		MaxDecodeJobs:       4,
	}
	budget := BudgetModelFromPolicy(policy)
	pressure := BudgetPressureSummary{
		Refinement: pressureFor(0.6),
		Record:     pressureFor(1.3),
		Decode:     pressureFor(0.6),
	}
	rebalanced := ApplyBudgetRebalance(policy, budget, pressure)
	if rebalanced.Decode.RebalanceDelta < 0 {
		t.Fatalf("expected decode to be protected from donating, got delta=%d", rebalanced.Decode.RebalanceDelta)
	}
	if rebalanced.Record.RebalanceDelta <= 0 {
		t.Fatalf("expected record to receive a slot, got delta=%d", rebalanced.Record.RebalanceDelta)
	}
	if rebalanced.Refinement.RebalanceDelta >= 0 {
		t.Fatalf("expected refinement to donate a slot, got delta=%d", rebalanced.Refinement.RebalanceDelta)
	}
}

func TestRebalanceAggressiveFavorsRefinement(t *testing.T) {
	policy := Policy{
		Profile:             "wideband-aggressive",
		Intent:              "wideband-surveillance",
		MaxRefinementJobs:   6,
		MaxRecordingStreams: 4,
		MaxDecodeJobs:       4,
	}
	budget := BudgetModelFromPolicy(policy)
	pressure := BudgetPressureSummary{
		Refinement: pressureFor(1.3),
		Record:     pressureFor(0.5),
		Decode:     pressureFor(0.5),
	}
	rebalanced := ApplyBudgetRebalance(policy, budget, pressure)
	if rebalanced.Refinement.RebalanceDelta <= 0 {
		t.Fatalf("expected refinement to receive slots, got delta=%d", rebalanced.Refinement.RebalanceDelta)
	}
}

func TestRebalanceLegacyStaysConservative(t *testing.T) {
	policy := Policy{
		Profile:             "legacy",
		Intent:              "general-monitoring",
		MaxRefinementJobs:   4,
		MaxRecordingStreams: 4,
		MaxDecodeJobs:       4,
	}
	budget := BudgetModelFromPolicy(policy)
	pressure := BudgetPressureSummary{
		Refinement: pressureFor(0.5),
		Record:     pressureFor(1.3),
		Decode:     pressureFor(0.5),
	}
	rebalanced := ApplyBudgetRebalance(policy, budget, pressure)
	if rebalanced.Rebalance.Active {
		t.Fatalf("expected legacy rebalance to remain inactive")
	}
	if rebalanced.Refinement.RebalanceDelta != 0 || rebalanced.Record.RebalanceDelta != 0 || rebalanced.Decode.RebalanceDelta != 0 {
		t.Fatalf("expected no rebalance deltas, got ref=%d record=%d decode=%d", rebalanced.Refinement.RebalanceDelta, rebalanced.Record.RebalanceDelta, rebalanced.Decode.RebalanceDelta)
	}
}

func pressureFor(value float64) BudgetPressure {
	level := ""
	switch {
	case value >= 1.5:
		level = "critical"
	case value >= 1.15:
		level = "high"
	case value >= 0.85:
		level = "elevated"
	case value > 0:
		level = "steady"
	default:
		level = "idle"
	}
	demand := 1
	if value == 0 {
		demand = 0
	}
	return BudgetPressure{Pressure: value, Level: level, Demand: demand}
}

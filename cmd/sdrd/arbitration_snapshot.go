package main

import "sdr-wideband-suite/internal/pipeline"

func buildArbitrationSnapshot(step pipeline.RefinementStep, arb pipeline.ArbitrationState) *ArbitrationSnapshot {
	return &ArbitrationSnapshot{
		Budgets:             &arb.Budgets,
		HoldPolicy:          &arb.HoldPolicy,
		RefinementAdmission: &arb.Refinement,
		Queue:               arb.Queue,
		Pressure:            &arb.Pressure,
		Rebalance:           &arb.Rebalance,
		DecisionSummary:     summarizeDecisions(step.Result.Decisions),
		DecisionItems:       compactDecisions(step.Result.Decisions),
	}
}

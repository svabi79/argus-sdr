package main

import "sdr-wideband-suite/internal/pipeline"

func buildArbitrationSnapshot(step pipeline.RefinementStep, queue decisionQueueStats) *ArbitrationSnapshot {
	return &ArbitrationSnapshot{
		Budgets:         &step.Input.Budgets,
		RefinementPlan:  &step.Input.Plan,
		Queue:           queue,
		DecisionSummary: summarizeDecisions(step.Result.Decisions),
		DecisionItems:   compactDecisions(step.Result.Decisions),
	}
}

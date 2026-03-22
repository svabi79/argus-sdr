package main

import (
	"time"

	"sdr-wideband-suite/internal/pipeline"
)

type arbitrator struct {
	refinementHold *pipeline.RefinementHold
	queues         *decisionQueues
}

func newArbitrator() *arbitrator {
	return &arbitrator{
		refinementHold: &pipeline.RefinementHold{Active: map[int64]time.Time{}},
		queues:         newDecisionQueues(),
	}
}

func (a *arbitrator) AdmitRefinement(plan pipeline.RefinementPlan, policy pipeline.Policy, now time.Time) pipeline.RefinementAdmissionResult {
	if a == nil {
		return pipeline.AdmitRefinementPlan(plan, policy, now, nil)
	}
	if a.refinementHold == nil {
		a.refinementHold = &pipeline.RefinementHold{Active: map[int64]time.Time{}}
	}
	return pipeline.AdmitRefinementPlan(plan, policy, now, a.refinementHold)
}

func (a *arbitrator) ApplyDecisions(decisions []pipeline.SignalDecision, budget pipeline.BudgetModel, now time.Time, policy pipeline.Policy) decisionQueueStats {
	if a == nil || a.queues == nil {
		return decisionQueueStats{}
	}
	return a.queues.Apply(decisions, budget, now, policy)
}

package pipeline

import "time"

type Arbiter struct {
	refinementHold *RefinementHold
	queues         *decisionQueues
}

func NewArbiter() *Arbiter {
	return &Arbiter{
		refinementHold: &RefinementHold{Active: map[int64]time.Time{}},
		queues:         newDecisionQueues(),
	}
}

func (a *Arbiter) AdmitRefinement(plan RefinementPlan, policy Policy, now time.Time) RefinementAdmissionResult {
	if a == nil {
		return AdmitRefinementPlan(plan, policy, now, nil)
	}
	if a.refinementHold == nil {
		a.refinementHold = &RefinementHold{Active: map[int64]time.Time{}}
	}
	return AdmitRefinementPlan(plan, policy, now, a.refinementHold)
}

func (a *Arbiter) AdmitRefinementWithBudget(plan RefinementPlan, policy Policy, budget BudgetModel, now time.Time) RefinementAdmissionResult {
	if a == nil {
		return AdmitRefinementPlanWithBudget(plan, policy, budget, now, nil)
	}
	if a.refinementHold == nil {
		a.refinementHold = &RefinementHold{Active: map[int64]time.Time{}}
	}
	return AdmitRefinementPlanWithBudget(plan, policy, budget, now, a.refinementHold)
}

func (a *Arbiter) ApplyDecisions(decisions []SignalDecision, budget BudgetModel, now time.Time, policy Policy) DecisionQueueStats {
	if a == nil || a.queues == nil {
		return DecisionQueueStats{}
	}
	return a.queues.Apply(decisions, budget, now, policy)
}

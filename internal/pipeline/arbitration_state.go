package pipeline

type ArbitrationState struct {
	Budgets    BudgetModel           `json:"budgets,omitempty"`
	HoldPolicy HoldPolicy            `json:"hold_policy,omitempty"`
	Refinement RefinementAdmission   `json:"refinement,omitempty"`
	Queue      DecisionQueueStats    `json:"queue,omitempty"`
	Pressure   BudgetPressureSummary `json:"pressure,omitempty"`
	Rebalance  BudgetRebalance       `json:"rebalance,omitempty"`
}

func BuildArbitrationState(policy Policy, budget BudgetModel, admission RefinementAdmission, queue DecisionQueueStats) ArbitrationState {
	return ArbitrationState{
		Budgets:    budget,
		HoldPolicy: HoldPolicyFromPolicy(policy),
		Refinement: admission,
		Queue:      queue,
		Pressure:   BuildBudgetPressureSummary(budget, admission, queue),
		Rebalance:  budget.Rebalance,
	}
}

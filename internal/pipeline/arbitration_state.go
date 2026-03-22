package pipeline

type ArbitrationState struct {
	Budgets    BudgetModel         `json:"budgets,omitempty"`
	HoldPolicy HoldPolicy          `json:"hold_policy,omitempty"`
	Refinement RefinementAdmission `json:"refinement,omitempty"`
	Queue      DecisionQueueStats  `json:"queue,omitempty"`
}

func BuildArbitrationState(policy Policy, budget BudgetModel, admission RefinementAdmission, queue DecisionQueueStats) ArbitrationState {
	return ArbitrationState{
		Budgets:    budget,
		HoldPolicy: HoldPolicyFromPolicy(policy),
		Refinement: admission,
		Queue:      queue,
	}
}

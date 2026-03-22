package pipeline

import "strings"

type BudgetQueue struct {
	Max        int     `json:"max"`
	IntentBias float64 `json:"intent_bias,omitempty"`
	Source     string  `json:"source,omitempty"`
}

type BudgetModel struct {
	Refinement BudgetQueue `json:"refinement"`
	Record     BudgetQueue `json:"record"`
	Decode     BudgetQueue `json:"decode"`
	HoldMs     int         `json:"hold_ms"`
	Intent     string      `json:"intent,omitempty"`
	Profile    string      `json:"profile,omitempty"`
	Strategy   string      `json:"strategy,omitempty"`
}

func BudgetModelFromPolicy(policy Policy) BudgetModel {
	recordBias, decodeBias := budgetIntentBias(policy.Intent)
	refBudget, refSource := refinementBudgetFromPolicy(policy)
	return BudgetModel{
		Refinement: BudgetQueue{
			Max:    refBudget,
			Source: refSource,
		},
		Record: BudgetQueue{
			Max:        policy.MaxRecordingStreams,
			IntentBias: recordBias,
			Source:     "resources.max_recording_streams",
		},
		Decode: BudgetQueue{
			Max:        policy.MaxDecodeJobs,
			IntentBias: decodeBias,
			Source:     "resources.max_decode_jobs",
		},
		HoldMs:   policy.DecisionHoldMs,
		Intent:   policy.Intent,
		Profile:  policy.Profile,
		Strategy: policy.RefinementStrategy,
	}
}

func refinementBudgetFromPolicy(policy Policy) (int, string) {
	budget := policy.MaxRefinementJobs
	source := "resources.max_refinement_jobs"
	if policy.RefinementMaxConcurrent > 0 && (budget <= 0 || policy.RefinementMaxConcurrent < budget) {
		budget = policy.RefinementMaxConcurrent
		source = "refinement.max_concurrent"
	}
	return budget, source
}

func budgetIntentBias(intent string) (float64, float64) {
	if intent == "" {
		return 0, 0
	}
	recordBias := 0.0
	decodeBias := 0.0
	intent = strings.ToLower(intent)
	if strings.Contains(intent, "archive") || strings.Contains(intent, "record") {
		recordBias += 1.5
	}
	if strings.Contains(intent, "triage") {
		recordBias += 0.5
		decodeBias += 0.5
	}
	if strings.Contains(intent, "decode") || strings.Contains(intent, "analysis") {
		decodeBias += 1.0
	}
	if strings.Contains(intent, "digital") {
		decodeBias += 0.5
	}
	return recordBias, decodeBias
}

package pipeline

import "strings"

type BudgetQueue struct {
	Max            int     `json:"max"`
	IntentBias     float64 `json:"intent_bias,omitempty"`
	Preference     float64 `json:"preference,omitempty"`
	EffectiveMax   float64 `json:"effective_max,omitempty"`
	RebalancedMax  int     `json:"rebalanced_max,omitempty"`
	RebalanceDelta int     `json:"rebalance_delta,omitempty"`
	Source         string  `json:"source,omitempty"`
}

type BudgetPreference struct {
	Refinement float64  `json:"refinement"`
	Record     float64  `json:"record"`
	Decode     float64  `json:"decode"`
	Reasons    []string `json:"reasons,omitempty"`
}

type BudgetModel struct {
	Refinement BudgetQueue      `json:"refinement"`
	Record     BudgetQueue      `json:"record"`
	Decode     BudgetQueue      `json:"decode"`
	HoldMs     int              `json:"hold_ms"`
	Intent     string           `json:"intent,omitempty"`
	Profile    string           `json:"profile,omitempty"`
	Strategy   string           `json:"strategy,omitempty"`
	Preference BudgetPreference `json:"preference,omitempty"`
	Rebalance  BudgetRebalance  `json:"rebalance,omitempty"`
}

func BudgetModelFromPolicy(policy Policy) BudgetModel {
	recordBias, decodeBias := budgetIntentBias(policy.Intent)
	refBudget, refSource := refinementBudgetFromPolicy(policy)
	preference := BudgetPreferenceFromPolicy(policy)
	refEffective := effectiveBudget(refBudget, preference.Refinement)
	recordEffective := effectiveBudget(policy.MaxRecordingStreams, preference.Record)
	decodeEffective := effectiveBudget(policy.MaxDecodeJobs, preference.Decode)
	return BudgetModel{
		Refinement: BudgetQueue{
			Max:          refBudget,
			Preference:   preference.Refinement,
			EffectiveMax: refEffective,
			Source:       refSource,
		},
		Record: BudgetQueue{
			Max:          policy.MaxRecordingStreams,
			IntentBias:   recordBias,
			Preference:   preference.Record,
			EffectiveMax: recordEffective,
			Source:       "resources.max_recording_streams",
		},
		Decode: BudgetQueue{
			Max:          policy.MaxDecodeJobs,
			IntentBias:   decodeBias,
			Preference:   preference.Decode,
			EffectiveMax: decodeEffective,
			Source:       "resources.max_decode_jobs",
		},
		HoldMs:     policy.DecisionHoldMs,
		Intent:     policy.Intent,
		Profile:    policy.Profile,
		Strategy:   policy.RefinementStrategy,
		Preference: preference,
	}
}

func BudgetModelFromPolicyWithRebalance(policy Policy, pressure BudgetPressureSummary) BudgetModel {
	base := BudgetModelFromPolicy(policy)
	return ApplyBudgetRebalance(policy, base, pressure)
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

func BudgetPreferenceFromPolicy(policy Policy) BudgetPreference {
	pref := BudgetPreference{Refinement: 1.0, Record: 1.0, Decode: 1.0}
	reasons := make([]string, 0, 6)
	addReason := func(tag string) {
		if tag == "" {
			return
		}
		for _, r := range reasons {
			if r == tag {
				return
			}
		}
		reasons = append(reasons, tag)
	}

	profile := strings.ToLower(strings.TrimSpace(policy.Profile))
	intent := strings.ToLower(strings.TrimSpace(policy.Intent))
	strategy := strings.ToLower(strings.TrimSpace(policy.RefinementStrategy))

	if strings.Contains(profile, "archive") {
		pref.Record += 0.6
		pref.Decode += 0.2
		pref.Refinement += 0.15
		addReason("profile:archive")
	}
	if strings.Contains(profile, "digital") {
		pref.Decode += 0.6
		pref.Record += 0.1
		pref.Refinement += 0.15
		addReason("profile:digital")
	}
	if strings.Contains(profile, "aggressive") {
		pref.Refinement += 0.35
		addReason("profile:aggressive")
	}

	if strings.Contains(intent, "archive") || strings.Contains(intent, "record") {
		pref.Record += 0.5
		addReason("intent:record")
	}
	if strings.Contains(intent, "decode") || strings.Contains(intent, "analysis") || strings.Contains(intent, "classif") {
		pref.Decode += 0.5
		addReason("intent:decode")
	}
	if strings.Contains(intent, "digital") || strings.Contains(intent, "hunt") {
		pref.Decode += 0.25
		addReason("intent:digital")
	}
	if strings.Contains(intent, "wideband") || strings.Contains(intent, "surveillance") {
		pref.Refinement += 0.25
		addReason("intent:wideband")
	}

	if strings.Contains(strategy, "archive") {
		pref.Record += 0.2
		pref.Refinement += 0.1
		addReason("strategy:archive")
	}
	if strings.Contains(strategy, "digital") {
		pref.Decode += 0.2
		addReason("strategy:digital")
	}
	if strings.Contains(strategy, "multi") {
		pref.Refinement += 0.2
		addReason("strategy:multi-resolution")
	}

	pref.Refinement = clampPreference(pref.Refinement)
	pref.Record = clampPreference(pref.Record)
	pref.Decode = clampPreference(pref.Decode)
	pref.Reasons = reasons
	return pref
}

func clampPreference(value float64) float64 {
	if value < 0.35 {
		return 0.35
	}
	return value
}

func effectiveBudget(max int, preference float64) float64 {
	if max <= 0 {
		return 0
	}
	if preference <= 0 {
		preference = 1.0
	}
	return float64(max) * preference
}

func budgetQueueLimit(queue BudgetQueue) int {
	if queue.RebalanceDelta != 0 {
		return queue.RebalancedMax
	}
	if queue.RebalancedMax != 0 {
		return queue.RebalancedMax
	}
	return queue.Max
}

package pipeline

import (
	"math"
	"strings"
	"time"
)

type HoldPolicy struct {
	BaseMs       int      `json:"base_ms"`
	RefinementMs int      `json:"refinement_ms"`
	RecordMs     int      `json:"record_ms"`
	DecodeMs     int      `json:"decode_ms"`
	Profile      string   `json:"profile,omitempty"`
	Strategy     string   `json:"strategy,omitempty"`
	Reasons      []string `json:"reasons,omitempty"`
}

type RefinementHold struct {
	Active map[int64]time.Time
}

type RefinementAdmission struct {
	Budget         int     `json:"budget"`
	BudgetSource   string  `json:"budget_source,omitempty"`
	HoldMs         int     `json:"hold_ms"`
	HoldSource     string  `json:"hold_source,omitempty"`
	Planned        int     `json:"planned"`
	Admitted       int     `json:"admitted"`
	Skipped        int     `json:"skipped"`
	Displaced      int     `json:"displaced"`
	PriorityCutoff float64 `json:"priority_cutoff,omitempty"`
	Reason         string  `json:"reason,omitempty"`
}

type RefinementAdmissionResult struct {
	Plan      RefinementPlan
	WorkItems []RefinementWorkItem
	Admitted  []ScheduledCandidate
	Admission RefinementAdmission
}

func HoldPolicyFromPolicy(policy Policy) HoldPolicy {
	base := policy.DecisionHoldMs
	if base < 0 {
		base = 0
	}
	refMult := 1.0
	recMult := 1.0
	decMult := 1.0
	reasons := make([]string, 0, 2)
	profile := strings.ToLower(strings.TrimSpace(policy.Profile))
	strategy := strings.ToLower(strings.TrimSpace(policy.RefinementStrategy))

	if profileContains(profile, "archive") || strategyContains(strategy, "archive") {
		recMult *= 1.5
		decMult *= 1.1
		refMult *= 1.2
		reasons = append(reasons, "archive")
	}
	if profileContains(profile, "digital") || strategyContains(strategy, "digital") {
		decMult *= 1.6
		recMult *= 0.85
		refMult *= 1.1
		reasons = append(reasons, "digital")
	}
	if profileContains(profile, "aggressive") {
		refMult *= 1.15
		reasons = append(reasons, "aggressive")
	}
	if strategyContains(strings.ToLower(strings.TrimSpace(policy.SurveillanceStrategy)), "multi") {
		refMult *= 1.1
		reasons = append(reasons, "multi-resolution")
	}

	return HoldPolicy{
		BaseMs:       base,
		RefinementMs: scaleHold(base, refMult),
		RecordMs:     scaleHold(base, recMult),
		DecodeMs:     scaleHold(base, decMult),
		Profile:      policy.Profile,
		Strategy:     policy.RefinementStrategy,
		Reasons:      reasons,
	}
}

func AdmitRefinementPlan(plan RefinementPlan, policy Policy, now time.Time, hold *RefinementHold) RefinementAdmissionResult {
	ranked := plan.Ranked
	if len(ranked) == 0 {
		ranked = plan.Selected
	}
	workItems := append([]RefinementWorkItem(nil), plan.WorkItems...)
	admission := RefinementAdmission{
		Budget:       plan.Budget,
		BudgetSource: plan.BudgetSource,
	}
	if len(ranked) == 0 {
		admission.Reason = "no-candidates"
		return RefinementAdmissionResult{Plan: plan, WorkItems: workItems, Admission: admission}
	}

	holdPolicy := HoldPolicyFromPolicy(policy)
	admission.HoldMs = holdPolicy.RefinementMs
	admission.HoldSource = "resources.decision_hold_ms"
	if len(holdPolicy.Reasons) > 0 {
		admission.HoldSource += ":" + strings.Join(holdPolicy.Reasons, ",")
	}

	planned := len(ranked)
	admission.Planned = planned
	selected := map[int64]struct{}{}
	if hold != nil {
		purgeHold(hold.Active, now)
		for id := range hold.Active {
			if rankedContains(ranked, id) {
				selected[id] = struct{}{}
			}
		}
	}
	limit := plan.Budget
	if limit <= 0 || limit > planned {
		limit = planned
	}
	if len(selected) > limit {
		limit = len(selected)
		if limit > planned {
			limit = planned
		}
	}
	for _, cand := range ranked {
		if len(selected) >= limit {
			break
		}
		if _, ok := selected[cand.Candidate.ID]; ok {
			continue
		}
		selected[cand.Candidate.ID] = struct{}{}
	}
	if hold != nil && admission.HoldMs > 0 {
		until := now.Add(time.Duration(admission.HoldMs) * time.Millisecond)
		if hold.Active == nil {
			hold.Active = map[int64]time.Time{}
		}
		for id := range selected {
			hold.Active[id] = until
		}
	}

	admitted := make([]ScheduledCandidate, 0, len(selected))
	for _, cand := range ranked {
		if _, ok := selected[cand.Candidate.ID]; ok {
			admitted = append(admitted, cand)
		}
	}
	admission.Admitted = len(admitted)
	admission.Skipped = planned - admission.Admitted
	if admission.Skipped < 0 {
		admission.Skipped = 0
	}

	displaced := map[int64]struct{}{}
	if len(admitted) > 0 {
		admission.PriorityCutoff = admitted[len(admitted)-1].Priority
		for _, cand := range ranked {
			if _, ok := selected[cand.Candidate.ID]; ok {
				continue
			}
			if cand.Priority >= admission.PriorityCutoff {
				displaced[cand.Candidate.ID] = struct{}{}
			}
		}
	}
	admission.Displaced = len(displaced)

	plan.Selected = admitted
	plan.PriorityCutoff = admission.PriorityCutoff
	plan.DroppedByBudget = admission.Skipped
	for i := range workItems {
		item := &workItems[i]
		if item.Status != RefinementStatusPlanned {
			continue
		}
		id := item.Candidate.ID
		if _, ok := selected[id]; ok {
			item.Status = RefinementStatusAdmitted
			item.Reason = RefinementReasonAdmitted
			continue
		}
		if _, ok := displaced[id]; ok {
			item.Status = RefinementStatusDisplaced
			item.Reason = RefinementReasonDisplaced
			continue
		}
		item.Status = RefinementStatusSkipped
		item.Reason = RefinementReasonBudget
	}
	return RefinementAdmissionResult{
		Plan:      plan,
		WorkItems: workItems,
		Admitted:  admitted,
		Admission: admission,
	}
}

func purgeHold(active map[int64]time.Time, now time.Time) {
	for id, until := range active {
		if now.After(until) {
			delete(active, id)
		}
	}
}

func rankedContains(items []ScheduledCandidate, id int64) bool {
	for _, item := range items {
		if item.Candidate.ID == id {
			return true
		}
	}
	return false
}

func scaleHold(base int, mult float64) int {
	if base <= 0 {
		return 0
	}
	return int(math.Round(float64(base) * mult))
}

func profileContains(profile string, token string) bool {
	if profile == "" || token == "" {
		return false
	}
	return strings.Contains(profile, strings.ToLower(token))
}

func strategyContains(strategy string, token string) bool {
	if strategy == "" || token == "" {
		return false
	}
	return strings.Contains(strategy, strings.ToLower(token))
}

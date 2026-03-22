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
	Budget          int            `json:"budget"`
	BudgetSource    string         `json:"budget_source,omitempty"`
	DecisionHoldMs  int            `json:"decision_hold_ms,omitempty"`
	HoldMs          int            `json:"hold_ms"`
	HoldSource      string         `json:"hold_source,omitempty"`
	Planned         int            `json:"planned"`
	Admitted        int            `json:"admitted"`
	Skipped         int            `json:"skipped"`
	Displaced       int            `json:"displaced"`
	DisplacedByHold int            `json:"displaced_by_hold,omitempty"`
	HoldActive      int            `json:"hold_active"`
	HoldSelected    int            `json:"hold_selected"`
	HoldProtected   int            `json:"hold_protected"`
	HoldExpired     int            `json:"hold_expired"`
	HoldDisplaced   int            `json:"hold_displaced"`
	Opportunistic   int            `json:"opportunistic"`
	PriorityCutoff  float64        `json:"priority_cutoff,omitempty"`
	PriorityTier    string         `json:"priority_tier,omitempty"`
	Reason          string         `json:"reason,omitempty"`
	Pressure        BudgetPressure `json:"pressure,omitempty"`
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
	intent := strings.ToLower(strings.TrimSpace(policy.Intent))

	archiveProfile := profileContains(profile, "archive")
	archiveStrategy := strategyContains(strategy, "archive")
	if archiveProfile || archiveStrategy {
		recMult *= 1.5
		decMult *= 1.1
		refMult *= 1.2
		if archiveProfile {
			reasons = append(reasons, HoldReasonProfileArchive)
		}
		if archiveStrategy {
			reasons = append(reasons, HoldReasonStrategyArchive)
		}
	}
	digitalProfile := profileContains(profile, "digital")
	digitalStrategy := strategyContains(strategy, "digital")
	if digitalProfile || digitalStrategy {
		decMult *= 1.6
		recMult *= 0.85
		refMult *= 1.1
		if digitalProfile {
			reasons = append(reasons, HoldReasonProfileDigital)
		}
		if digitalStrategy {
			reasons = append(reasons, HoldReasonStrategyDigital)
		}
	}
	if profileContains(profile, "aggressive") {
		refMult *= 1.15
		reasons = append(reasons, HoldReasonProfileAggressive)
	}
	if strategyContains(strings.ToLower(strings.TrimSpace(policy.SurveillanceStrategy)), "multi") {
		refMult *= 1.1
		reasons = append(reasons, HoldReasonStrategyMultiRes)
	}
	intentArchive := strings.Contains(intent, "archive") || strings.Contains(intent, "triage") || strings.Contains(intent, "record")
	intentDecode := strings.Contains(intent, "decode") || strings.Contains(intent, "digital") || strings.Contains(intent, "analysis")
	intentSurveillance := strings.Contains(intent, "surveillance") || strings.Contains(intent, "wideband")
	if intentArchive {
		recMult *= 1.25
		refMult *= 1.1
		decMult *= 1.05
		reasons = append(reasons, HoldReasonIntentArchive)
	}
	if intentDecode {
		decMult *= 1.25
		refMult *= 1.05
		reasons = append(reasons, HoldReasonIntentDecode)
	}
	if intentSurveillance {
		refMult *= 1.1
		reasons = append(reasons, HoldReasonIntentSurveillance)
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
	budget := BudgetModelFromPolicy(policy)
	return AdmitRefinementPlanWithBudget(plan, policy, budget, now, hold)
}

func AdmitRefinementPlanWithBudget(plan RefinementPlan, policy Policy, budgetModel BudgetModel, now time.Time, hold *RefinementHold) RefinementAdmissionResult {
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
		admission.Reason = ReasonAdmissionNoCandidates
		return RefinementAdmissionResult{Plan: plan, WorkItems: workItems, Admission: admission}
	}

	holdPolicy := HoldPolicyFromPolicy(policy)
	admission.DecisionHoldMs = holdPolicy.BaseMs
	admission.HoldMs = holdPolicy.RefinementMs
	admission.HoldSource = "resources.decision_hold_ms"
	if len(holdPolicy.Reasons) > 0 {
		admission.HoldSource += ":" + strings.Join(holdPolicy.Reasons, ",")
	}

	planned := len(ranked)
	admission.Planned = planned
	selected := map[int64]struct{}{}
	held := map[int64]struct{}{}
	protected := map[int64]struct{}{}
	expired := map[int64]struct{}{}
	if hold != nil {
		expired = expireHold(hold.Active, now)
		for id := range hold.Active {
			if rankedContains(ranked, id) {
				selected[id] = struct{}{}
				held[id] = struct{}{}
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
	tierByID := map[int64]string{}
	scoreByID := map[int64]float64{}
	familyByID := map[int64]string{}
	familyRankByID := map[int64]int{}
	familyFloorByID := map[int64]string{}
	for _, cand := range ranked {
		id := cand.Candidate.ID
		family, familyRank := signalPriorityMatch(policy, cand.Candidate.Hint, "")
		familyByID[id] = family
		familyRankByID[id] = familyRank
		familyFloorByID[id] = signalPriorityTierFloor(familyRank)
		baseTier := PriorityTierFromRange(cand.Priority, plan.PriorityMin, plan.PriorityMax)
		tierByID[id] = applyTierFloor(baseTier, familyFloorByID[id])
		scoreByID[id] = cand.Priority
	}
	for id := range held {
		if isProtectedTier(tierByID[id]) {
			protected[id] = struct{}{}
		}
	}
	displaceable := buildDisplaceableHold(held, protected, tierByID, scoreByID, familyRankByID)
	opportunistic := map[int64]struct{}{}
	displacedHold := map[int64]struct{}{}
	for _, cand := range ranked {
		if _, ok := selected[cand.Candidate.ID]; ok {
			continue
		}
		if len(selected) < limit {
			selected[cand.Candidate.ID] = struct{}{}
			continue
		}
		if len(displaceable) == 0 {
			continue
		}
		target := displaceable[0]
		if priorityTierRank(tierByID[cand.Candidate.ID]) <= priorityTierRank(tierByID[target]) {
			continue
		}
		displaceable = displaceable[1:]
		delete(selected, target)
		displacedHold[target] = struct{}{}
		selected[cand.Candidate.ID] = struct{}{}
		opportunistic[cand.Candidate.ID] = struct{}{}
	}
	if hold != nil && admission.HoldMs > 0 {
		until := now.Add(time.Duration(admission.HoldMs) * time.Millisecond)
		if hold.Active == nil {
			hold.Active = map[int64]time.Time{}
		}
		for id := range displacedHold {
			delete(hold.Active, id)
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
	if hold != nil {
		admission.HoldActive = len(hold.Active)
	}
	admission.HoldSelected = len(held) - len(displacedHold)
	admission.HoldProtected = len(protected)
	admission.HoldExpired = len(expired)
	admission.HoldDisplaced = len(displacedHold)
	admission.Opportunistic = len(opportunistic)

	displacedByHold := map[int64]struct{}{}
	if len(admitted) > 0 {
		admission.PriorityCutoff = admitted[len(admitted)-1].Priority
		for _, cand := range ranked {
			if _, ok := selected[cand.Candidate.ID]; ok {
				continue
			}
			if cand.Priority >= admission.PriorityCutoff {
				if _, ok := displacedHold[cand.Candidate.ID]; ok {
					continue
				}
				displacedByHold[cand.Candidate.ID] = struct{}{}
			}
		}
	}
	admission.Displaced = len(displacedByHold) + len(displacedHold)
	admission.DisplacedByHold = len(displacedByHold)
	admission.PriorityTier = PriorityTierFromRange(admission.PriorityCutoff, plan.PriorityMin, plan.PriorityMax)
	admission.Pressure = buildRefinementPressure(budgetModel, admission)
	if admission.PriorityCutoff > 0 {
		admission.Reason = admissionReason("admission:budget", policy, holdPolicy, pressureReasonTag(admission.Pressure), "budget:"+slugToken(plan.BudgetSource))
	}

	plan.Selected = admitted
	plan.PriorityCutoff = admission.PriorityCutoff
	plan.DroppedByBudget = admission.Skipped
	for i := range workItems {
		item := &workItems[i]
		if item.Status != RefinementStatusPlanned {
			continue
		}
		id := item.Candidate.ID
		familyRankOut := familyRankForOutput(familyRankByID[id])
		if _, ok := selected[id]; ok {
			item.Status = RefinementStatusAdmitted
			item.Reason = RefinementReasonAdmitted
			class := AdmissionClassAdmit
			reason := "refinement:admit:budget"
			if _, wasHeld := held[id]; wasHeld {
				class = AdmissionClassHold
				reason = "refinement:admit:hold"
			}
			if item.Admission == nil {
				item.Admission = &PriorityAdmission{Basis: "refinement"}
			}
			item.Admission.Class = class
			item.Admission.Score = item.Priority
			item.Admission.Cutoff = admission.PriorityCutoff
			item.Admission.Tier = tierByID[id]
			item.Admission.TierFloor = familyFloorByID[id]
			item.Admission.Family = familyByID[id]
			item.Admission.FamilyRank = familyRankOut
			extras := []string{pressureReasonTag(admission.Pressure), "budget:" + slugToken(plan.BudgetSource)}
			if _, wasHeld := held[id]; wasHeld {
				extras = append(extras, "pressure:hold", ReasonTagHoldActive)
				if _, ok := protected[id]; ok {
					extras = append(extras, ReasonTagHoldProtected)
				}
			}
			if _, ok := opportunistic[id]; ok {
				extras = append(extras, "pressure:hold", ReasonTagDisplaceOpportunist, ReasonTagDisplaceTier, ReasonTagHoldDisplaced)
			}
			item.Admission.Reason = admissionReason(reason, policy, holdPolicy, extras...)
			continue
		}
		if _, ok := displacedHold[id]; ok {
			item.Status = RefinementStatusDisplaced
			item.Reason = RefinementReasonDisplaced
			if item.Admission == nil {
				item.Admission = &PriorityAdmission{Basis: "refinement"}
			}
			item.Admission.Class = AdmissionClassDisplace
			item.Admission.Score = item.Priority
			item.Admission.Cutoff = admission.PriorityCutoff
			item.Admission.Tier = tierByID[id]
			item.Admission.TierFloor = familyFloorByID[id]
			item.Admission.Family = familyByID[id]
			item.Admission.FamilyRank = familyRankOut
			item.Admission.Reason = admissionReason("refinement:displace:hold", policy, holdPolicy, pressureReasonTag(admission.Pressure), "pressure:hold", ReasonTagDisplaceOpportunist, ReasonTagDisplaceTier, ReasonTagHoldDisplaced, "budget:"+slugToken(plan.BudgetSource))
			continue
		}
		if _, ok := displacedByHold[id]; ok {
			item.Status = RefinementStatusDisplaced
			item.Reason = RefinementReasonDisplaced
			if item.Admission == nil {
				item.Admission = &PriorityAdmission{Basis: "refinement"}
			}
			item.Admission.Class = AdmissionClassDisplace
			item.Admission.Score = item.Priority
			item.Admission.Cutoff = admission.PriorityCutoff
			item.Admission.Tier = tierByID[id]
			item.Admission.TierFloor = familyFloorByID[id]
			item.Admission.Family = familyByID[id]
			item.Admission.FamilyRank = familyRankOut
			item.Admission.Reason = admissionReason("refinement:displace:hold", policy, holdPolicy, pressureReasonTag(admission.Pressure), "pressure:hold", ReasonTagHoldActive, "budget:"+slugToken(plan.BudgetSource))
			continue
		}
		item.Status = RefinementStatusSkipped
		item.Reason = RefinementReasonBudget
		if item.Admission == nil {
			item.Admission = &PriorityAdmission{Basis: "refinement"}
		}
		item.Admission.Class = AdmissionClassDefer
		item.Admission.Score = item.Priority
		item.Admission.Cutoff = admission.PriorityCutoff
		item.Admission.Tier = tierByID[id]
		item.Admission.TierFloor = familyFloorByID[id]
		item.Admission.Family = familyByID[id]
		item.Admission.FamilyRank = familyRankOut
		extras := []string{pressureReasonTag(admission.Pressure), "pressure:budget", "budget:" + slugToken(plan.BudgetSource)}
		if _, ok := expired[id]; ok {
			extras = append(extras, ReasonTagHoldExpired)
		}
		item.Admission.Reason = admissionReason("refinement:skip:budget", policy, holdPolicy, extras...)
	}
	return RefinementAdmissionResult{
		Plan:      plan,
		WorkItems: workItems,
		Admitted:  admitted,
		Admission: admission,
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

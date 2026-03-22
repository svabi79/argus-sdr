package pipeline

import (
	"sort"
	"strings"
)

type ScheduledCandidate struct {
	Candidate Candidate               `json:"candidate"`
	Priority  float64                 `json:"priority"`
	Score     *RefinementScore        `json:"score,omitempty"`
	Breakdown *RefinementScoreDetails `json:"breakdown,omitempty"`
}

type RefinementScoreModel struct {
	SNRWeight       float64 `json:"snr_weight"`
	BandwidthWeight float64 `json:"bandwidth_weight"`
	PeakWeight      float64 `json:"peak_weight"`
}

type RefinementScoreDetails struct {
	SNRScore       float64 `json:"snr_score"`
	BandwidthScore float64 `json:"bandwidth_score"`
	PeakScore      float64 `json:"peak_score"`
	PolicyBoost    float64 `json:"policy_boost"`
}

type RefinementScore struct {
	Total     float64                `json:"total"`
	Breakdown RefinementScoreDetails `json:"breakdown"`
	Weights   *RefinementScoreModel  `json:"weights,omitempty"`
}

type RefinementWorkItem struct {
	Candidate Candidate               `json:"candidate"`
	Window    RefinementWindow        `json:"window,omitempty"`
	Priority  float64                 `json:"priority,omitempty"`
	Score     *RefinementScore        `json:"score,omitempty"`
	Breakdown *RefinementScoreDetails `json:"breakdown,omitempty"`
	Status    string                  `json:"status,omitempty"`
	Reason    string                  `json:"reason,omitempty"`
}

const (
	RefinementStatusSelected = "selected"
	RefinementStatusDropped  = "dropped"
	RefinementStatusDeferred = "deferred"
)

const (
	RefinementReasonSelected     = "selected"
	RefinementReasonMonitorGate  = "dropped:monitor"
	RefinementReasonBelowSNR     = "dropped:snr"
	RefinementReasonBudget       = "dropped:budget"
	RefinementReasonDisabled     = "dropped:disabled"
	RefinementReasonUnclassified = "dropped:unclassified"
)

// BuildRefinementPlan scores and budgets candidates for costly local refinement.
// Current heuristic is intentionally simple and deterministic; later phases can add
// richer scoring (novelty, persistence, profile-aware band priorities, decoder value).
func BuildRefinementPlan(candidates []Candidate, policy Policy) RefinementPlan {
	budget := refinementBudget(policy)
	strategy, strategyReason := refinementStrategy(policy)
	plan := RefinementPlan{
		TotalCandidates:   len(candidates),
		MinCandidateSNRDb: policy.MinCandidateSNRDb,
		Budget:            budget,
		Strategy:          strategy,
		StrategyReason:    strategyReason,
	}
	if start, end, ok := monitorBounds(policy); ok {
		plan.MonitorStartHz = start
		plan.MonitorEndHz = end
		if end > start {
			plan.MonitorSpanHz = end - start
		}
	}
	if len(candidates) == 0 {
		return plan
	}
	snrWeight, bwWeight, peakWeight := refinementIntentWeights(policy.Intent)
	scoreModel := RefinementScoreModel{
		SNRWeight:       snrWeight,
		BandwidthWeight: bwWeight,
		PeakWeight:      peakWeight,
	}
	plan.ScoreModel = scoreModel
	scored := make([]ScheduledCandidate, 0, len(candidates))
	workItems := make([]RefinementWorkItem, 0, len(candidates))
	for _, c := range candidates {
		if !candidateInMonitor(policy, c) {
			plan.DroppedByMonitor++
			workItems = append(workItems, RefinementWorkItem{
				Candidate: c,
				Status:    RefinementStatusDropped,
				Reason:    RefinementReasonMonitorGate,
			})
			continue
		}
		if c.SNRDb < policy.MinCandidateSNRDb {
			plan.DroppedBySNR++
			workItems = append(workItems, RefinementWorkItem{
				Candidate: c,
				Status:    RefinementStatusDropped,
				Reason:    RefinementReasonBelowSNR,
			})
			continue
		}
		snrScore := c.SNRDb * snrWeight
		bwScore := 0.0
		peakScore := 0.0
		policyBoost := CandidatePriorityBoost(policy, c.Hint)
		if c.BandwidthHz > 0 {
			bwScore = minFloat64(c.BandwidthHz/25000.0, 6) * bwWeight
		}
		if c.PeakDb > 0 {
			peakScore = (c.PeakDb / 20.0) * peakWeight
		}
		priority := snrScore + bwScore + peakScore + policyBoost
		score := &RefinementScore{
			Total: priority,
			Breakdown: RefinementScoreDetails{
				SNRScore:       snrScore,
				BandwidthScore: bwScore,
				PeakScore:      peakScore,
				PolicyBoost:    policyBoost,
			},
			Weights: &scoreModel,
		}
		scored = append(scored, ScheduledCandidate{
			Candidate: c,
			Priority:  priority,
			Score:     score,
			Breakdown: &score.Breakdown,
		})
		workItems = append(workItems, RefinementWorkItem{
			Candidate: c,
			Priority:  priority,
			Score:     score,
			Breakdown: &score.Breakdown,
			Status:    RefinementStatusDeferred,
		})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Priority == scored[j].Priority {
			return scored[i].Candidate.CenterHz < scored[j].Candidate.CenterHz
		}
		return scored[i].Priority > scored[j].Priority
	})
	if len(scored) > 0 {
		minPriority := scored[0].Priority
		maxPriority := scored[0].Priority
		sumPriority := 0.0
		for _, s := range scored {
			if s.Priority < minPriority {
				minPriority = s.Priority
			}
			if s.Priority > maxPriority {
				maxPriority = s.Priority
			}
			sumPriority += s.Priority
		}
		plan.PriorityMin = minPriority
		plan.PriorityMax = maxPriority
		plan.PriorityAvg = sumPriority / float64(len(scored))
	}
	limit := plan.Budget
	if limit <= 0 || limit > len(scored) {
		limit = len(scored)
	}
	plan.Selected = scored[:limit]
	if len(plan.Selected) > 0 {
		plan.PriorityCutoff = plan.Selected[len(plan.Selected)-1].Priority
	}
	plan.DroppedByBudget = len(scored) - len(plan.Selected)
	if len(plan.Selected) > 0 {
		selected := map[int64]struct{}{}
		for _, s := range plan.Selected {
			selected[s.Candidate.ID] = struct{}{}
		}
		for i := range workItems {
			item := &workItems[i]
			if _, ok := selected[item.Candidate.ID]; ok {
				item.Status = RefinementStatusSelected
				item.Reason = RefinementReasonSelected
			} else if item.Status == RefinementStatusDeferred {
				item.Status = RefinementStatusDropped
				item.Reason = RefinementReasonBudget
			}
		}
	}
	plan.WorkItems = workItems
	return plan
}

func ScheduleCandidates(candidates []Candidate, policy Policy) []ScheduledCandidate {
	return BuildRefinementPlan(candidates, policy).Selected
}

func refinementBudget(policy Policy) int {
	budget := policy.MaxRefinementJobs
	if policy.RefinementMaxConcurrent > 0 && (budget <= 0 || policy.RefinementMaxConcurrent < budget) {
		budget = policy.RefinementMaxConcurrent
	}
	return budget
}

func refinementStrategy(policy Policy) (string, string) {
	intent := strings.ToLower(strings.TrimSpace(policy.Intent))
	switch {
	case strings.Contains(intent, "digital") || strings.Contains(intent, "hunt") || strings.Contains(intent, "decode"):
		return "digital-hunting", "intent"
	case strings.Contains(intent, "archive") || strings.Contains(intent, "triage") || strings.Contains(policy.Mode, "archive"):
		return "archive-oriented", "intent"
	case strings.Contains(strings.ToLower(policy.SurveillanceStrategy), "multi"):
		return "multi-resolution", "surveillance-strategy"
	default:
		return "single-resolution", "default"
	}
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

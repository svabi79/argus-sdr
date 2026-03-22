package pipeline

import "sort"

type ScheduledCandidate struct {
	Candidate Candidate `json:"candidate"`
	Priority  float64   `json:"priority"`
}

// BuildRefinementPlan scores and budgets candidates for costly local refinement.
// Current heuristic is intentionally simple and deterministic; later phases can add
// richer scoring (novelty, persistence, profile-aware band priorities, decoder value).
func BuildRefinementPlan(candidates []Candidate, policy Policy) RefinementPlan {
	budget := policy.MaxRefinementJobs
	if policy.RefinementMaxConcurrent > 0 && (budget <= 0 || policy.RefinementMaxConcurrent < budget) {
		budget = policy.RefinementMaxConcurrent
	}
	plan := RefinementPlan{
		TotalCandidates:   len(candidates),
		MinCandidateSNRDb: policy.MinCandidateSNRDb,
		Budget:            budget,
	}
	if len(candidates) == 0 {
		return plan
	}
	snrWeight, bwWeight, peakWeight := refinementIntentWeights(policy.Intent)
	scored := make([]ScheduledCandidate, 0, len(candidates))
	for _, c := range candidates {
		if !candidateInMonitor(policy, c) {
			plan.DroppedByMonitor++
			continue
		}
		if c.SNRDb < policy.MinCandidateSNRDb {
			plan.DroppedBySNR++
			continue
		}
		priority := c.SNRDb*snrWeight + CandidatePriorityBoost(policy, c.Hint)
		if c.BandwidthHz > 0 {
			priority += minFloat64(c.BandwidthHz/25000.0, 6) * bwWeight
		}
		if c.PeakDb > 0 {
			priority += (c.PeakDb / 20.0) * peakWeight
		}
		scored = append(scored, ScheduledCandidate{Candidate: c, Priority: priority})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Priority == scored[j].Priority {
			return scored[i].Candidate.CenterHz < scored[j].Candidate.CenterHz
		}
		return scored[i].Priority > scored[j].Priority
	})
	limit := plan.Budget
	if limit <= 0 || limit > len(scored) {
		limit = len(scored)
	}
	plan.Selected = scored[:limit]
	plan.DroppedByBudget = len(scored) - len(plan.Selected)
	return plan
}

func ScheduleCandidates(candidates []Candidate, policy Policy) []ScheduledCandidate {
	return BuildRefinementPlan(candidates, policy).Selected
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

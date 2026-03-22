package pipeline

import "sort"

type ScheduledCandidate struct {
	Candidate Candidate          `json:"candidate"`
	Priority  float64            `json:"priority"`
	Breakdown *PriorityBreakdown `json:"breakdown,omitempty"`
}

type PriorityBreakdown struct {
	SNRScore       float64 `json:"snr_score"`
	BandwidthScore float64 `json:"bandwidth_score"`
	PeakScore      float64 `json:"peak_score"`
	PolicyBoost    float64 `json:"policy_boost"`
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
		scored = append(scored, ScheduledCandidate{
			Candidate: c,
			Priority:  priority,
			Breakdown: &PriorityBreakdown{
				SNRScore:       snrScore,
				BandwidthScore: bwScore,
				PeakScore:      peakScore,
				PolicyBoost:    policyBoost,
			},
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

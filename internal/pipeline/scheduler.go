package pipeline

import "sort"

type ScheduledCandidate struct {
	Candidate Candidate `json:"candidate"`
	Priority  float64   `json:"priority"`
}

// ScheduleCandidates picks the most valuable candidates for costly local refinement.
// Current heuristic is intentionally simple and deterministic; later phases can add
// richer scoring (novelty, persistence, profile-aware band priorities, decoder value).
func ScheduleCandidates(candidates []Candidate, policy Policy) []ScheduledCandidate {
	if len(candidates) == 0 {
		return nil
	}
	out := make([]ScheduledCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.SNRDb < policy.MinCandidateSNRDb {
			continue
		}
		priority := c.SNRDb
		if c.BandwidthHz > 0 {
			priority += minFloat64(c.BandwidthHz/25000.0, 6)
		}
		if c.PeakDb > 0 {
			priority += c.PeakDb / 20.0
		}
		out = append(out, ScheduledCandidate{Candidate: c, Priority: priority})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].Candidate.CenterHz < out[j].Candidate.CenterHz
		}
		return out[i].Priority > out[j].Priority
	})
	limit := policy.MaxRefinementJobs
	if limit <= 0 || limit > len(out) {
		limit = len(out)
	}
	return out[:limit]
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

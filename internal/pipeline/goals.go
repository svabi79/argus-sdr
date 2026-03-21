package pipeline

import "strings"

func WantsClass(values []string, class string) bool {
	if len(values) == 0 || class == "" {
		return false
	}
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), class) {
			return true
		}
	}
	return false
}

func CandidatePriorityBoost(policy Policy, hint string) float64 {
	h := strings.ToLower(strings.TrimSpace(hint))
	for i, want := range policy.SignalPriorities {
		w := strings.ToLower(strings.TrimSpace(want))
		if w == "" {
			continue
		}
		if strings.Contains(h, w) || strings.Contains(w, h) {
			return float64(len(policy.SignalPriorities)-i) * 3.0
		}
	}
	return 0
}

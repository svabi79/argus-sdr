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
	boost := hintMatchBoost(policy.SignalPriorities, hint, 3.0)
	boost += hintMatchBoost(policy.AutoRecordClasses, hint, 1.5)
	boost += hintMatchBoost(policy.AutoDecodeClasses, hint, 1.0)
	return boost
}

func DecisionPriorityBoost(policy Policy, hint string, class string, queue string) float64 {
	tag := strings.TrimSpace(hint)
	if tag == "" {
		tag = strings.TrimSpace(class)
	}
	boost := CandidatePriorityBoost(policy, tag)
	switch strings.ToLower(strings.TrimSpace(queue)) {
	case "record":
		boost += hintMatchBoost(policy.AutoRecordClasses, tag, 3.0)
	case "decode":
		boost += hintMatchBoost(policy.AutoDecodeClasses, tag, 3.0)
	}
	return boost
}

func hintMatchBoost(values []string, hint string, weight float64) float64 {
	h := strings.ToLower(strings.TrimSpace(hint))
	if h == "" || len(values) == 0 {
		return 0
	}
	for i, want := range values {
		w := strings.ToLower(strings.TrimSpace(want))
		if w == "" {
			continue
		}
		if strings.Contains(h, w) || strings.Contains(w, h) {
			return float64(len(values)-i) * weight
		}
	}
	return 0
}

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
	boost += intentHintBoost(policy.Intent, hint, 2.0)
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
	boost += intentQueueBoost(policy.Intent, queue)
	boost += queueStrategyBoost(policy, queue)
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

func intentHintBoost(intent string, hint string, weight float64) float64 {
	tokens := intentTokens(intent)
	if len(tokens) == 0 {
		return 0
	}
	return hintMatchBoost(tokens, hint, weight)
}

func intentQueueBoost(intent string, queue string) float64 {
	if intent == "" {
		return 0
	}
	intent = strings.ToLower(intent)
	queue = strings.ToLower(strings.TrimSpace(queue))
	boost := 0.0
	switch queue {
	case "record":
		if strings.Contains(intent, "archive") || strings.Contains(intent, "record") {
			boost += 2.0
		}
		if strings.Contains(intent, "triage") {
			boost += 1.0
		}
	case "decode":
		if strings.Contains(intent, "triage") {
			boost += 1.5
		}
		if strings.Contains(intent, "decode") || strings.Contains(intent, "analysis") || strings.Contains(intent, "classif") {
			boost += 1.0
		}
	}
	return boost
}

func queueStrategyBoost(policy Policy, queue string) float64 {
	queue = strings.ToLower(strings.TrimSpace(queue))
	if queue == "" {
		return 0
	}
	boost := 0.0
	profile := strings.ToLower(strings.TrimSpace(policy.Profile))
	strategy := strings.ToLower(strings.TrimSpace(policy.RefinementStrategy))
	if strings.Contains(profile, "archive") || strings.Contains(strategy, "archive") {
		if queue == "record" {
			boost += 1.5
		} else if queue == "decode" {
			boost += 0.5
		}
	}
	if strings.Contains(profile, "digital") || strings.Contains(strategy, "digital") {
		if queue == "decode" {
			boost += 1.5
		} else if queue == "record" {
			boost += 0.3
		}
	}
	return boost
}

func refinementIntentWeights(intent string) (float64, float64, float64) {
	if intent == "" {
		return 1.0, 1.0, 1.0
	}
	intent = strings.ToLower(intent)
	snrWeight := 1.0
	bwWeight := 1.0
	peakWeight := 1.0
	if strings.Contains(intent, "wideband") {
		bwWeight = 1.25
	}
	if strings.Contains(intent, "high-density") || strings.Contains(intent, "highdensity") {
		bwWeight = 1.4
		peakWeight = 1.1
	}
	if strings.Contains(intent, "archive") || strings.Contains(intent, "triage") {
		snrWeight = 1.15
		peakWeight = 1.1
	}
	return snrWeight, bwWeight, peakWeight
}

func intentTokens(intent string) []string {
	if intent == "" {
		return nil
	}
	fields := strings.FieldsFunc(intent, func(r rune) bool {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			return false
		}
		if r >= '0' && r <= '9' {
			return false
		}
		return true
	})
	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		t := strings.ToLower(strings.TrimSpace(f))
		if len(t) < 2 {
			continue
		}
		tokens = append(tokens, t)
	}
	return tokens
}

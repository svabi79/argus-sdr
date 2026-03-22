package pipeline

import (
	"strings"
)

const (
	PriorityTierCritical   = "critical"
	PriorityTierHigh       = "high"
	PriorityTierMedium     = "medium"
	PriorityTierLow        = "low"
	PriorityTierBackground = "background"
)

const (
	AdmissionClassPlanned  = "plan"
	AdmissionClassAdmit    = "admit"
	AdmissionClassHold     = "hold"
	AdmissionClassDefer    = "defer"
	AdmissionClassDisplace = "displace"
	AdmissionClassDrop     = "drop"
)

type PriorityAdmission struct {
	Tier   string  `json:"tier,omitempty"`
	Class  string  `json:"class,omitempty"`
	Score  float64 `json:"score,omitempty"`
	Cutoff float64 `json:"cutoff,omitempty"`
	Basis  string  `json:"basis,omitempty"`
	Reason string  `json:"reason,omitempty"`
}

func PriorityTierFromRange(score, min, max float64) string {
	if max <= min {
		return PriorityTierHigh
	}
	norm := (score - min) / (max - min)
	switch {
	case norm >= 0.85:
		return PriorityTierCritical
	case norm >= 0.65:
		return PriorityTierHigh
	case norm >= 0.45:
		return PriorityTierMedium
	case norm >= 0.25:
		return PriorityTierLow
	default:
		return PriorityTierBackground
	}
}

func priorityTierRank(tier string) int {
	switch tier {
	case PriorityTierCritical:
		return 4
	case PriorityTierHigh:
		return 3
	case PriorityTierMedium:
		return 2
	case PriorityTierLow:
		return 1
	default:
		return 0
	}
}

func admissionReason(base string, policy Policy, holdPolicy HoldPolicy, extras ...string) string {
	tags := uniqueReasonTags(policy, holdPolicy, extras...)
	if len(tags) == 0 {
		return base
	}
	return base + ":" + strings.Join(tags, ":")
}

func uniqueReasonTags(policy Policy, holdPolicy HoldPolicy, extras ...string) []string {
	seen := map[string]struct{}{}
	tags := make([]string, 0, 6)
	add := func(tag string) {
		if tag == "" {
			return
		}
		if _, ok := seen[tag]; ok {
			return
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	if policy.Profile != "" {
		add("profile:" + slugToken(policy.Profile))
	}
	if policy.Intent != "" {
		add("intent:" + slugToken(policy.Intent))
	}
	if policy.RefinementStrategy != "" {
		add("strategy:" + slugToken(policy.RefinementStrategy))
	}
	for _, reason := range holdPolicy.Reasons {
		add(reason)
	}
	for _, extra := range extras {
		add(extra)
	}
	return tags
}

func slugToken(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return ""
	}
	parts := strings.Fields(input)
	return strings.Join(parts, "-")
}

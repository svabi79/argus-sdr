package pipeline

import "strings"

// SurveillanceDetectionPolicy describes how surveillance levels are governed for detection.
type SurveillanceDetectionPolicy struct {
	DerivedDetection        string `json:"derived_detection"`
	DerivedDetectionEnabled bool   `json:"derived_detection_enabled"`
	DerivedDetectionReason  string `json:"derived_detection_reason,omitempty"`
	PrimaryRole             string `json:"primary_role"`
	DerivedRole             string `json:"derived_role"`
	SupportRole             string `json:"support_role"`
	PresentationRole        string `json:"presentation_role"`
}

func normalizeDerivedDetection(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "on", "true", "enabled", "enable":
		return "on"
	case "off", "false", "disabled", "disable":
		return "off"
	case "auto", "":
		return "auto"
	default:
		return "auto"
	}
}

func strategyIsMulti(strategy string) bool {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "multi-resolution", "multi", "multi-res", "multi_res":
		return true
	default:
		return strings.Contains(strings.ToLower(strategy), "multi")
	}
}

// SurveillanceDetectionPolicyFromPolicy derives detection governance from policy intent/profile.
func SurveillanceDetectionPolicyFromPolicy(policy Policy) SurveillanceDetectionPolicy {
	mode := normalizeDerivedDetection(policy.SurveillanceDerivedDetection)
	enabled := false
	reason := ""
	switch mode {
	case "on":
		enabled = true
		reason = "config"
	case "off":
		enabled = false
		reason = "config"
	default:
		if !strategyIsMulti(policy.SurveillanceStrategy) {
			enabled = false
			reason = "strategy"
		} else {
			intent := strings.ToLower(strings.TrimSpace(policy.Intent))
			profile := strings.ToLower(strings.TrimSpace(policy.Profile))
			modeName := strings.ToLower(strings.TrimSpace(policy.Mode))
			switch {
			case strings.Contains(profile, "archive") || strings.Contains(intent, "archive") || strings.Contains(intent, "triage") || strings.Contains(modeName, "archive"):
				enabled = false
				reason = "archive"
			case strings.Contains(profile, "legacy") || strings.Contains(modeName, "legacy"):
				enabled = false
				reason = "legacy"
			default:
				enabled = true
				reason = "strategy"
			}
		}
	}
	return SurveillanceDetectionPolicy{
		DerivedDetection:        mode,
		DerivedDetectionEnabled: enabled,
		DerivedDetectionReason:  reason,
		PrimaryRole:             RoleSurveillancePrimary,
		DerivedRole:             RoleSurveillanceDerived,
		SupportRole:             RoleSurveillanceSupport,
		PresentationRole:        RolePresentation,
	}
}

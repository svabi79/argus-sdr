package pipeline

import "testing"

func TestSurveillanceDetectionPolicyAuto(t *testing.T) {
	policy := Policy{
		Mode:                 "wideband-balanced",
		Profile:              "wideband-balanced",
		Intent:               "wideband-surveillance",
		SurveillanceStrategy: "multi-resolution",
	}
	policy.SurveillanceDerivedDetection = "auto"
	got := SurveillanceDetectionPolicyFromPolicy(policy)
	if !got.DerivedDetectionEnabled {
		t.Fatalf("expected auto policy to enable derived detection, got %+v", got)
	}

	policy.Profile = "archive"
	policy.Intent = "archive-and-triage"
	got = SurveillanceDetectionPolicyFromPolicy(policy)
	if got.DerivedDetectionEnabled {
		t.Fatalf("expected archive policy to disable derived detection, got %+v", got)
	}

	policy = Policy{SurveillanceStrategy: "single-resolution", SurveillanceDerivedDetection: "auto"}
	got = SurveillanceDetectionPolicyFromPolicy(policy)
	if got.DerivedDetectionEnabled {
		t.Fatalf("expected single-resolution to disable derived detection, got %+v", got)
	}
}

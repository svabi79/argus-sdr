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
	if got.DerivedDetectionMode != "detection" {
		t.Fatalf("expected detection mode, got %+v", got)
	}

	policy.Profile = "archive"
	policy.Intent = "archive-and-triage"
	got = SurveillanceDetectionPolicyFromPolicy(policy)
	if got.DerivedDetectionEnabled {
		t.Fatalf("expected archive policy to disable derived detection, got %+v", got)
	}
	if got.DerivedDetectionMode != "support" {
		t.Fatalf("expected support mode for archive policy, got %+v", got)
	}

	policy = Policy{SurveillanceStrategy: "single-resolution", SurveillanceDerivedDetection: "auto"}
	got = SurveillanceDetectionPolicyFromPolicy(policy)
	if got.DerivedDetectionEnabled {
		t.Fatalf("expected single-resolution to disable derived detection, got %+v", got)
	}
	if got.DerivedDetectionMode != "disabled" {
		t.Fatalf("expected disabled mode for single-resolution, got %+v", got)
	}
}

func TestSurveillanceDetectionPolicyOverrides(t *testing.T) {
	policy := Policy{SurveillanceStrategy: "single-resolution", SurveillanceDerivedDetection: "on"}
	got := SurveillanceDetectionPolicyFromPolicy(policy)
	if got.DerivedDetectionEnabled {
		t.Fatalf("expected single-resolution to force derived detection off, got %+v", got)
	}
	if got.DerivedDetectionReason != "strategy" || got.DerivedDetectionMode != "disabled" {
		t.Fatalf("expected strategy-based disable, got %+v", got)
	}

	policy = Policy{SurveillanceStrategy: "multi-resolution", SurveillanceDerivedDetection: "off"}
	got = SurveillanceDetectionPolicyFromPolicy(policy)
	if got.DerivedDetectionEnabled || got.DerivedDetectionMode != "support" {
		t.Fatalf("expected off to yield support mode, got %+v", got)
	}

	policy = Policy{SurveillanceStrategy: "multi-resolution", SurveillanceDerivedDetection: "on", Profile: "archive"}
	got = SurveillanceDetectionPolicyFromPolicy(policy)
	if !got.DerivedDetectionEnabled || got.DerivedDetectionReason != "config" || got.DerivedDetectionMode != "detection" {
		t.Fatalf("expected config on to override archive, got %+v", got)
	}
}

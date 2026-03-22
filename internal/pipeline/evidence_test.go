package pipeline

import "testing"

func TestLevelRoleClassification(t *testing.T) {
	primary := AnalysisLevel{Name: "surveillance", Role: RoleSurveillancePrimary, Truth: "surveillance"}
	derived := AnalysisLevel{Name: "surveillance-lowres", Role: RoleSurveillanceDerived, Truth: "surveillance"}
	support := AnalysisLevel{Name: "surveillance-lowres", Role: RoleSurveillanceSupport, Truth: "surveillance"}
	presentation := AnalysisLevel{Name: "presentation", Role: RolePresentation, Truth: "presentation"}

	if !IsDetectionLevel(primary) || IsPresentationLevel(primary) || IsSupportLevel(primary) {
		t.Fatalf("primary role classification failed: %+v", primary)
	}
	if !IsDetectionLevel(derived) || IsPresentationLevel(derived) || IsSupportLevel(derived) {
		t.Fatalf("derived role classification failed: %+v", derived)
	}
	if IsDetectionLevel(support) || IsPresentationLevel(support) || !IsSupportLevel(support) {
		t.Fatalf("support role classification failed: %+v", support)
	}
	if IsDetectionLevel(presentation) || !IsPresentationLevel(presentation) || IsSupportLevel(presentation) {
		t.Fatalf("presentation role classification failed: %+v", presentation)
	}
}

func TestCandidateEvidenceStateTracksSupportLevels(t *testing.T) {
	candidate := Candidate{
		ID: 1,
		Evidence: []LevelEvidence{
			{Level: AnalysisLevel{Name: "surveillance", Role: RoleSurveillancePrimary, Truth: "surveillance"}, Provenance: "primary"},
			{Level: AnalysisLevel{Name: "surveillance-lowres", Role: RoleSurveillanceDerived, Truth: "surveillance"}, Provenance: "derived"},
			{Level: AnalysisLevel{Name: "surveillance-support", Role: RoleSurveillanceSupport, Truth: "surveillance"}, Provenance: "support"},
			{Level: AnalysisLevel{Name: "presentation", Role: RolePresentation, Truth: "presentation"}, Provenance: "display"},
		},
	}
	state := CandidateEvidenceStateFor(candidate)
	if state.DetectionLevelCount != 2 || state.PrimaryLevelCount != 1 || state.DerivedLevelCount != 1 {
		t.Fatalf("unexpected detection counts: %+v", state)
	}
	if state.SupportLevelCount != 1 || state.PresentationLevelCount != 1 {
		t.Fatalf("unexpected support/presentation counts: %+v", state)
	}
	if !state.MultiLevelConfirmed || state.DerivedOnly {
		t.Fatalf("unexpected confirmation flags: %+v", state)
	}
}

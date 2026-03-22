package pipeline

import (
	"testing"

	"sdr-wideband-suite/internal/classifier"
)

func TestDecideSignalAction(t *testing.T) {
	policy := Policy{AutoRecordClasses: []string{"WFM"}, AutoDecodeClasses: []string{"RDS", "WFM"}}
	cls := &classifier.Classification{ModType: classifier.ClassWFM}
	decision := DecideSignalAction(policy, Candidate{ID: 1, Hint: "WFM"}, cls)
	if !decision.ShouldRecord {
		t.Fatalf("expected record decision")
	}
	if !decision.ShouldAutoDecode {
		t.Fatalf("expected auto decode decision")
	}
}

func TestDecideSignalActionUsesHintWithoutClass(t *testing.T) {
	policy := Policy{AutoRecordClasses: []string{"WFM"}, AutoDecodeClasses: []string{"FT8"}}
	decision := DecideSignalAction(policy, Candidate{ID: 2, Hint: "WFM"}, nil)
	if !decision.ShouldRecord {
		t.Fatalf("expected record decision from hint")
	}
	if decision.ShouldAutoDecode {
		t.Fatalf("unexpected auto decode decision from hint")
	}
	if decision.Reason == "" {
		t.Fatalf("expected reason for hint-based decision")
	}
}

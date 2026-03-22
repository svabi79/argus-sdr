package pipeline

import (
	"strings"

	"sdr-wideband-suite/internal/classifier"
)

type SignalDecision struct {
	Candidate        Candidate `json:"candidate"`
	Class            string    `json:"class,omitempty"`
	ShouldRecord     bool      `json:"should_record"`
	ShouldAutoDecode bool      `json:"should_auto_decode"`
	Reason           string    `json:"reason,omitempty"`
}

func DecideSignalAction(policy Policy, candidate Candidate, cls *classifier.Classification) SignalDecision {
	decision := SignalDecision{Candidate: candidate}
	classTag := ""
	hintTag := strings.TrimSpace(candidate.Hint)
	if cls != nil {
		decision.Class = string(cls.ModType)
		classTag = decision.Class
	}
	if classTag != "" && WantsClass(policy.AutoRecordClasses, classTag) {
		decision.ShouldRecord = true
		decision.Reason = "matched auto_record_classes"
	} else if classTag == "" && hintTag != "" && WantsClass(policy.AutoRecordClasses, hintTag) {
		decision.ShouldRecord = true
		decision.Reason = "matched auto_record_classes (hint)"
	}
	if classTag != "" && WantsClass(policy.AutoDecodeClasses, classTag) {
		decision.ShouldAutoDecode = true
		if decision.Reason == "" {
			decision.Reason = "matched auto_decode_classes"
		}
	} else if classTag == "" && hintTag != "" && WantsClass(policy.AutoDecodeClasses, hintTag) {
		decision.ShouldAutoDecode = true
		if decision.Reason == "" {
			decision.Reason = "matched auto_decode_classes (hint)"
		}
	}
	if decision.Reason == "" && candidate.Hint != "" {
		decision.Reason = "policy evaluated candidate hint"
	}
	return decision
}

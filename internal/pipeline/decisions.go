package pipeline

import (
	"strings"

	"sdr-wideband-suite/internal/classifier"
)

type SignalDecision struct {
	Candidate        Candidate          `json:"candidate"`
	Class            string             `json:"class,omitempty"`
	ShouldRecord     bool               `json:"should_record"`
	ShouldAutoDecode bool               `json:"should_auto_decode"`
	Reason           string             `json:"reason,omitempty"`
	RecordAdmission  *PriorityAdmission `json:"record_admission,omitempty"`
	DecodeAdmission  *PriorityAdmission `json:"decode_admission,omitempty"`
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
		decision.Reason = DecisionReasonRecordClass
	} else if classTag == "" && hintTag != "" && WantsClass(policy.AutoRecordClasses, hintTag) {
		decision.ShouldRecord = true
		decision.Reason = DecisionReasonRecordHint
	}
	if classTag != "" && WantsClass(policy.AutoDecodeClasses, classTag) {
		decision.ShouldAutoDecode = true
		if decision.Reason == "" {
			decision.Reason = DecisionReasonDecodeClass
		}
	} else if classTag == "" && hintTag != "" && WantsClass(policy.AutoDecodeClasses, hintTag) {
		decision.ShouldAutoDecode = true
		if decision.Reason == "" {
			decision.Reason = DecisionReasonDecodeHint
		}
	}
	if decision.Reason == "" && candidate.Hint != "" {
		decision.Reason = DecisionReasonHintOnly
	}
	return decision
}

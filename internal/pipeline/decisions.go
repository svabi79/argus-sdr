package pipeline

import "sdr-wideband-suite/internal/classifier"

type SignalDecision struct {
	Candidate        Candidate `json:"candidate"`
	Class            string    `json:"class,omitempty"`
	ShouldRecord     bool      `json:"should_record"`
	ShouldAutoDecode bool      `json:"should_auto_decode"`
	Reason           string    `json:"reason,omitempty"`
}

func DecideSignalAction(policy Policy, candidate Candidate, cls *classifier.Classification) SignalDecision {
	decision := SignalDecision{Candidate: candidate}
	if cls != nil {
		decision.Class = string(cls.ModType)
	}
	if cls != nil && WantsClass(policy.AutoRecordClasses, string(cls.ModType)) {
		decision.ShouldRecord = true
		decision.Reason = "matched auto_record_classes"
	}
	if cls != nil && WantsClass(policy.AutoDecodeClasses, string(cls.ModType)) {
		decision.ShouldAutoDecode = true
		if decision.Reason == "" {
			decision.Reason = "matched auto_decode_classes"
		}
	}
	if decision.Reason == "" && candidate.Hint != "" {
		decision.Reason = "policy evaluated candidate hint"
	}
	return decision
}

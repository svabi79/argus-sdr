package main

import "sdr-wideband-suite/internal/pipeline"

type compactDecision struct {
	ID        int64              `json:"id"`
	Class     string             `json:"class,omitempty"`
	Record    bool               `json:"record"`
	Decode    bool               `json:"decode"`
	Reason    string             `json:"reason,omitempty"`
	Candidate pipeline.Candidate `json:"candidate"`
}

func compactDecisions(decisions []pipeline.SignalDecision) []compactDecision {
	out := make([]compactDecision, 0, len(decisions))
	for _, d := range decisions {
		out = append(out, compactDecision{
			ID:        d.Candidate.ID,
			Class:     d.Class,
			Record:    d.ShouldRecord,
			Decode:    d.ShouldAutoDecode,
			Reason:    d.Reason,
			Candidate: d.Candidate,
		})
	}
	return out
}

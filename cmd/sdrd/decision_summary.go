package main

import "sdr-wideband-suite/internal/pipeline"

type decisionSummary struct {
	Total         int            `json:"total"`
	RecordEnabled int            `json:"record_enabled"`
	DecodeEnabled int            `json:"decode_enabled"`
	Reasons       map[string]int `json:"reasons,omitempty"`
}

func summarizeDecisions(decisions []pipeline.SignalDecision) decisionSummary {
	summary := decisionSummary{Reasons: map[string]int{}}
	summary.Total = len(decisions)
	for _, d := range decisions {
		if d.ShouldRecord {
			summary.RecordEnabled++
		}
		if d.ShouldAutoDecode {
			summary.DecodeEnabled++
		}
		reason := d.Reason
		if reason == "" {
			reason = pipeline.DecisionReasonUnspecified
		}
		summary.Reasons[reason]++
	}
	return summary
}

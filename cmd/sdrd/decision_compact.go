package main

import "sdr-wideband-suite/internal/pipeline"

type compactDecision struct {
	ID              int64                        `json:"id"`
	Class           string                       `json:"class,omitempty"`
	Record          bool                         `json:"record"`
	Decode          bool                         `json:"decode"`
	Reason          string                       `json:"reason,omitempty"`
	MonitorBias     float64                      `json:"monitor_bias,omitempty"`
	MonitorDetail   *pipeline.MonitorWindowMatch `json:"monitor_detail,omitempty"`
	RecordWindow    *pipeline.MonitorWindowMatch `json:"record_window,omitempty"`
	DecodeWindow    *pipeline.MonitorWindowMatch `json:"decode_window,omitempty"`
	RecordAdmission *pipeline.PriorityAdmission  `json:"record_admission,omitempty"`
	DecodeAdmission *pipeline.PriorityAdmission  `json:"decode_admission,omitempty"`
	Candidate       pipeline.Candidate           `json:"candidate"`
}

func compactDecisions(decisions []pipeline.SignalDecision) []compactDecision {
	out := make([]compactDecision, 0, len(decisions))
	for _, d := range decisions {
		out = append(out, compactDecision{
			ID:              d.Candidate.ID,
			Class:           d.Class,
			Record:          d.ShouldRecord,
			Decode:          d.ShouldAutoDecode,
			Reason:          d.Reason,
			MonitorBias:     d.MonitorBias,
			MonitorDetail:   d.MonitorDetail,
			RecordWindow:    d.RecordWindow,
			DecodeWindow:    d.DecodeWindow,
			RecordAdmission: d.RecordAdmission,
			DecodeAdmission: d.DecodeAdmission,
			Candidate:       d.Candidate,
		})
	}
	return out
}

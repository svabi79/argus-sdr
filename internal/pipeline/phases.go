package pipeline

import "sdr-wideband-suite/internal/detector"

type SurveillanceResult struct {
	Candidates []Candidate          `json:"candidates"`
	Scheduled  []ScheduledCandidate `json:"scheduled,omitempty"`
	Finished   []detector.Event     `json:"finished"`
	Signals    []detector.Signal    `json:"signals"`
	NoiseFloor float64              `json:"noise_floor"`
	Thresholds []float64            `json:"thresholds,omitempty"`
}

type RefinementResult struct {
	Signals    []detector.Signal   `json:"signals"`
	Decisions  []SignalDecision    `json:"decisions,omitempty"`
	Candidates []Candidate         `json:"candidates,omitempty"`
}

package pipeline

import "sdr-wideband-suite/internal/detector"

type AnalysisLevel struct {
	Name       string  `json:"name"`
	SampleRate int     `json:"sample_rate"`
	FFTSize    int     `json:"fft_size"`
	CenterHz   float64 `json:"center_hz"`
	SpanHz     float64 `json:"span_hz"`
	Source     string  `json:"source,omitempty"`
}

type SurveillanceResult struct {
	Level      AnalysisLevel        `json:"level"`
	Candidates []Candidate          `json:"candidates"`
	Scheduled  []ScheduledCandidate `json:"scheduled,omitempty"`
	Finished   []detector.Event     `json:"finished"`
	Signals    []detector.Signal    `json:"signals"`
	NoiseFloor float64              `json:"noise_floor"`
	Thresholds []float64            `json:"thresholds,omitempty"`
}

type RefinementPlan struct {
	TotalCandidates   int                  `json:"total_candidates"`
	MinCandidateSNRDb float64              `json:"min_candidate_snr_db"`
	Budget            int                  `json:"budget"`
	DroppedBySNR      int                  `json:"dropped_by_snr"`
	DroppedByBudget   int                  `json:"dropped_by_budget"`
	Selected          []ScheduledCandidate `json:"selected,omitempty"`
}

type RefinementInput struct {
	Level      AnalysisLevel        `json:"level"`
	Candidates []Candidate          `json:"candidates,omitempty"`
	Scheduled  []ScheduledCandidate `json:"scheduled,omitempty"`
	Plan       RefinementPlan       `json:"plan,omitempty"`
	Windows    []RefinementWindow   `json:"windows,omitempty"`
	SampleRate int                  `json:"sample_rate"`
	FFTSize    int                  `json:"fft_size"`
	CenterHz   float64              `json:"center_hz"`
	Source     string               `json:"source,omitempty"`
}

type RefinementResult struct {
	Level      AnalysisLevel     `json:"level"`
	Signals    []detector.Signal `json:"signals"`
	Decisions  []SignalDecision  `json:"decisions,omitempty"`
	Candidates []Candidate       `json:"candidates,omitempty"`
}

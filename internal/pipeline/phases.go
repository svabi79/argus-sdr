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
	Level        AnalysisLevel        `json:"level"`
	Levels       []AnalysisLevel      `json:"levels,omitempty"`
	Candidates   []Candidate          `json:"candidates"`
	Scheduled    []ScheduledCandidate `json:"scheduled,omitempty"`
	Finished     []detector.Event     `json:"finished"`
	Signals      []detector.Signal    `json:"signals"`
	NoiseFloor   float64              `json:"noise_floor"`
	Thresholds   []float64            `json:"thresholds,omitempty"`
	DisplayLevel AnalysisLevel        `json:"display_level"`
}

type RefinementPlan struct {
	TotalCandidates   int                  `json:"total_candidates"`
	MinCandidateSNRDb float64              `json:"min_candidate_snr_db"`
	Budget            int                  `json:"budget"`
	MonitorStartHz    float64              `json:"monitor_start_hz,omitempty"`
	MonitorEndHz      float64              `json:"monitor_end_hz,omitempty"`
	MonitorSpanHz     float64              `json:"monitor_span_hz,omitempty"`
	DroppedByMonitor  int                  `json:"dropped_by_monitor"`
	DroppedBySNR      int                  `json:"dropped_by_snr"`
	DroppedByBudget   int                  `json:"dropped_by_budget"`
	PriorityMin       float64              `json:"priority_min,omitempty"`
	PriorityMax       float64              `json:"priority_max,omitempty"`
	PriorityAvg       float64              `json:"priority_avg,omitempty"`
	PriorityCutoff    float64              `json:"priority_cutoff,omitempty"`
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

type RefinementStep struct {
	Input  RefinementInput  `json:"input"`
	Result RefinementResult `json:"result"`
}

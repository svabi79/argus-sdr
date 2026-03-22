package pipeline

import "sdr-wideband-suite/internal/detector"

type AnalysisLevel struct {
	Name       string  `json:"name"`
	Role       string  `json:"role,omitempty"`
	Truth      string  `json:"truth,omitempty"`
	SampleRate int     `json:"sample_rate"`
	FFTSize    int     `json:"fft_size"`
	BinHz      float64 `json:"bin_hz,omitempty"`
	Decimation int     `json:"decimation,omitempty"`
	CenterHz   float64 `json:"center_hz"`
	SpanHz     float64 `json:"span_hz"`
	Source     string  `json:"source,omitempty"`
}

type SurveillanceLevelSpectrum struct {
	Level    AnalysisLevel `json:"level"`
	Spectrum []float64     `json:"spectrum_db,omitempty"`
}

type AnalysisContext struct {
	Surveillance AnalysisLevel   `json:"surveillance,omitempty"`
	Refinement   AnalysisLevel   `json:"refinement,omitempty"`
	Presentation AnalysisLevel   `json:"presentation,omitempty"`
	Detail       AnalysisLevel   `json:"detail,omitempty"`
	Derived      []AnalysisLevel `json:"derived,omitempty"`
}

type SurveillanceLevelSet struct {
	Primary      AnalysisLevel   `json:"primary"`
	Derived      []AnalysisLevel `json:"derived,omitempty"`
	Support      []AnalysisLevel `json:"support,omitempty"`
	Presentation AnalysisLevel   `json:"presentation,omitempty"`
	Detection    []AnalysisLevel `json:"detection,omitempty"`
	All          []AnalysisLevel `json:"all,omitempty"`
}

type SurveillanceResult struct {
	Level           AnalysisLevel               `json:"level"`
	Levels          []AnalysisLevel             `json:"levels,omitempty"`
	LevelSet        SurveillanceLevelSet        `json:"level_set,omitempty"`
	DetectionPolicy SurveillanceDetectionPolicy `json:"detection_policy,omitempty"`
	Candidates      []Candidate                 `json:"candidates"`
	Scheduled       []ScheduledCandidate        `json:"scheduled,omitempty"`
	Finished        []detector.Event            `json:"finished"`
	Signals         []detector.Signal           `json:"signals"`
	NoiseFloor      float64                     `json:"noise_floor"`
	Thresholds      []float64                   `json:"thresholds,omitempty"`
	DisplayLevel    AnalysisLevel               `json:"display_level"`
	Context         AnalysisContext             `json:"context,omitempty"`
	Spectra         []SurveillanceLevelSpectrum `json:"spectra,omitempty"`
}

type RefinementPlan struct {
	TotalCandidates    int                  `json:"total_candidates"`
	MinCandidateSNRDb  float64              `json:"min_candidate_snr_db"`
	Budget             int                  `json:"budget"`
	BudgetSource       string               `json:"budget_source,omitempty"`
	Strategy           string               `json:"strategy,omitempty"`
	StrategyReason     string               `json:"strategy_reason,omitempty"`
	MonitorStartHz     float64              `json:"monitor_start_hz,omitempty"`
	MonitorEndHz       float64              `json:"monitor_end_hz,omitempty"`
	MonitorSpanHz      float64              `json:"monitor_span_hz,omitempty"`
	MonitorWindows     []MonitorWindow      `json:"monitor_windows,omitempty"`
	MonitorWindowStats []MonitorWindowStats `json:"monitor_window_stats,omitempty"`
	DroppedByMonitor   int                  `json:"dropped_by_monitor"`
	DroppedBySNR       int                  `json:"dropped_by_snr"`
	DroppedByBudget    int                  `json:"dropped_by_budget"`
	ScoreModel         RefinementScoreModel `json:"score_model,omitempty"`
	PriorityMin        float64              `json:"priority_min,omitempty"`
	PriorityMax        float64              `json:"priority_max,omitempty"`
	PriorityAvg        float64              `json:"priority_avg,omitempty"`
	PriorityCutoff     float64              `json:"priority_cutoff,omitempty"`
	Ranked             []ScheduledCandidate `json:"ranked,omitempty"`
	Selected           []ScheduledCandidate `json:"selected,omitempty"`
	WorkItems          []RefinementWorkItem `json:"work_items,omitempty"`
}

type RefinementRequest struct {
	Strategy   string  `json:"strategy,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	SpanHintHz float64 `json:"span_hint_hz,omitempty"`
}

type RefinementInput struct {
	Level      AnalysisLevel        `json:"level"`
	Detail     AnalysisLevel        `json:"detail,omitempty"`
	Context    AnalysisContext      `json:"context,omitempty"`
	Request    RefinementRequest    `json:"request,omitempty"`
	Budgets    BudgetModel          `json:"budgets,omitempty"`
	Admission  RefinementAdmission  `json:"admission,omitempty"`
	Candidates []Candidate          `json:"candidates,omitempty"`
	Scheduled  []ScheduledCandidate `json:"scheduled,omitempty"`
	WorkItems  []RefinementWorkItem `json:"work_items,omitempty"`
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

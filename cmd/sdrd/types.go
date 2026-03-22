package main

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/demod/gpudemod"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/pipeline"
	"sdr-wideband-suite/internal/sdr"
)

type SpectrumDebug struct {
	Thresholds     []float64                `json:"thresholds,omitempty"`
	NoiseFloor     float64                  `json:"noise_floor,omitempty"`
	Scores         []map[string]any         `json:"scores,omitempty"`
	RefinementPlan *pipeline.RefinementPlan `json:"refinement_plan,omitempty"`
	Windows        *RefinementWindowStats   `json:"refinement_windows,omitempty"`
	Refinement     *RefinementDebug         `json:"refinement,omitempty"`
	Decisions      *DecisionDebug           `json:"decisions,omitempty"`
}

type RefinementWindowStats struct {
	Count   int            `json:"count"`
	MinSpan float64        `json:"min_span_hz"`
	MaxSpan float64        `json:"max_span_hz"`
	AvgSpan float64        `json:"avg_span_hz"`
	Sources map[string]int `json:"sources,omitempty"`
}

type RefinementDebug struct {
	Plan        *pipeline.RefinementPlan      `json:"plan,omitempty"`
	Request     *pipeline.RefinementRequest   `json:"request,omitempty"`
	WorkItems   []pipeline.RefinementWorkItem `json:"work_items,omitempty"`
	Windows     *RefinementWindowStats        `json:"windows,omitempty"`
	Arbitration *ArbitrationSnapshot          `json:"arbitration,omitempty"`
}

type DecisionDebug struct {
	Summary decisionSummary   `json:"summary"`
	Items   []compactDecision `json:"items,omitempty"`
}

type ArbitrationSnapshot struct {
	Budgets             *pipeline.BudgetModel         `json:"budgets,omitempty"`
	HoldPolicy          *pipeline.HoldPolicy          `json:"hold_policy,omitempty"`
	RefinementPlan      *pipeline.RefinementPlan      `json:"refinement_plan,omitempty"`
	RefinementAdmission *pipeline.RefinementAdmission `json:"refinement_admission,omitempty"`
	Queue               pipeline.DecisionQueueStats   `json:"queue,omitempty"`
	DecisionSummary     decisionSummary               `json:"decision_summary,omitempty"`
	DecisionItems       []compactDecision             `json:"decision_items,omitempty"`
}

type SpectrumFrame struct {
	Timestamp int64             `json:"ts"`
	CenterHz  float64           `json:"center_hz"`
	SampleHz  int               `json:"sample_rate"`
	FFTSize   int               `json:"fft_size"`
	Spectrum  []float64         `json:"spectrum_db"`
	Signals   []detector.Signal `json:"signals"`
	Debug     *SpectrumDebug    `json:"debug,omitempty"`
}

type client struct {
	conn      *websocket.Conn
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once

	// Per-client settings (set via initial config message)
	binary    bool // send binary spectrum frames instead of JSON
	maxBins   int  // target bin count (0 = full resolution)
	targetFps int  // target frame rate (0 = full rate)
	frameSkip int  // skip counter: send every N-th frame
	frameN    int  // current frame counter
}

type hub struct {
	mu        sync.Mutex
	clients   map[*client]struct{}
	frameCnt  int64
	lastLogTs time.Time
}

type gpuStatus struct {
	mu        sync.RWMutex
	Available bool   `json:"available"`
	Active    bool   `json:"active"`
	Error     string `json:"error"`
}

type signalSnapshot struct {
	mu      sync.RWMutex
	signals []detector.Signal
}

type sourceManager struct {
	mu        sync.RWMutex
	src       sdr.Source
	newSource func(cfg config.Config) (sdr.Source, error)
}

type extractionManager struct {
	mu         sync.Mutex
	runner     *gpudemod.BatchRunner
	maxSamples int
}

type dspUpdate struct {
	cfg       config.Config
	det       *detector.Detector
	window    []float64
	dcBlock   bool
	iqBalance bool
	useGPUFFT bool
}

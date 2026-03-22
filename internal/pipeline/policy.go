package pipeline

import "sdr-wideband-suite/internal/config"

type Policy struct {
	Mode                         string                      `json:"mode"`
	Profile                      string                      `json:"profile,omitempty"`
	Intent                       string                      `json:"intent"`
	MonitorCenterHz              float64                     `json:"monitor_center_hz,omitempty"`
	MonitorStartHz               float64                     `json:"monitor_start_hz,omitempty"`
	MonitorEndHz                 float64                     `json:"monitor_end_hz,omitempty"`
	MonitorSpanHz                float64                     `json:"monitor_span_hz,omitempty"`
	SignalPriorities             []string                    `json:"signal_priorities,omitempty"`
	AutoRecordClasses            []string                    `json:"auto_record_classes,omitempty"`
	AutoDecodeClasses            []string                    `json:"auto_decode_classes,omitempty"`
	SurveillanceFFTSize          int                         `json:"surveillance_fft_size"`
	SurveillanceFPS              int                         `json:"surveillance_fps"`
	DisplayBins                  int                         `json:"display_bins"`
	DisplayFPS                   int                         `json:"display_fps"`
	SurveillanceStrategy         string                      `json:"surveillance_strategy"`
	SurveillanceDerivedDetection string                      `json:"surveillance_derived_detection"`
	RefinementStrategy           string                      `json:"refinement_strategy,omitempty"`
	RefinementEnabled            bool                        `json:"refinement_enabled"`
	MaxRefinementJobs            int                         `json:"max_refinement_jobs"`
	RefinementMaxConcurrent      int                         `json:"refinement_max_concurrent"`
	RefinementDetailFFTSize      int                         `json:"refinement_detail_fft_size"`
	MinCandidateSNRDb            float64                     `json:"min_candidate_snr_db"`
	RefinementMinSpanHz          float64                     `json:"refinement_min_span_hz"`
	RefinementMaxSpanHz          float64                     `json:"refinement_max_span_hz"`
	RefinementAutoSpan           bool                        `json:"refinement_auto_span"`
	PreferGPU                    bool                        `json:"prefer_gpu"`
	MaxRecordingStreams          int                         `json:"max_recording_streams"`
	MaxDecodeJobs                int                         `json:"max_decode_jobs"`
	DecisionHoldMs               int                         `json:"decision_hold_ms"`
	SurveillanceDetection        SurveillanceDetectionPolicy `json:"surveillance_detection,omitempty"`
}

func PolicyFromConfig(cfg config.Config) Policy {
	detailFFT := cfg.Refinement.DetailFFTSize
	if detailFFT <= 0 {
		detailFFT = cfg.Surveillance.AnalysisFFTSize
	}
	p := Policy{
		Mode:                         cfg.Pipeline.Mode,
		Profile:                      cfg.Pipeline.Profile,
		Intent:                       cfg.Pipeline.Goals.Intent,
		MonitorCenterHz:              cfg.CenterHz,
		MonitorStartHz:               cfg.Pipeline.Goals.MonitorStartHz,
		MonitorEndHz:                 cfg.Pipeline.Goals.MonitorEndHz,
		MonitorSpanHz:                cfg.Pipeline.Goals.MonitorSpanHz,
		SignalPriorities:             append([]string(nil), cfg.Pipeline.Goals.SignalPriorities...),
		AutoRecordClasses:            append([]string(nil), cfg.Pipeline.Goals.AutoRecordClasses...),
		AutoDecodeClasses:            append([]string(nil), cfg.Pipeline.Goals.AutoDecodeClasses...),
		SurveillanceFFTSize:          cfg.Surveillance.AnalysisFFTSize,
		SurveillanceFPS:              cfg.Surveillance.FrameRate,
		DisplayBins:                  cfg.Surveillance.DisplayBins,
		DisplayFPS:                   cfg.Surveillance.DisplayFPS,
		SurveillanceStrategy:         cfg.Surveillance.Strategy,
		SurveillanceDerivedDetection: cfg.Surveillance.DerivedDetection,
		RefinementEnabled:            cfg.Refinement.Enabled,
		MaxRefinementJobs:            cfg.Resources.MaxRefinementJobs,
		RefinementMaxConcurrent:      cfg.Refinement.MaxConcurrent,
		RefinementDetailFFTSize:      detailFFT,
		MinCandidateSNRDb:            cfg.Refinement.MinCandidateSNRDb,
		RefinementMinSpanHz:          cfg.Refinement.MinSpanHz,
		RefinementMaxSpanHz:          cfg.Refinement.MaxSpanHz,
		RefinementAutoSpan:           config.BoolValue(cfg.Refinement.AutoSpan, true),
		PreferGPU:                    cfg.Resources.PreferGPU,
		MaxRecordingStreams:          cfg.Resources.MaxRecordingStreams,
		MaxDecodeJobs:                cfg.Resources.MaxDecodeJobs,
		DecisionHoldMs:               cfg.Resources.DecisionHoldMs,
	}
	p.RefinementStrategy, _ = refinementStrategy(p)
	p.SurveillanceDetection = SurveillanceDetectionPolicyFromPolicy(p)
	if p.MonitorSpanHz <= 0 && p.MonitorStartHz != 0 && p.MonitorEndHz != 0 && p.MonitorEndHz > p.MonitorStartHz {
		p.MonitorSpanHz = p.MonitorEndHz - p.MonitorStartHz
	}
	return p
}

func ApplyNamedProfile(cfg *config.Config, name string) {
	if cfg == nil || name == "" {
		return
	}
	cfg.Pipeline.Profile = name
	if prof, ok := ResolveProfile(*cfg, name); ok {
		MergeProfile(cfg, prof)
	}
	switch name {
	case "legacy":
		cfg.Pipeline.Mode = "legacy"
		cfg.Pipeline.Goals.Intent = "general-monitoring"
		cfg.Surveillance.Strategy = "single-resolution"
		cfg.Refinement.Enabled = true
		if cfg.Resources.MaxRefinementJobs <= 0 {
			cfg.Resources.MaxRefinementJobs = 8
		}
	case "wideband-balanced":
		cfg.Pipeline.Mode = "wideband-balanced"
		cfg.Pipeline.Goals.Intent = "wideband-surveillance"
		cfg.Surveillance.Strategy = "multi-resolution"
		if cfg.Surveillance.AnalysisFFTSize < 4096 {
			cfg.Surveillance.AnalysisFFTSize = 4096
		}
		if cfg.FrameRate < 12 {
			cfg.FrameRate = 12
		}
		if cfg.Surveillance.FrameRate < 12 {
			cfg.Surveillance.FrameRate = 12
		}
		cfg.Refinement.Enabled = true
		if cfg.Refinement.MaxConcurrent < 16 {
			cfg.Refinement.MaxConcurrent = 16
		}
		if cfg.Resources.MaxRefinementJobs < 16 {
			cfg.Resources.MaxRefinementJobs = 16
		}
		if cfg.Refinement.MinSpanHz <= 0 {
			cfg.Refinement.MinSpanHz = 4000
		}
		if cfg.Refinement.MaxSpanHz <= 0 {
			cfg.Refinement.MaxSpanHz = 200000
		}
		if len(cfg.Pipeline.Goals.SignalPriorities) == 0 {
			cfg.Pipeline.Goals.SignalPriorities = []string{"digital", "wfm"}
		}
		cfg.Resources.PreferGPU = true
	case "wideband-aggressive":
		cfg.Pipeline.Mode = "wideband-aggressive"
		cfg.Pipeline.Goals.Intent = "high-density-wideband-surveillance"
		cfg.Surveillance.Strategy = "multi-resolution"
		if cfg.Surveillance.AnalysisFFTSize < 8192 {
			cfg.Surveillance.AnalysisFFTSize = 8192
		}
		if cfg.FrameRate < 10 {
			cfg.FrameRate = 10
		}
		if cfg.Surveillance.FrameRate < 10 {
			cfg.Surveillance.FrameRate = 10
		}
		cfg.Refinement.Enabled = true
		if cfg.Refinement.MaxConcurrent < 32 {
			cfg.Refinement.MaxConcurrent = 32
		}
		if cfg.Resources.MaxRefinementJobs < 32 {
			cfg.Resources.MaxRefinementJobs = 32
		}
		if cfg.Refinement.MinSpanHz <= 0 {
			cfg.Refinement.MinSpanHz = 6000
		}
		if cfg.Refinement.MaxSpanHz <= 0 {
			cfg.Refinement.MaxSpanHz = 250000
		}
		if len(cfg.Pipeline.Goals.SignalPriorities) == 0 {
			cfg.Pipeline.Goals.SignalPriorities = []string{"digital", "wfm", "trunk"}
		}
		cfg.Resources.PreferGPU = true
	case "archive":
		cfg.Pipeline.Mode = "archive"
		cfg.Pipeline.Goals.Intent = "archive-and-triage"
		cfg.Refinement.Enabled = true
		if cfg.Refinement.MaxConcurrent < 12 {
			cfg.Refinement.MaxConcurrent = 12
		}
		if cfg.Resources.MaxRefinementJobs < 12 {
			cfg.Resources.MaxRefinementJobs = 12
		}
		if cfg.Resources.MaxRecordingStreams < 24 {
			cfg.Resources.MaxRecordingStreams = 24
		}
		if cfg.Resources.MaxDecodeJobs < 12 {
			cfg.Resources.MaxDecodeJobs = 12
		}
		if len(cfg.Pipeline.Goals.SignalPriorities) == 0 {
			cfg.Pipeline.Goals.SignalPriorities = []string{"wfm", "nfm", "digital"}
		}
		if !cfg.Recorder.Enabled {
			cfg.Recorder.Enabled = true
		}
	case "digital-hunting":
		cfg.Pipeline.Mode = "digital-hunting"
		cfg.Pipeline.Goals.Intent = "digital-surveillance"
		cfg.Surveillance.Strategy = "multi-resolution"
		if cfg.Surveillance.AnalysisFFTSize < 4096 {
			cfg.Surveillance.AnalysisFFTSize = 4096
		}
		if cfg.FrameRate < 12 {
			cfg.FrameRate = 12
		}
		if cfg.Surveillance.FrameRate < 12 {
			cfg.Surveillance.FrameRate = 12
		}
		cfg.Refinement.Enabled = true
		if cfg.Refinement.MaxConcurrent < 16 {
			cfg.Refinement.MaxConcurrent = 16
		}
		if cfg.Resources.MaxRefinementJobs < 16 {
			cfg.Resources.MaxRefinementJobs = 16
		}
		if cfg.Refinement.MinSpanHz <= 0 {
			cfg.Refinement.MinSpanHz = 3000
		}
		if cfg.Refinement.MaxSpanHz <= 0 {
			cfg.Refinement.MaxSpanHz = 120000
		}
		if len(cfg.Pipeline.Goals.SignalPriorities) == 0 {
			cfg.Pipeline.Goals.SignalPriorities = []string{"ft8", "wspr", "fsk", "psk", "dmr"}
		}
		cfg.Resources.PreferGPU = true
	}
	if cfg.Pipeline.Goals.MonitorSpanHz <= 0 && cfg.SampleRate > 0 {
		cfg.Pipeline.Goals.MonitorSpanHz = float64(cfg.SampleRate)
	}
	if cfg.Resources.MaxDecodeJobs <= 0 {
		cfg.Resources.MaxDecodeJobs = cfg.Resources.MaxRecordingStreams
	}
	if cfg.Refinement.DetailFFTSize <= 0 {
		cfg.Refinement.DetailFFTSize = cfg.Surveillance.AnalysisFFTSize
	}
	cfg.FFTSize = cfg.Surveillance.AnalysisFFTSize
}

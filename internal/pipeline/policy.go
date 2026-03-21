package pipeline

import "sdr-wideband-suite/internal/config"

type Policy struct {
	Mode                    string   `json:"mode"`
	Intent                  string   `json:"intent"`
	MonitorStartHz          float64  `json:"monitor_start_hz,omitempty"`
	MonitorEndHz            float64  `json:"monitor_end_hz,omitempty"`
	MonitorSpanHz           float64  `json:"monitor_span_hz,omitempty"`
	SignalPriorities        []string `json:"signal_priorities,omitempty"`
	AutoRecordClasses       []string `json:"auto_record_classes,omitempty"`
	AutoDecodeClasses       []string `json:"auto_decode_classes,omitempty"`
	SurveillanceFFTSize     int      `json:"surveillance_fft_size"`
	SurveillanceFPS         int      `json:"surveillance_fps"`
	DisplayBins             int      `json:"display_bins"`
	DisplayFPS              int      `json:"display_fps"`
	RefinementEnabled       bool     `json:"refinement_enabled"`
	MaxRefinementJobs       int      `json:"max_refinement_jobs"`
	RefinementMaxConcurrent int      `json:"refinement_max_concurrent"`
	MinCandidateSNRDb       float64  `json:"min_candidate_snr_db"`
	RefinementMinSpanHz     float64  `json:"refinement_min_span_hz"`
	RefinementMaxSpanHz     float64  `json:"refinement_max_span_hz"`
	RefinementAutoSpan      bool     `json:"refinement_auto_span"`
	PreferGPU               bool     `json:"prefer_gpu"`
	MaxDecodeJobs           int      `json:"max_decode_jobs"`
}

func PolicyFromConfig(cfg config.Config) Policy {
	return Policy{
		Mode:                    cfg.Pipeline.Mode,
		Intent:                  cfg.Pipeline.Goals.Intent,
		MonitorStartHz:          cfg.Pipeline.Goals.MonitorStartHz,
		MonitorEndHz:            cfg.Pipeline.Goals.MonitorEndHz,
		MonitorSpanHz:           cfg.Pipeline.Goals.MonitorSpanHz,
		SignalPriorities:        append([]string(nil), cfg.Pipeline.Goals.SignalPriorities...),
		AutoRecordClasses:       append([]string(nil), cfg.Pipeline.Goals.AutoRecordClasses...),
		AutoDecodeClasses:       append([]string(nil), cfg.Pipeline.Goals.AutoDecodeClasses...),
		SurveillanceFFTSize:     cfg.Surveillance.AnalysisFFTSize,
		SurveillanceFPS:         cfg.Surveillance.FrameRate,
		DisplayBins:             cfg.Surveillance.DisplayBins,
		DisplayFPS:              cfg.Surveillance.DisplayFPS,
		RefinementEnabled:       cfg.Refinement.Enabled,
		MaxRefinementJobs:       cfg.Resources.MaxRefinementJobs,
		RefinementMaxConcurrent: cfg.Refinement.MaxConcurrent,
		MinCandidateSNRDb:       cfg.Refinement.MinCandidateSNRDb,
		RefinementMinSpanHz:     cfg.Refinement.MinSpanHz,
		RefinementMaxSpanHz:     cfg.Refinement.MaxSpanHz,
		RefinementAutoSpan:      config.BoolValue(cfg.Refinement.AutoSpan, true),
		PreferGPU:               cfg.Resources.PreferGPU,
		MaxDecodeJobs:           cfg.Resources.MaxDecodeJobs,
	}
}

func ApplyNamedProfile(cfg *config.Config, name string) {
	if cfg == nil || name == "" {
		return
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
		cfg.Surveillance.Strategy = "single-resolution"
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
		cfg.Resources.PreferGPU = true
	case "wideband-aggressive":
		cfg.Pipeline.Mode = "wideband-aggressive"
		cfg.Pipeline.Goals.Intent = "high-density-wideband-surveillance"
		cfg.Surveillance.Strategy = "single-resolution"
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
		if !cfg.Recorder.Enabled {
			cfg.Recorder.Enabled = true
		}
	}
	cfg.FFTSize = cfg.Surveillance.AnalysisFFTSize
}

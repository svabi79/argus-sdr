package pipeline

import "sdr-wideband-suite/internal/config"

type Policy struct {
	Mode                string  `json:"mode"`
	SurveillanceFFTSize int     `json:"surveillance_fft_size"`
	SurveillanceFPS     int     `json:"surveillance_fps"`
	RefinementEnabled   bool    `json:"refinement_enabled"`
	MaxRefinementJobs   int     `json:"max_refinement_jobs"`
	MinCandidateSNRDb   float64 `json:"min_candidate_snr_db"`
	PreferGPU           bool    `json:"prefer_gpu"`
}

func PolicyFromConfig(cfg config.Config) Policy {
	return Policy{
		Mode:                cfg.Pipeline.Mode,
		SurveillanceFFTSize: cfg.Surveillance.AnalysisFFTSize,
		SurveillanceFPS:     cfg.Surveillance.FrameRate,
		RefinementEnabled:   cfg.Refinement.Enabled,
		MaxRefinementJobs:   cfg.Resources.MaxRefinementJobs,
		MinCandidateSNRDb:   cfg.Refinement.MinCandidateSNRDb,
		PreferGPU:           cfg.Resources.PreferGPU,
	}
}

func ApplyNamedProfile(cfg *config.Config, name string) {
	if cfg == nil || name == "" {
		return
	}
	switch name {
	case "legacy":
		cfg.Pipeline.Mode = "legacy"
		cfg.Surveillance.Strategy = "single-resolution"
		cfg.Refinement.Enabled = true
		if cfg.Resources.MaxRefinementJobs <= 0 {
			cfg.Resources.MaxRefinementJobs = 8
		}
	case "wideband-balanced":
		cfg.Pipeline.Mode = "wideband-balanced"
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
		cfg.Resources.PreferGPU = true
	case "wideband-aggressive":
		cfg.Pipeline.Mode = "wideband-aggressive"
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
		cfg.Resources.PreferGPU = true
	case "archive":
		cfg.Pipeline.Mode = "archive"
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

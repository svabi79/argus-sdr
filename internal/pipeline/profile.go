package pipeline

import "sdr-wideband-suite/internal/config"

func ResolveProfile(cfg config.Config, name string) (config.ProfileConfig, bool) {
	for _, p := range cfg.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return config.ProfileConfig{}, false
}

func MergeProfile(cfg *config.Config, profile config.ProfileConfig) {
	if cfg == nil {
		return
	}
	if profile.Pipeline != nil {
		cfg.Pipeline = *profile.Pipeline
	}
	if profile.Surveillance != nil {
		cfg.Surveillance = *profile.Surveillance
	}
	if profile.Refinement != nil {
		cfg.Refinement = *profile.Refinement
	}
	if profile.Resources != nil {
		cfg.Resources = *profile.Resources
	}
	if cfg.Surveillance.AnalysisFFTSize > 0 {
		cfg.FFTSize = cfg.Surveillance.AnalysisFFTSize
	}
	if cfg.Surveillance.FrameRate > 0 {
		cfg.FrameRate = cfg.Surveillance.FrameRate
	}
}

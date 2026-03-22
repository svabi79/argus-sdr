package main

import (
	"sort"

	"sdr-wideband-suite/internal/pipeline"
)

type SurveillanceLevelSummary struct {
	Name         string  `json:"name"`
	Role         string  `json:"role,omitempty"`
	Truth        string  `json:"truth,omitempty"`
	SampleRate   int     `json:"sample_rate,omitempty"`
	FFTSize      int     `json:"fft_size,omitempty"`
	BinHz        float64 `json:"bin_hz,omitempty"`
	Decimation   int     `json:"decimation,omitempty"`
	SpanHz       float64 `json:"span_hz,omitempty"`
	CenterHz     float64 `json:"center_hz,omitempty"`
	Source       string  `json:"source,omitempty"`
	SpectrumBins int     `json:"spectrum_bins,omitempty"`
}

type CandidateEvidenceSummary struct {
	Level      string `json:"level"`
	Provenance string `json:"provenance,omitempty"`
	Count      int    `json:"count"`
}

func buildSurveillanceLevelSummaries(set pipeline.SurveillanceLevelSet, spectra []pipeline.SurveillanceLevelSpectrum) map[string]SurveillanceLevelSummary {
	if set.Primary.Name == "" && len(set.Derived) == 0 && set.Presentation.Name == "" && len(set.All) == 0 {
		return nil
	}
	bins := map[string]int{}
	for _, spec := range spectra {
		if spec.Level.Name == "" || len(spec.Spectrum) == 0 {
			continue
		}
		bins[spec.Level.Name] = len(spec.Spectrum)
	}
	levels := set.All
	if len(levels) == 0 {
		if set.Primary.Name != "" {
			levels = append(levels, set.Primary)
		}
		if len(set.Derived) > 0 {
			levels = append(levels, set.Derived...)
		}
		if set.Presentation.Name != "" {
			levels = append(levels, set.Presentation)
		}
	}
	out := make(map[string]SurveillanceLevelSummary, len(levels))
	for _, level := range levels {
		name := level.Name
		if name == "" {
			continue
		}
		binHz := level.BinHz
		if binHz == 0 && level.SampleRate > 0 && level.FFTSize > 0 {
			binHz = float64(level.SampleRate) / float64(level.FFTSize)
		}
		out[name] = SurveillanceLevelSummary{
			Name:         name,
			Role:         level.Role,
			Truth:        level.Truth,
			SampleRate:   level.SampleRate,
			FFTSize:      level.FFTSize,
			BinHz:        binHz,
			Decimation:   level.Decimation,
			SpanHz:       level.SpanHz,
			CenterHz:     level.CenterHz,
			Source:       level.Source,
			SpectrumBins: bins[name],
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildCandidateSourceSummary(candidates []pipeline.Candidate) map[string]int {
	if len(candidates) == 0 {
		return nil
	}
	out := map[string]int{}
	for _, cand := range candidates {
		if cand.Source == "" {
			continue
		}
		out[cand.Source]++
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildCandidateEvidenceSummary(candidates []pipeline.Candidate) []CandidateEvidenceSummary {
	if len(candidates) == 0 {
		return nil
	}
	type key struct {
		level      string
		provenance string
	}
	counts := map[key]int{}
	for _, cand := range candidates {
		for _, ev := range cand.Evidence {
			name := ev.Level.Name
			if name == "" {
				name = "unknown"
			}
			k := key{level: name, provenance: ev.Provenance}
			counts[k]++
		}
	}
	if len(counts) == 0 {
		return nil
	}
	out := make([]CandidateEvidenceSummary, 0, len(counts))
	for k, v := range counts {
		out = append(out, CandidateEvidenceSummary{Level: k.level, Provenance: k.provenance, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			if out[i].Level == out[j].Level {
				return out[i].Provenance < out[j].Provenance
			}
			return out[i].Level < out[j].Level
		}
		return out[i].Count > out[j].Count
	})
	return out
}

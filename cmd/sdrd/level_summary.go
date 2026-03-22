package main

import (
	"sort"
	"strings"

	"sdr-wideband-suite/internal/pipeline"
)

type SurveillanceLevelSummary struct {
	Name         string  `json:"name"`
	Role         string  `json:"role,omitempty"`
	Truth        string  `json:"truth,omitempty"`
	Kind         string  `json:"kind,omitempty"`
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
	Role       string `json:"role,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Provenance string `json:"provenance,omitempty"`
	Count      int    `json:"count"`
}

type CandidateEvidenceStateSummary struct {
	Total               int `json:"total"`
	WithEvidence        int `json:"with_evidence"`
	Fused               int `json:"fused"`
	MultiLevelConfirmed int `json:"multi_level_confirmed"`
	DerivedOnly         int `json:"derived_only"`
	SupportOnly         int `json:"support_only"`
	PrimaryPresent      int `json:"primary_present"`
	DerivedPresent      int `json:"derived_present"`
	SupportPresent      int `json:"support_present"`
	PrimaryOnly         int `json:"primary_only"`
}

type CandidateWindowSummary struct {
	Index        int     `json:"index"`
	Label        string  `json:"label,omitempty"`
	Source       string  `json:"source,omitempty"`
	StartHz      float64 `json:"start_hz,omitempty"`
	EndHz        float64 `json:"end_hz,omitempty"`
	CenterHz     float64 `json:"center_hz,omitempty"`
	SpanHz       float64 `json:"span_hz,omitempty"`
	Priority     float64 `json:"priority,omitempty"`
	PriorityBias float64 `json:"priority_bias,omitempty"`
	AutoRecord   bool    `json:"auto_record,omitempty"`
	AutoDecode   bool    `json:"auto_decode,omitempty"`
	Candidates   int     `json:"candidates"`
}

func buildSurveillanceLevelSummaries(set pipeline.SurveillanceLevelSet, spectra []pipeline.SurveillanceLevelSpectrum) map[string]SurveillanceLevelSummary {
	if set.Primary.Name == "" && len(set.Derived) == 0 && len(set.Support) == 0 && set.Presentation.Name == "" && len(set.All) == 0 {
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
		if len(set.Support) > 0 {
			levels = append(levels, set.Support...)
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
		kind := evidenceKind(level)
		out[name] = SurveillanceLevelSummary{
			Name:         name,
			Role:         level.Role,
			Truth:        level.Truth,
			Kind:         kind,
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
		role       string
		kind       string
		provenance string
	}
	counts := map[key]int{}
	for _, cand := range candidates {
		for _, ev := range cand.Evidence {
			name := ev.Level.Name
			if name == "" {
				name = "unknown"
			}
			role := strings.TrimSpace(ev.Level.Role)
			kind := evidenceKind(ev.Level)
			k := key{level: name, role: role, kind: kind, provenance: ev.Provenance}
			counts[k]++
		}
	}
	if len(counts) == 0 {
		return nil
	}
	out := make([]CandidateEvidenceSummary, 0, len(counts))
	for k, v := range counts {
		out = append(out, CandidateEvidenceSummary{Level: k.level, Role: k.role, Kind: k.kind, Provenance: k.provenance, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			if out[i].Level == out[j].Level {
				if out[i].Kind == out[j].Kind {
					return out[i].Provenance < out[j].Provenance
				}
				return out[i].Kind < out[j].Kind
			}
			return out[i].Level < out[j].Level
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func buildCandidateEvidenceStateSummary(candidates []pipeline.Candidate) *CandidateEvidenceStateSummary {
	if len(candidates) == 0 {
		return nil
	}
	summary := CandidateEvidenceStateSummary{Total: len(candidates)}
	for _, cand := range candidates {
		state := pipeline.CandidateEvidenceStateFor(cand)
		if state.TotalLevelEntries == 0 {
			continue
		}
		summary.WithEvidence++
		if state.Fused {
			summary.Fused++
		}
		if state.MultiLevelConfirmed {
			summary.MultiLevelConfirmed++
		}
		if state.DerivedOnly {
			summary.DerivedOnly++
		}
		if state.SupportOnly {
			summary.SupportOnly++
		}
		if state.PrimaryLevelCount > 0 {
			summary.PrimaryPresent++
		}
		if state.DerivedLevelCount > 0 {
			summary.DerivedPresent++
		}
		if state.SupportLevelCount > 0 {
			summary.SupportPresent++
		}
		if state.PrimaryLevelCount > 0 && state.DerivedLevelCount == 0 {
			summary.PrimaryOnly++
		}
	}
	if summary.WithEvidence == 0 {
		return nil
	}
	return &summary
}

func buildCandidateWindowSummary(candidates []pipeline.Candidate, windows []pipeline.MonitorWindow) []CandidateWindowSummary {
	if len(windows) == 0 {
		return nil
	}
	out := make([]CandidateWindowSummary, 0, len(windows))
	index := make(map[int]int, len(windows))
	for _, win := range windows {
		entry := CandidateWindowSummary{
			Index:        win.Index,
			Label:        win.Label,
			Source:       win.Source,
			StartHz:      win.StartHz,
			EndHz:        win.EndHz,
			CenterHz:     win.CenterHz,
			SpanHz:       win.SpanHz,
			Priority:     win.Priority,
			PriorityBias: win.PriorityBias,
			AutoRecord:   win.AutoRecord,
			AutoDecode:   win.AutoDecode,
		}
		index[win.Index] = len(out)
		out = append(out, entry)
	}
	totalCandidates := 0
	for _, cand := range candidates {
		matches := cand.MonitorMatches
		if len(matches) == 0 {
			matches = pipeline.MonitorWindowMatchesForCandidate(windows, cand)
		}
		for _, match := range matches {
			idx, ok := index[match.Index]
			if !ok {
				continue
			}
			out[idx].Candidates++
			totalCandidates++
		}
	}
	if totalCandidates == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Index < out[j].Index
	})
	return out
}

func evidenceKind(level pipeline.AnalysisLevel) string {
	if pipeline.IsPresentationLevel(level) {
		return "presentation"
	}
	if pipeline.IsSupportLevel(level) {
		return "support"
	}
	if pipeline.IsDetectionLevel(level) {
		return "detection"
	}
	return "unknown"
}

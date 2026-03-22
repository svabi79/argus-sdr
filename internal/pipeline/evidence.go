package pipeline

import (
	"fmt"
	"sort"
	"strings"
)

// CandidateEvidenceState summarizes fused evidence semantics for a candidate.
type CandidateEvidenceState struct {
	TotalLevelEntries       int      `json:"total_level_entries"`
	LevelCount              int      `json:"level_count"`
	DetectionLevelCount     int      `json:"detection_level_count"`
	PrimaryLevelCount       int      `json:"primary_level_count,omitempty"`
	DerivedLevelCount       int      `json:"derived_level_count,omitempty"`
	PresentationLevelCount  int      `json:"presentation_level_count,omitempty"`
	Levels                  []string `json:"levels,omitempty"`
	Provenance              []string `json:"provenance,omitempty"`
	Fused                   bool     `json:"fused,omitempty"`
	DerivedOnly             bool     `json:"derived_only,omitempty"`
	MultiLevelConfirmed     bool     `json:"multi_level_confirmed,omitempty"`
	MultiLevelConfirmedHint string   `json:"multi_level_confirmed_hint,omitempty"`
}

// EvidenceScoreDetails explains how evidence influenced refinement scoring.
type EvidenceScoreDetails struct {
	RawScore            float64 `json:"raw_score"`
	Weight              float64 `json:"weight"`
	WeightedScore       float64 `json:"weighted_score"`
	DetectionLevels     int     `json:"detection_levels"`
	PrimaryLevels       int     `json:"primary_levels,omitempty"`
	DerivedLevels       int     `json:"derived_levels,omitempty"`
	ProvenanceCount     int     `json:"provenance_count,omitempty"`
	DerivedOnly         bool    `json:"derived_only,omitempty"`
	MultiLevelConfirmed bool    `json:"multi_level_confirmed,omitempty"`
	MultiLevelBonus     float64 `json:"multi_level_bonus,omitempty"`
	ProvenanceBonus     float64 `json:"provenance_bonus,omitempty"`
	DerivedPenalty      float64 `json:"derived_penalty,omitempty"`
	StrategyBias        float64 `json:"strategy_bias,omitempty"`
}

// IsPresentationLevel reports whether a level is intended only for presentation.
func IsPresentationLevel(level AnalysisLevel) bool {
	role := strings.ToLower(strings.TrimSpace(level.Role))
	truth := strings.ToLower(strings.TrimSpace(level.Truth))
	name := strings.ToLower(strings.TrimSpace(level.Name))
	if strings.Contains(role, "presentation") || strings.Contains(truth, "presentation") {
		return true
	}
	return strings.Contains(name, "presentation") || strings.Contains(name, "display")
}

// IsDetectionLevel reports whether a level is intended for detection/analysis.
func IsDetectionLevel(level AnalysisLevel) bool {
	if IsPresentationLevel(level) {
		return false
	}
	role := strings.ToLower(strings.TrimSpace(level.Role))
	truth := strings.ToLower(strings.TrimSpace(level.Truth))
	name := strings.ToLower(strings.TrimSpace(level.Name))
	if strings.Contains(truth, "surveillance") {
		return true
	}
	if role == "surveillance" || strings.HasPrefix(role, "surveillance-") {
		return true
	}
	return strings.Contains(name, "surveillance")
}

func isPrimarySurveillanceLevel(level AnalysisLevel) bool {
	role := strings.ToLower(strings.TrimSpace(level.Role))
	name := strings.ToLower(strings.TrimSpace(level.Name))
	return role == "surveillance" || name == "surveillance"
}

func isDerivedSurveillanceLevel(level AnalysisLevel) bool {
	role := strings.ToLower(strings.TrimSpace(level.Role))
	name := strings.ToLower(strings.TrimSpace(level.Name))
	if strings.HasPrefix(role, "surveillance-") && role != "surveillance" {
		return true
	}
	if strings.HasPrefix(name, "surveillance-") && name != "surveillance" {
		return true
	}
	return strings.Contains(role, "lowres") || strings.Contains(name, "lowres") || strings.Contains(name, "derived")
}

func evidenceLevelKey(level AnalysisLevel) string {
	if level.Name != "" {
		return level.Name
	}
	if level.SampleRate > 0 && level.FFTSize > 0 {
		return fmt.Sprintf("sr%d-fft%d", level.SampleRate, level.FFTSize)
	}
	return "unknown"
}

// CandidateEvidenceStateFor builds a fused evidence state from a candidate.
func CandidateEvidenceStateFor(candidate Candidate) CandidateEvidenceState {
	state := CandidateEvidenceState{}
	if len(candidate.Evidence) == 0 {
		return state
	}
	levelSet := map[string]struct{}{}
	provenanceSet := map[string]struct{}{}
	detectionLevels := map[string]struct{}{}
	primaryLevels := map[string]struct{}{}
	derivedLevels := map[string]struct{}{}
	presentationLevels := map[string]struct{}{}
	for _, ev := range candidate.Evidence {
		levelKey := evidenceLevelKey(ev.Level)
		levelSet[levelKey] = struct{}{}
		if ev.Provenance != "" {
			provenanceSet[ev.Provenance] = struct{}{}
		}
		if IsPresentationLevel(ev.Level) {
			presentationLevels[levelKey] = struct{}{}
			continue
		}
		if IsDetectionLevel(ev.Level) {
			detectionLevels[levelKey] = struct{}{}
			if isPrimarySurveillanceLevel(ev.Level) {
				primaryLevels[levelKey] = struct{}{}
			} else if isDerivedSurveillanceLevel(ev.Level) {
				derivedLevels[levelKey] = struct{}{}
			}
		}
	}
	state.TotalLevelEntries = len(candidate.Evidence)
	state.LevelCount = len(levelSet)
	state.DetectionLevelCount = len(detectionLevels)
	state.PrimaryLevelCount = len(primaryLevels)
	state.DerivedLevelCount = len(derivedLevels)
	state.PresentationLevelCount = len(presentationLevels)
	state.Levels = sortedKeys(levelSet)
	state.Provenance = sortedKeys(provenanceSet)
	state.Fused = state.LevelCount > 1 || len(state.Provenance) > 1
	state.DerivedOnly = state.DerivedLevelCount > 0 && state.PrimaryLevelCount == 0 && state.DetectionLevelCount == state.DerivedLevelCount
	state.MultiLevelConfirmed = state.DetectionLevelCount >= 2
	if state.MultiLevelConfirmed {
		if state.PrimaryLevelCount > 0 && state.DerivedLevelCount > 0 {
			state.MultiLevelConfirmedHint = "primary+derived"
		} else {
			state.MultiLevelConfirmedHint = "multi-detection"
		}
	}
	return state
}

// RefreshCandidateEvidenceState updates the candidate's cached evidence summary.
func RefreshCandidateEvidenceState(candidate *Candidate) {
	if candidate == nil {
		return
	}
	state := CandidateEvidenceStateFor(*candidate)
	if state.TotalLevelEntries == 0 {
		candidate.EvidenceState = nil
		return
	}
	candidate.EvidenceState = &state
}

func sortedKeys(src map[string]struct{}) []string {
	if len(src) == 0 {
		return nil
	}
	out := make([]string, 0, len(src))
	for k := range src {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

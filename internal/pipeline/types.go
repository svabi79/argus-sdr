package pipeline

import (
	"sdr-wideband-suite/internal/classifier"
	"sdr-wideband-suite/internal/detector"
)

// Candidate is the coarse output of the surveillance detector.
// It intentionally stays lightweight and cheap to produce.
type Candidate struct {
	ID             int64                   `json:"id"`
	CenterHz       float64                 `json:"center_hz"`
	BandwidthHz    float64                 `json:"bandwidth_hz"`
	PeakDb         float64                 `json:"peak_db"`
	SNRDb          float64                 `json:"snr_db"`
	FirstBin       int                     `json:"first_bin"`
	LastBin        int                     `json:"last_bin"`
	NoiseDb        float64                 `json:"noise_db,omitempty"`
	Source         string                  `json:"source,omitempty"`
	Hint           string                  `json:"hint,omitempty"`
	Evidence       []LevelEvidence         `json:"evidence,omitempty"`
	EvidenceState  *CandidateEvidenceState `json:"evidence_state,omitempty"`
	MonitorMatches []MonitorWindowMatch    `json:"monitor_matches,omitempty"`
}

// LevelEvidence captures which analysis level produced a candidate.
// This is intentionally lightweight for later multi-level fusion.
type LevelEvidence struct {
	Level      AnalysisLevel `json:"level"`
	Provenance string        `json:"provenance,omitempty"`
}

// MonitorWindow describes a monitoring window to gate candidates.
type MonitorWindow struct {
	Index        int     `json:"index,omitempty"`
	Label        string  `json:"label,omitempty"`
	Zone         string  `json:"zone,omitempty"`
	StartHz      float64 `json:"start_hz,omitempty"`
	EndHz        float64 `json:"end_hz,omitempty"`
	CenterHz     float64 `json:"center_hz,omitempty"`
	SpanHz       float64 `json:"span_hz,omitempty"`
	Source       string  `json:"source,omitempty"`
	Priority     float64 `json:"priority,omitempty"`
	PriorityBias float64 `json:"priority_bias,omitempty"`
	RecordBias   float64 `json:"record_bias,omitempty"`
	DecodeBias   float64 `json:"decode_bias,omitempty"`
	AutoRecord   bool    `json:"auto_record,omitempty"`
	AutoDecode   bool    `json:"auto_decode,omitempty"`
}

// MonitorWindowMatch captures how a candidate overlaps a monitor window.
type MonitorWindowMatch struct {
	Index      int     `json:"index"`
	Label      string  `json:"label,omitempty"`
	Zone       string  `json:"zone,omitempty"`
	Source     string  `json:"source,omitempty"`
	StartHz    float64 `json:"start_hz,omitempty"`
	EndHz      float64 `json:"end_hz,omitempty"`
	CenterHz   float64 `json:"center_hz,omitempty"`
	SpanHz     float64 `json:"span_hz,omitempty"`
	OverlapHz  float64 `json:"overlap_hz,omitempty"`
	Coverage   float64 `json:"coverage,omitempty"`
	DistanceHz float64 `json:"distance_hz,omitempty"`
	Bias       float64 `json:"bias,omitempty"`
	RecordBias float64 `json:"record_bias,omitempty"`
	DecodeBias float64 `json:"decode_bias,omitempty"`
	AutoRecord bool    `json:"auto_record,omitempty"`
	AutoDecode bool    `json:"auto_decode,omitempty"`
}

// MonitorWindowStats summarizes candidate attribution per monitor window.
type MonitorWindowStats struct {
	Index        int     `json:"index"`
	Label        string  `json:"label,omitempty"`
	Zone         string  `json:"zone,omitempty"`
	Source       string  `json:"source,omitempty"`
	StartHz      float64 `json:"start_hz,omitempty"`
	EndHz        float64 `json:"end_hz,omitempty"`
	CenterHz     float64 `json:"center_hz,omitempty"`
	SpanHz       float64 `json:"span_hz,omitempty"`
	Priority     float64 `json:"priority,omitempty"`
	PriorityBias float64 `json:"priority_bias,omitempty"`
	RecordBias   float64 `json:"record_bias,omitempty"`
	DecodeBias   float64 `json:"decode_bias,omitempty"`
	AutoRecord   bool    `json:"auto_record,omitempty"`
	AutoDecode   bool    `json:"auto_decode,omitempty"`
	Candidates   int     `json:"candidates,omitempty"`
	Planned      int     `json:"planned,omitempty"`
	Dropped      int     `json:"dropped,omitempty"`
}

// RefinementWindow describes the local analysis span that refinement should use.
type RefinementWindow struct {
	CenterHz float64 `json:"center_hz"`
	SpanHz   float64 `json:"span_hz"`
	Source   string  `json:"source,omitempty"`
}

// Refinement contains higher-cost local analysis derived from a candidate.
type Refinement struct {
	Candidate   Candidate                  `json:"candidate"`
	Window      RefinementWindow           `json:"window"`
	Signal      detector.Signal            `json:"signal"`
	SnippetRate int                        `json:"snippet_rate"`
	Class       *classifier.Classification `json:"class,omitempty"`
	Stage       string                     `json:"stage,omitempty"`
}

func CandidatesFromSignals(signals []detector.Signal, source string) []Candidate {
	out := make([]Candidate, 0, len(signals))
	for _, s := range signals {
		hint := ""
		if s.Class != nil {
			hint = string(s.Class.ModType)
		}
		out = append(out, Candidate{
			ID:          s.ID,
			CenterHz:    s.CenterHz,
			BandwidthHz: s.BWHz,
			PeakDb:      s.PeakDb,
			SNRDb:       s.SNRDb,
			FirstBin:    s.FirstBin,
			LastBin:     s.LastBin,
			NoiseDb:     s.NoiseDb,
			Source:      source,
			Hint:        hint,
		})
	}
	return out
}

func CandidatesFromSignalsWithLevel(signals []detector.Signal, source string, level AnalysisLevel) []Candidate {
	out := CandidatesFromSignals(signals, source)
	if level.Name == "" && level.FFTSize == 0 && level.SampleRate == 0 {
		return out
	}
	evidence := LevelEvidence{Level: level, Provenance: source}
	for i := range out {
		AddCandidateEvidence(&out[i], evidence)
	}
	return out
}

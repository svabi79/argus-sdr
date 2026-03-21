package pipeline

import (
	"sdr-wideband-suite/internal/classifier"
	"sdr-wideband-suite/internal/detector"
)

// Candidate is the coarse output of the surveillance detector.
// It intentionally stays lightweight and cheap to produce.
type Candidate struct {
	ID         int64     `json:"id"`
	CenterHz   float64   `json:"center_hz"`
	BandwidthHz float64  `json:"bandwidth_hz"`
	PeakDb     float64   `json:"peak_db"`
	SNRDb      float64   `json:"snr_db"`
	FirstBin   int       `json:"first_bin"`
	LastBin    int       `json:"last_bin"`
	NoiseDb    float64   `json:"noise_db,omitempty"`
	Source     string    `json:"source,omitempty"`
	Hint       string    `json:"hint,omitempty"`
}

// Refinement contains higher-cost local analysis derived from a candidate.
type Refinement struct {
	Candidate   Candidate                    `json:"candidate"`
	Signal      detector.Signal              `json:"signal"`
	SnippetRate int                          `json:"snippet_rate"`
	Class       *classifier.Classification   `json:"class,omitempty"`
	Stage       string                       `json:"stage,omitempty"`
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

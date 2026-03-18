package recorder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"sdr-visual-suite/internal/classifier"
	"sdr-visual-suite/internal/detector"
)

type Meta struct {
	EventID     int64                      `json:"event_id"`
	Start       time.Time                  `json:"start"`
	End         time.Time                  `json:"end"`
	CenterHz    float64                    `json:"center_hz"`
	BandwidthHz float64                    `json:"bandwidth_hz"`
	SampleRate  int                        `json:"sample_rate"`
	SNRDb       float64                    `json:"snr_db"`
	PeakDb      float64                    `json:"peak_db"`
	Class       *classifier.Classification `json:"classification,omitempty"`
	DurationMs  int64                      `json:"duration_ms"`
	Files       map[string]any             `json:"files"`
}

func writeMeta(dir string, ev detector.Event, sampleRate int, files map[string]any) error {
	m := Meta{
		EventID:     ev.ID,
		Start:       ev.Start,
		End:         ev.End,
		CenterHz:    ev.CenterHz,
		BandwidthHz: ev.Bandwidth,
		SampleRate:  sampleRate,
		SNRDb:       ev.SNRDb,
		PeakDb:      ev.PeakDb,
		Class:       ev.Class,
		DurationMs:  ev.End.Sub(ev.Start).Milliseconds(),
		Files:       files,
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "meta.json"), b, 0o644)
}

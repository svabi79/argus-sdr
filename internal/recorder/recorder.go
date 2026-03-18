package recorder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sdr-visual-suite/internal/detector"
)

type Policy struct {
	Enabled     bool          `yaml:"enabled" json:"enabled"`
	MinSNRDb    float64       `yaml:"min_snr_db" json:"min_snr_db"`
	MinDuration time.Duration `yaml:"min_duration" json:"min_duration"`
	MaxDuration time.Duration `yaml:"max_duration" json:"max_duration"`
	PrerollMs   int           `yaml:"preroll_ms" json:"preroll_ms"`
	RecordIQ    bool          `yaml:"record_iq" json:"record_iq"`
	RecordAudio bool          `yaml:"record_audio" json:"record_audio"`
	AutoDemod   bool          `yaml:"auto_demod" json:"auto_demod"`
	AutoDecode  bool          `yaml:"auto_decode" json:"auto_decode"`
	OutputDir   string        `yaml:"output_dir" json:"output_dir"`
	ClassFilter []string      `yaml:"class_filter" json:"class_filter"`
	RingSeconds int           `yaml:"ring_seconds" json:"ring_seconds"`
}

type Manager struct {
	policy         Policy
	ring           *Ring
	sampleRate     int
	blockSize      int
	centerHz       float64
	decodeCommands map[string]string
}

func New(sampleRate int, blockSize int, policy Policy, centerHz float64, decodeCommands map[string]string) *Manager {
	if policy.OutputDir == "" {
		policy.OutputDir = "data/recordings"
	}
	if policy.RingSeconds <= 0 {
		policy.RingSeconds = 8
	}
	return &Manager{policy: policy, ring: NewRing(sampleRate, blockSize, policy.RingSeconds), sampleRate: sampleRate, blockSize: blockSize, centerHz: centerHz, decodeCommands: decodeCommands}
}

func (m *Manager) Update(sampleRate int, blockSize int, policy Policy, centerHz float64, decodeCommands map[string]string) {
	m.policy = policy
	m.sampleRate = sampleRate
	m.blockSize = blockSize
	m.centerHz = centerHz
	m.decodeCommands = decodeCommands
	if m.ring == nil {
		m.ring = NewRing(sampleRate, blockSize, policy.RingSeconds)
		return
	}
	m.ring.Reset(sampleRate, blockSize, policy.RingSeconds)
}

func (m *Manager) Ingest(t0 time.Time, samples []complex64) {
	if m == nil || m.ring == nil {
		return
	}
	m.ring.Push(t0, samples)
}

func (m *Manager) OnEvents(events []detector.Event) {
	if m == nil || !m.policy.Enabled || len(events) == 0 {
		return
	}
	for _, ev := range events {
		_ = m.recordEvent(ev)
	}
}

func (m *Manager) recordEvent(ev detector.Event) error {
	if !m.policy.Enabled {
		return nil
	}
	if ev.SNRDb < m.policy.MinSNRDb {
		return nil
	}
	dur := ev.End.Sub(ev.Start)
	if m.policy.MinDuration > 0 && dur < m.policy.MinDuration {
		return nil
	}
	if m.policy.MaxDuration > 0 && dur > m.policy.MaxDuration {
		return nil
	}
	if len(m.policy.ClassFilter) > 0 && ev.Class != nil {
		match := false
		for _, c := range m.policy.ClassFilter {
			if strings.EqualFold(c, string(ev.Class.ModType)) {
				match = true
				break
			}
		}
		if !match {
			return nil
		}
	}
	if !m.policy.RecordIQ && !m.policy.RecordAudio {
		return nil
	}

	start := ev.Start.Add(-time.Duration(m.policy.PrerollMs) * time.Millisecond)
	end := ev.End
	if start.After(end) {
		return errors.New("invalid event window")
	}

	segment := m.ring.Slice(start, end)
	if len(segment) == 0 {
		return errors.New("no iq in ring")
	}

	dir := filepath.Join(m.policy.OutputDir, fmt.Sprintf("%s_%0.fHz_evt%d", ev.Start.Format("2006-01-02T15-04-05"), ev.CenterHz, ev.ID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string]any{}
	var iqPath string
	if m.policy.RecordIQ {
		iqPath = filepath.Join(dir, "signal.cf32")
		if err := writeCF32(iqPath, segment); err != nil {
			return err
		}
		files["iq"] = "signal.cf32"
		files["iq_format"] = "cf32"
		files["iq_sample_rate"] = m.sampleRate
	}

	// Optional demod + audio
	if m.policy.RecordAudio && m.policy.AutoDemod && ev.Class != nil {
		if err := m.demodAndWrite(dir, ev, segment, files); err != nil {
			return err
		}
	}
	if m.policy.AutoDecode && iqPath != "" && ev.Class != nil {
		m.runDecodeIfConfigured(string(ev.Class.ModType), iqPath, m.sampleRate, files, dir)
	}

	return writeMeta(dir, ev, m.sampleRate, files)
}

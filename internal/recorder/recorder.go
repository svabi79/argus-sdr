package recorder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sdr-visual-suite/internal/demod/gpudemod"
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
	MaxDiskMB   int           `yaml:"max_disk_mb" json:"max_disk_mb"`
	OutputDir   string        `yaml:"output_dir" json:"output_dir"`
	ClassFilter []string      `yaml:"class_filter" json:"class_filter"`
	RingSeconds int           `yaml:"ring_seconds" json:"ring_seconds"`
}

type Manager struct {
	mu             sync.RWMutex
	policy         Policy
	ring           *Ring
	sampleRate     int
	blockSize      int
	centerHz       float64
	decodeCommands map[string]string
	queue          chan detector.Event
	gpuDemod       *gpudemod.Engine
}

func New(sampleRate int, blockSize int, policy Policy, centerHz float64, decodeCommands map[string]string) *Manager {
	if policy.OutputDir == "" {
		policy.OutputDir = "data/recordings"
	}
	if policy.RingSeconds <= 0 {
		policy.RingSeconds = 8
	}
	m := &Manager{policy: policy, ring: NewRing(sampleRate, blockSize, policy.RingSeconds), sampleRate: sampleRate, blockSize: blockSize, centerHz: centerHz, decodeCommands: decodeCommands, queue: make(chan detector.Event, 64)}
	m.initGPUDemod(sampleRate, blockSize)
	go m.worker()
	return m
}

func (m *Manager) Update(sampleRate int, blockSize int, policy Policy, centerHz float64, decodeCommands map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
	m.sampleRate = sampleRate
	m.blockSize = blockSize
	m.centerHz = centerHz
	m.decodeCommands = decodeCommands
	m.initGPUDemodLocked(sampleRate, blockSize)
	if m.ring == nil {
		m.ring = NewRing(sampleRate, blockSize, policy.RingSeconds)
		return
	}
	m.ring.Reset(sampleRate, blockSize, policy.RingSeconds)
}

func (m *Manager) Ingest(t0 time.Time, samples []complex64) {
	if m == nil {
		return
	}
	m.mu.RLock()
	ring := m.ring
	m.mu.RUnlock()
	if ring == nil {
		return
	}
	ring.Push(t0, samples)
}

func (m *Manager) OnEvents(events []detector.Event) {
	if m == nil || len(events) == 0 {
		return
	}
	m.mu.RLock()
	enabled := m.policy.Enabled
	m.mu.RUnlock()
	if !enabled {
		return
	}
	for _, ev := range events {
		select {
		case m.queue <- ev:
		default:
			// drop if queue full
		}
	}
}

func (m *Manager) worker() {
	for ev := range m.queue {
		_ = m.recordEvent(ev)
	}
}

func (m *Manager) initGPUDemod(sampleRate int, blockSize int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initGPUDemodLocked(sampleRate, blockSize)
}

func (m *Manager) initGPUDemodLocked(sampleRate int, blockSize int) {
	if m.gpuDemod != nil {
		m.gpuDemod.Close()
		m.gpuDemod = nil
	}
	if !gpudemod.Available() {
		return
	}
	eng, err := gpudemod.New(blockSize, sampleRate)
	if err != nil {
		return
	}
	m.gpuDemod = eng
}

func (m *Manager) recordEvent(ev detector.Event) error {
	m.mu.RLock()
	policy := m.policy
	ring := m.ring
	sampleRate := m.sampleRate
	centerHz := m.centerHz
	m.mu.RUnlock()

	if !policy.Enabled {
		return nil
	}
	if ev.SNRDb < policy.MinSNRDb {
		return nil
	}
	dur := ev.End.Sub(ev.Start)
	if policy.MinDuration > 0 && dur < policy.MinDuration {
		return nil
	}
	if policy.MaxDuration > 0 && dur > policy.MaxDuration {
		return nil
	}
	if len(policy.ClassFilter) > 0 && ev.Class != nil {
		match := false
		for _, c := range policy.ClassFilter {
			if strings.EqualFold(c, string(ev.Class.ModType)) {
				match = true
				break
			}
		}
		if !match {
			return nil
		}
	}
	if !policy.RecordIQ && !policy.RecordAudio {
		return nil
	}

	start := ev.Start.Add(-time.Duration(policy.PrerollMs) * time.Millisecond)
	end := ev.End
	if start.After(end) {
		return errors.New("invalid event window")
	}
	if ring == nil {
		return errors.New("no ring buffer")
	}

	segment := ring.Slice(start, end)
	if len(segment) == 0 {
		return errors.New("no iq in ring")
	}

	dir := filepath.Join(policy.OutputDir, fmt.Sprintf("%s_%0.fHz_evt%d", ev.Start.Format("2006-01-02T15-04-05"), ev.CenterHz, ev.ID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string]any{}
	var iqPath string
	if policy.RecordIQ {
		iqPath = filepath.Join(dir, "signal.cf32")
		if err := writeCF32(iqPath, segment); err != nil {
			return err
		}
		files["iq"] = "signal.cf32"
		files["iq_format"] = "cf32"
		files["iq_sample_rate"] = sampleRate
	}

	if policy.RecordAudio && policy.AutoDemod && ev.Class != nil {
		if err := m.demodAndWrite(dir, ev, segment, files); err != nil {
			return err
		}
	}
	if policy.AutoDecode && iqPath != "" && ev.Class != nil {
		m.runDecodeIfConfigured(string(ev.Class.ModType), iqPath, sampleRate, files, dir)
	}

	_ = centerHz
	return writeMeta(dir, ev, sampleRate, files)
}

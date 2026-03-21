package recorder

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sdr-wideband-suite/internal/demod/gpudemod"
	"sdr-wideband-suite/internal/detector"
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

	// Audio quality (AQ-2, AQ-3, AQ-5)
	DeemphasisUs     float64 `yaml:"deemphasis_us" json:"deemphasis_us"`
	ExtractionTaps   int     `yaml:"extraction_fir_taps" json:"extraction_fir_taps"`
	ExtractionBwMult float64 `yaml:"extraction_bw_mult" json:"extraction_bw_mult"`
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
	closed         bool
	closeOnce      sync.Once
	workerWG       sync.WaitGroup

	// Streaming recorder
	streamer    *Streamer
	streamedIDs map[int64]bool // signal IDs that were streamed (skip retroactive recording)
	streamedMu  sync.Mutex
}

func New(sampleRate int, blockSize int, policy Policy, centerHz float64, decodeCommands map[string]string) *Manager {
	if policy.OutputDir == "" {
		policy.OutputDir = "data/recordings"
	}
	if policy.RingSeconds <= 0 {
		policy.RingSeconds = 8
	}
	m := &Manager{
		policy:         policy,
		ring:           NewRing(sampleRate, blockSize, policy.RingSeconds),
		sampleRate:     sampleRate,
		blockSize:      blockSize,
		centerHz:       centerHz,
		decodeCommands: decodeCommands,
		queue:          make(chan detector.Event, 64),
		streamer:       newStreamer(policy, centerHz),
		streamedIDs:    make(map[int64]bool),
	}
	m.initGPUDemod(sampleRate, blockSize)
	m.workerWG.Add(1)
	go m.worker()
	return m
}

func (m *Manager) Update(sampleRate int, blockSize int, policy Policy, centerHz float64, decodeCommands map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
	m.centerHz = centerHz
	m.decodeCommands = decodeCommands
	// Only reset ring and GPU engine if sample parameters actually changed
	needRingReset := m.sampleRate != sampleRate || m.blockSize != blockSize
	m.sampleRate = sampleRate
	m.blockSize = blockSize
	if needRingReset {
		m.initGPUDemodLocked(sampleRate, blockSize)
		if m.ring == nil {
			m.ring = NewRing(sampleRate, blockSize, policy.RingSeconds)
		} else {
			m.ring.Reset(sampleRate, blockSize, policy.RingSeconds)
		}
	} else if m.ring == nil {
		m.ring = NewRing(sampleRate, blockSize, policy.RingSeconds)
	}
	if m.streamer != nil {
		m.streamer.updatePolicy(policy, centerHz)
	}
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
	closed := m.closed
	m.mu.RUnlock()
	if !enabled || closed {
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
	defer m.workerWG.Done()
	for ev := range m.queue {
		_ = m.recordEvent(ev)
	}
}

func (m *Manager) initGPUDemod(sampleRate int, blockSize int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initGPUDemodLocked(sampleRate, blockSize)
}

func (m *Manager) gpuEngine() *gpudemod.Engine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.gpuDemod
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

func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.closeOnce.Do(func() {
		// Close all active streaming sessions first
		if m.streamer != nil {
			m.streamer.CloseAll()
		}

		m.mu.Lock()
		m.closed = true
		if m.queue != nil {
			close(m.queue)
		}
		gpu := m.gpuDemod
		m.gpuDemod = nil
		m.mu.Unlock()
		m.workerWG.Wait()
		if gpu != nil {
			gpu.Close()
		}
	})
}

func (m *Manager) recordEvent(ev detector.Event) error {
	// Skip events that were already recorded via streaming
	m.streamedMu.Lock()
	wasStreamed := m.streamedIDs[ev.ID]
	delete(m.streamedIDs, ev.ID) // clean up — event is finished
	m.streamedMu.Unlock()
	if wasStreamed {
		log.Printf("STREAM: skipping retroactive recording for signal %d (already streamed)", ev.ID)
		return nil
	}

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

// SliceRecent returns the most recent `seconds` of raw IQ from the ring buffer.
// Returns the IQ samples, sample rate, and center frequency.
func (m *Manager) SliceRecent(seconds float64) ([]complex64, int, float64) {
	if m == nil {
		return nil, 0, 0
	}
	m.mu.RLock()
	ring := m.ring
	sr := m.sampleRate
	center := m.centerHz
	m.mu.RUnlock()
	if ring == nil || sr <= 0 {
		return nil, 0, 0
	}
	end := time.Now()
	start := end.Add(-time.Duration(seconds * float64(time.Second)))
	iq := ring.Slice(start, end)
	return iq, sr, center
}

// FeedSnippets is called once per DSP frame with pre-extracted IQ snippets
// (GPU-accelerated FreqShift+FIR+Decimate). The Streamer handles demod with
// persistent state (overlap-save, stereo decode, de-emphasis) asynchronously.
func (m *Manager) FeedSnippets(items []StreamFeedItem) {
	if m == nil || m.streamer == nil || len(items) == 0 {
		return
	}
	m.mu.RLock()
	closed := m.closed
	m.mu.RUnlock()
	if closed {
		return
	}

	// Mark all signal IDs so recordEvent skips them
	m.streamedMu.Lock()
	for _, item := range items {
		if item.Signal.ID != 0 {
			m.streamedIDs[item.Signal.ID] = true
		}
	}
	m.streamedMu.Unlock()

	// Convert to internal type
	internal := make([]streamFeedItem, len(items))
	for i, item := range items {
		internal[i] = streamFeedItem{
			signal:   item.Signal,
			snippet:  item.Snippet,
			snipRate: item.SnipRate,
		}
	}
	m.streamer.FeedSnippets(internal)
}

// StreamFeedItem is the public type for passing extracted snippets from DSP loop.
type StreamFeedItem struct {
	Signal   detector.Signal
	Snippet  []complex64
	SnipRate int
}

// Streamer returns the underlying Streamer for live-listen subscriptions.
func (m *Manager) StreamerRef() *Streamer {
	if m == nil {
		return nil
	}
	return m.streamer
}

// ActiveStreams returns info about currently active streaming sessions.
func (m *Manager) ActiveStreams() int {
	if m == nil || m.streamer == nil {
		return 0
	}
	return m.streamer.ActiveSessions()
}

// HasListeners returns true if any live-listen subscribers are active or pending.
func (m *Manager) HasListeners() bool {
	if m == nil || m.streamer == nil {
		return false
	}
	return m.streamer.HasListeners()
}

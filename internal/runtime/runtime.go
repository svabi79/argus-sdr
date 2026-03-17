package runtime

import (
	"errors"
	"sync"

	"sdr-visual-suite/internal/config"
)

type ConfigUpdate struct {
	CenterHz   *float64        `json:"center_hz"`
	SampleRate *int            `json:"sample_rate"`
	FFTSize    *int            `json:"fft_size"`
	GainDb     *float64        `json:"gain_db"`
	TunerBwKHz *int            `json:"tuner_bw_khz"`
	UseGPUFFT  *bool           `json:"use_gpu_fft"`
	Detector   *DetectorUpdate `json:"detector"`
}

type DetectorUpdate struct {
	ThresholdDb *float64 `json:"threshold_db"`
	MinDuration *int     `json:"min_duration_ms"`
	HoldMs      *int     `json:"hold_ms"`
}

type SettingsUpdate struct {
	AGC       *bool `json:"agc"`
	DCBlock   *bool `json:"dc_block"`
	IQBalance *bool `json:"iq_balance"`
}

type Manager struct {
	mu  sync.RWMutex
	cfg config.Config
}

func New(cfg config.Config) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) Snapshot() config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) Replace(cfg config.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
}

func (m *Manager) ApplyConfig(update ConfigUpdate) (config.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	next := m.cfg
	if update.CenterHz != nil {
		if *update.CenterHz < 1e3 || *update.CenterHz > 2e9 {
			return m.cfg, errors.New("center_hz out of range")
		}
		next.CenterHz = *update.CenterHz
	}
	if update.SampleRate != nil {
		if *update.SampleRate <= 0 {
			return m.cfg, errors.New("sample_rate must be > 0")
		}
		next.SampleRate = *update.SampleRate
	}
	if update.FFTSize != nil {
		if *update.FFTSize <= 0 {
			return m.cfg, errors.New("fft_size must be > 0")
		}
		next.FFTSize = *update.FFTSize
	}
	if update.GainDb != nil {
		next.GainDb = *update.GainDb
	}
	if update.TunerBwKHz != nil {
		if *update.TunerBwKHz <= 0 {
			return m.cfg, errors.New("tuner_bw_khz must be > 0")
		}
		next.TunerBwKHz = *update.TunerBwKHz
	}
	if update.UseGPUFFT != nil {
		next.UseGPUFFT = *update.UseGPUFFT
	}
	if update.Detector != nil {
		if update.Detector.ThresholdDb != nil {
			next.Detector.ThresholdDb = *update.Detector.ThresholdDb
		}
		if update.Detector.MinDuration != nil {
			if *update.Detector.MinDuration <= 0 {
				return m.cfg, errors.New("min_duration_ms must be > 0")
			}
			next.Detector.MinDurationMs = *update.Detector.MinDuration
		}
		if update.Detector.HoldMs != nil {
			if *update.Detector.HoldMs <= 0 {
				return m.cfg, errors.New("hold_ms must be > 0")
			}
			next.Detector.HoldMs = *update.Detector.HoldMs
		}
	}

	m.cfg = next
	return m.cfg, nil
}

func (m *Manager) ApplySettings(update SettingsUpdate) (config.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	next := m.cfg
	if update.AGC != nil {
		next.AGC = *update.AGC
	}
	if update.DCBlock != nil {
		next.DCBlock = *update.DCBlock
	}
	if update.IQBalance != nil {
		next.IQBalance = *update.IQBalance
	}

	m.cfg = next
	return m.cfg, nil
}

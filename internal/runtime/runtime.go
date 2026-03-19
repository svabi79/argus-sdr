package runtime

import (
	"errors"
	"math"
	"strings"
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
	Recorder   *RecorderUpdate `json:"recorder"`
}

type DetectorUpdate struct {
	ThresholdDb     *float64 `json:"threshold_db"`
	MinDuration     *int     `json:"min_duration_ms"`
	HoldMs          *int     `json:"hold_ms"`
	EmaAlpha        *float64 `json:"ema_alpha"`
	HysteresisDb    *float64 `json:"hysteresis_db"`
	MinStableFrames *int     `json:"min_stable_frames"`
	GapToleranceMs  *int     `json:"gap_tolerance_ms"`
	CFARMode        *string  `json:"cfar_mode"`
	CFARGuardCells  *int     `json:"cfar_guard_cells"`
	CFARTrainCells  *int     `json:"cfar_train_cells"`
	CFARRank        *int     `json:"cfar_rank"`
	CFARScaleDb     *float64 `json:"cfar_scale_db"`
	CFARWrapAround  *bool    `json:"cfar_wrap_around"`
	EdgeMarginDb    *float64 `json:"edge_margin_db"`
	MergeGapHz      *float64 `json:"merge_gap_hz"`
}

type SettingsUpdate struct {
	AGC       *bool `json:"agc"`
	DCBlock   *bool `json:"dc_block"`
	IQBalance *bool `json:"iq_balance"`
}

type RecorderUpdate struct {
	Enabled     *bool     `json:"enabled"`
	MinSNRDb    *float64  `json:"min_snr_db"`
	MinDuration *string   `json:"min_duration"`
	MaxDuration *string   `json:"max_duration"`
	PrerollMs   *int      `json:"preroll_ms"`
	RecordIQ    *bool     `json:"record_iq"`
	RecordAudio *bool     `json:"record_audio"`
	AutoDemod   *bool     `json:"auto_demod"`
	AutoDecode  *bool     `json:"auto_decode"`
	MaxDiskMB   *int      `json:"max_disk_mb"`
	OutputDir   *string   `json:"output_dir"`
	ClassFilter *[]string `json:"class_filter"`
	RingSeconds *int      `json:"ring_seconds"`
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
		if *update.FFTSize&(*update.FFTSize-1) != 0 {
			return m.cfg, errors.New("fft_size must be a power of 2")
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
		if update.Detector.EmaAlpha != nil {
			v := *update.Detector.EmaAlpha
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
				return m.cfg, errors.New("ema_alpha must be between 0 and 1")
			}
			next.Detector.EmaAlpha = v
		}
		if update.Detector.HysteresisDb != nil {
			v := *update.Detector.HysteresisDb
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return m.cfg, errors.New("hysteresis_db must be >= 0")
			}
			next.Detector.HysteresisDb = v
		}
		if update.Detector.MinStableFrames != nil {
			if *update.Detector.MinStableFrames < 1 {
				return m.cfg, errors.New("min_stable_frames must be >= 1")
			}
			next.Detector.MinStableFrames = *update.Detector.MinStableFrames
		}
		if update.Detector.GapToleranceMs != nil {
			if *update.Detector.GapToleranceMs < 0 {
				return m.cfg, errors.New("gap_tolerance_ms must be >= 0")
			}
			next.Detector.GapToleranceMs = *update.Detector.GapToleranceMs
		}
		if update.Detector.CFARMode != nil {
			mode := strings.ToUpper(strings.TrimSpace(*update.Detector.CFARMode))
			switch mode {
			case "OFF", "CA", "OS", "GOSCA", "CASO":
				next.Detector.CFARMode = mode
			default:
				return m.cfg, errors.New("cfar_mode must be OFF, CA, OS, GOSCA, or CASO")
			}
		}
		if update.Detector.CFARWrapAround != nil {
			next.Detector.CFARWrapAround = *update.Detector.CFARWrapAround
		}
		if update.Detector.CFARGuardCells != nil {
			if *update.Detector.CFARGuardCells < 0 {
				return m.cfg, errors.New("cfar_guard_cells must be >= 0")
			}
			next.Detector.CFARGuardCells = *update.Detector.CFARGuardCells
		}
		if update.Detector.CFARTrainCells != nil {
			if *update.Detector.CFARTrainCells <= 0 {
				return m.cfg, errors.New("cfar_train_cells must be > 0")
			}
			next.Detector.CFARTrainCells = *update.Detector.CFARTrainCells
		}
		if update.Detector.CFARRank != nil {
			if *update.Detector.CFARRank <= 0 {
				return m.cfg, errors.New("cfar_rank must be > 0")
			}
			if next.Detector.CFARTrainCells > 0 && *update.Detector.CFARRank > 2*next.Detector.CFARTrainCells {
				return m.cfg, errors.New("cfar_rank must be <= 2 * cfar_train_cells")
			}
			next.Detector.CFARRank = *update.Detector.CFARRank
		}
		if update.Detector.CFARScaleDb != nil {
			next.Detector.CFARScaleDb = *update.Detector.CFARScaleDb
		}
		if update.Detector.EdgeMarginDb != nil {
			v := *update.Detector.EdgeMarginDb
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return m.cfg, errors.New("edge_margin_db must be >= 0")
			}
			next.Detector.EdgeMarginDb = v
		}
		if update.Detector.MergeGapHz != nil {
			v := *update.Detector.MergeGapHz
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return m.cfg, errors.New("merge_gap_hz must be >= 0")
			}
			next.Detector.MergeGapHz = v
		}
	}
	if update.Recorder != nil {
		if update.Recorder.Enabled != nil {
			next.Recorder.Enabled = *update.Recorder.Enabled
		}
		if update.Recorder.MinSNRDb != nil {
			next.Recorder.MinSNRDb = *update.Recorder.MinSNRDb
		}
		if update.Recorder.MinDuration != nil {
			next.Recorder.MinDuration = *update.Recorder.MinDuration
		}
		if update.Recorder.MaxDuration != nil {
			next.Recorder.MaxDuration = *update.Recorder.MaxDuration
		}
		if update.Recorder.PrerollMs != nil {
			next.Recorder.PrerollMs = *update.Recorder.PrerollMs
		}
		if update.Recorder.RecordIQ != nil {
			next.Recorder.RecordIQ = *update.Recorder.RecordIQ
		}
		if update.Recorder.RecordAudio != nil {
			next.Recorder.RecordAudio = *update.Recorder.RecordAudio
		}
		if update.Recorder.AutoDemod != nil {
			next.Recorder.AutoDemod = *update.Recorder.AutoDemod
		}
		if update.Recorder.AutoDecode != nil {
			next.Recorder.AutoDecode = *update.Recorder.AutoDecode
		}
		if update.Recorder.MaxDiskMB != nil {
			next.Recorder.MaxDiskMB = *update.Recorder.MaxDiskMB
		}
		if update.Recorder.OutputDir != nil {
			next.Recorder.OutputDir = *update.Recorder.OutputDir
		}
		if update.Recorder.ClassFilter != nil {
			next.Recorder.ClassFilter = *update.Recorder.ClassFilter
		}
		if update.Recorder.RingSeconds != nil {
			next.Recorder.RingSeconds = *update.Recorder.RingSeconds
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

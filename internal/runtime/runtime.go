package runtime

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"sdr-wideband-suite/internal/config"
)

type PipelineUpdate struct {
	Mode              *string                 `json:"mode"`
	Profile           *string                 `json:"profile"`
	Intent            *string                 `json:"intent"`
	MonitorStartHz    *float64                `json:"monitor_start_hz"`
	MonitorEndHz      *float64                `json:"monitor_end_hz"`
	MonitorSpanHz     *float64                `json:"monitor_span_hz"`
	MonitorWindows    *[]config.MonitorWindow `json:"monitor_windows"`
	SignalPriorities  *[]string               `json:"signal_priorities"`
	AutoRecordClasses *[]string               `json:"auto_record_classes"`
	AutoDecodeClasses *[]string               `json:"auto_decode_classes"`
}

type SurveillanceUpdate struct {
	AnalysisFFTSize  *int    `json:"analysis_fft_size"`
	FrameRate        *int    `json:"frame_rate"`
	Strategy         *string `json:"strategy"`
	DisplayBins      *int    `json:"display_bins"`
	DisplayFPS       *int    `json:"display_fps"`
	DerivedDetection *string `json:"derived_detection"`
}

type RefinementUpdate struct {
	Enabled           *bool    `json:"enabled"`
	MaxConcurrent     *int     `json:"max_concurrent"`
	DetailFFTSize     *int     `json:"detail_fft_size"`
	MinCandidateSNRDb *float64 `json:"min_candidate_snr_db"`
	MinSpanHz         *float64 `json:"min_span_hz"`
	MaxSpanHz         *float64 `json:"max_span_hz"`
	AutoSpan          *bool    `json:"auto_span"`
}

type ResourcesUpdate struct {
	PreferGPU           *bool `json:"prefer_gpu"`
	MaxRefinementJobs   *int  `json:"max_refinement_jobs"`
	MaxRecordingStreams *int  `json:"max_recording_streams"`
	MaxDecodeJobs       *int  `json:"max_decode_jobs"`
	DecisionHoldMs      *int  `json:"decision_hold_ms"`
}

type ConfigUpdate struct {
	CenterHz       *float64            `json:"center_hz"`
	SampleRate     *int                `json:"sample_rate"`
	FFTSize        *int                `json:"fft_size"`
	GainDb         *float64            `json:"gain_db"`
	TunerBwKHz     *int                `json:"tuner_bw_khz"`
	UseGPUFFT      *bool               `json:"use_gpu_fft"`
	ClassifierMode *string             `json:"classifier_mode"`
	Pipeline       *PipelineUpdate     `json:"pipeline"`
	Surveillance   *SurveillanceUpdate `json:"surveillance"`
	Refinement     *RefinementUpdate   `json:"refinement"`
	Resources      *ResourcesUpdate    `json:"resources"`
	Detector       *DetectorUpdate     `json:"detector"`
	Recorder       *RecorderUpdate     `json:"recorder"`
}

type DetectorUpdate struct {
	ThresholdDb      *float64 `json:"threshold_db"`
	MinDuration      *int     `json:"min_duration_ms"`
	HoldMs           *int     `json:"hold_ms"`
	EmaAlpha         *float64 `json:"ema_alpha"`
	HysteresisDb     *float64 `json:"hysteresis_db"`
	MinStableFrames  *int     `json:"min_stable_frames"`
	GapToleranceMs   *int     `json:"gap_tolerance_ms"`
	CFARMode         *string  `json:"cfar_mode"`
	CFARGuardHz      *float64 `json:"cfar_guard_hz"`
	CFARTrainHz      *float64 `json:"cfar_train_hz"`
	CFARGuardCells   *int     `json:"cfar_guard_cells"`
	CFARTrainCells   *int     `json:"cfar_train_cells"`
	CFARRank         *int     `json:"cfar_rank"`
	CFARScaleDb      *float64 `json:"cfar_scale_db"`
	CFARWrapAround   *bool    `json:"cfar_wrap_around"`
	EdgeMarginDb     *float64 `json:"edge_margin_db"`
	MergeGapHz       *float64 `json:"merge_gap_hz"`
	ClassHistorySize *int     `json:"class_history_size"`
	ClassSwitchRatio *float64 `json:"class_switch_ratio"`
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
		next.Surveillance.AnalysisFFTSize = *update.FFTSize
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
	if update.ClassifierMode != nil {
		mode := *update.ClassifierMode
		switch mode {
		case "rule", "math", "combined":
			next.ClassifierMode = mode
		default:
			return m.cfg, errors.New("classifier_mode must be rule, math, or combined")
		}
	}
	if update.Pipeline != nil {
		if update.Pipeline.Mode != nil {
			next.Pipeline.Mode = *update.Pipeline.Mode
		}
		if update.Pipeline.Profile != nil {
			next.Pipeline.Profile = *update.Pipeline.Profile
		}
		if update.Pipeline.Intent != nil {
			next.Pipeline.Goals.Intent = *update.Pipeline.Intent
		}
		if update.Pipeline.MonitorStartHz != nil {
			next.Pipeline.Goals.MonitorStartHz = *update.Pipeline.MonitorStartHz
		}
		if update.Pipeline.MonitorEndHz != nil {
			next.Pipeline.Goals.MonitorEndHz = *update.Pipeline.MonitorEndHz
		}
		if update.Pipeline.MonitorSpanHz != nil {
			if *update.Pipeline.MonitorSpanHz <= 0 {
				return m.cfg, errors.New("monitor_span_hz must be > 0")
			}
			next.Pipeline.Goals.MonitorSpanHz = *update.Pipeline.MonitorSpanHz
		}
		if update.Pipeline.MonitorWindows != nil {
			windows := *update.Pipeline.MonitorWindows
			if err := validateMonitorWindows(windows); err != nil {
				return m.cfg, err
			}
			next.Pipeline.Goals.MonitorWindows = append([]config.MonitorWindow(nil), windows...)
		}
		if update.Pipeline.SignalPriorities != nil {
			next.Pipeline.Goals.SignalPriorities = append([]string(nil), (*update.Pipeline.SignalPriorities)...)
		}
		if update.Pipeline.AutoRecordClasses != nil {
			next.Pipeline.Goals.AutoRecordClasses = append([]string(nil), (*update.Pipeline.AutoRecordClasses)...)
		}
		if update.Pipeline.AutoDecodeClasses != nil {
			next.Pipeline.Goals.AutoDecodeClasses = append([]string(nil), (*update.Pipeline.AutoDecodeClasses)...)
		}
		if next.Pipeline.Goals.MonitorStartHz != 0 && next.Pipeline.Goals.MonitorEndHz != 0 && next.Pipeline.Goals.MonitorEndHz <= next.Pipeline.Goals.MonitorStartHz {
			return m.cfg, errors.New("monitor_end_hz must be > monitor_start_hz")
		}
		if next.Pipeline.Goals.MonitorSpanHz <= 0 && next.Pipeline.Goals.MonitorStartHz != 0 && next.Pipeline.Goals.MonitorEndHz != 0 && next.Pipeline.Goals.MonitorEndHz > next.Pipeline.Goals.MonitorStartHz {
			next.Pipeline.Goals.MonitorSpanHz = next.Pipeline.Goals.MonitorEndHz - next.Pipeline.Goals.MonitorStartHz
		}
	}
	if update.Surveillance != nil {
		if update.Surveillance.AnalysisFFTSize != nil {
			v := *update.Surveillance.AnalysisFFTSize
			if v <= 0 {
				return m.cfg, errors.New("surveillance.analysis_fft_size must be > 0")
			}
			if v&(v-1) != 0 {
				return m.cfg, errors.New("surveillance.analysis_fft_size must be a power of 2")
			}
			next.Surveillance.AnalysisFFTSize = v
			next.FFTSize = v
		}
		if update.Surveillance.FrameRate != nil {
			v := *update.Surveillance.FrameRate
			if v <= 0 {
				return m.cfg, errors.New("surveillance.frame_rate must be > 0")
			}
			next.Surveillance.FrameRate = v
			next.FrameRate = v
		}
		if update.Surveillance.Strategy != nil {
			next.Surveillance.Strategy = *update.Surveillance.Strategy
		}
		if update.Surveillance.DisplayBins != nil {
			v := *update.Surveillance.DisplayBins
			if v <= 0 {
				return m.cfg, errors.New("surveillance.display_bins must be > 0")
			}
			next.Surveillance.DisplayBins = v
		}
		if update.Surveillance.DisplayFPS != nil {
			v := *update.Surveillance.DisplayFPS
			if v <= 0 {
				return m.cfg, errors.New("surveillance.display_fps must be > 0")
			}
			next.Surveillance.DisplayFPS = v
		}
		if update.Surveillance.DerivedDetection != nil {
			mode := strings.ToLower(strings.TrimSpace(*update.Surveillance.DerivedDetection))
			switch mode {
			case "auto", "on", "off", "true", "false", "enabled", "disabled", "enable", "disable":
				next.Surveillance.DerivedDetection = mode
			default:
				return m.cfg, errors.New("surveillance.derived_detection must be auto, on, or off")
			}
		}
	}
	if update.Refinement != nil {
		if update.Refinement.Enabled != nil {
			next.Refinement.Enabled = *update.Refinement.Enabled
		}
		if update.Refinement.MaxConcurrent != nil {
			if *update.Refinement.MaxConcurrent <= 0 {
				return m.cfg, errors.New("refinement.max_concurrent must be > 0")
			}
			next.Refinement.MaxConcurrent = *update.Refinement.MaxConcurrent
		}
		if update.Refinement.DetailFFTSize != nil {
			v := *update.Refinement.DetailFFTSize
			if v <= 0 {
				return m.cfg, errors.New("refinement.detail_fft_size must be > 0")
			}
			if v&(v-1) != 0 {
				return m.cfg, errors.New("refinement.detail_fft_size must be a power of 2")
			}
			next.Refinement.DetailFFTSize = v
		}
		if update.Refinement.MinCandidateSNRDb != nil {
			next.Refinement.MinCandidateSNRDb = *update.Refinement.MinCandidateSNRDb
		}
		if update.Refinement.MinSpanHz != nil {
			if *update.Refinement.MinSpanHz < 0 {
				return m.cfg, errors.New("refinement.min_span_hz must be >= 0")
			}
			next.Refinement.MinSpanHz = *update.Refinement.MinSpanHz
		}
		if update.Refinement.MaxSpanHz != nil {
			if *update.Refinement.MaxSpanHz < 0 {
				return m.cfg, errors.New("refinement.max_span_hz must be >= 0")
			}
			next.Refinement.MaxSpanHz = *update.Refinement.MaxSpanHz
		}
		if update.Refinement.AutoSpan != nil {
			next.Refinement.AutoSpan = update.Refinement.AutoSpan
		}
		if next.Refinement.MaxSpanHz > 0 && next.Refinement.MinSpanHz > next.Refinement.MaxSpanHz {
			return m.cfg, errors.New("refinement.min_span_hz must be <= refinement.max_span_hz")
		}
	}
	if update.Resources != nil {
		if update.Resources.PreferGPU != nil {
			next.Resources.PreferGPU = *update.Resources.PreferGPU
		}
		if update.Resources.MaxRefinementJobs != nil {
			if *update.Resources.MaxRefinementJobs <= 0 {
				return m.cfg, errors.New("resources.max_refinement_jobs must be > 0")
			}
			next.Resources.MaxRefinementJobs = *update.Resources.MaxRefinementJobs
		}
		if update.Resources.MaxRecordingStreams != nil {
			if *update.Resources.MaxRecordingStreams <= 0 {
				return m.cfg, errors.New("resources.max_recording_streams must be > 0")
			}
			next.Resources.MaxRecordingStreams = *update.Resources.MaxRecordingStreams
		}
		if update.Resources.MaxDecodeJobs != nil {
			if *update.Resources.MaxDecodeJobs <= 0 {
				return m.cfg, errors.New("resources.max_decode_jobs must be > 0")
			}
			next.Resources.MaxDecodeJobs = *update.Resources.MaxDecodeJobs
		}
		if update.Resources.DecisionHoldMs != nil {
			if *update.Resources.DecisionHoldMs < 0 {
				return m.cfg, errors.New("resources.decision_hold_ms must be >= 0")
			}
			next.Resources.DecisionHoldMs = *update.Resources.DecisionHoldMs
		}
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
		if update.Detector.CFARGuardHz != nil {
			if *update.Detector.CFARGuardHz < 0 {
				return m.cfg, errors.New("cfar_guard_hz must be >= 0")
			}
			next.Detector.CFARGuardHz = *update.Detector.CFARGuardHz
		}
		if update.Detector.CFARTrainHz != nil {
			if *update.Detector.CFARTrainHz <= 0 {
				return m.cfg, errors.New("cfar_train_hz must be > 0")
			}
			next.Detector.CFARTrainHz = *update.Detector.CFARTrainHz
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
		if update.Detector.ClassHistorySize != nil {
			if *update.Detector.ClassHistorySize < 1 {
				return m.cfg, errors.New("class_history_size must be >= 1")
			}
			next.Detector.ClassHistorySize = *update.Detector.ClassHistorySize
		}
		if update.Detector.ClassSwitchRatio != nil {
			v := *update.Detector.ClassSwitchRatio
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0.1 || v > 1.0 {
				return m.cfg, errors.New("class_switch_ratio must be between 0.1 and 1.0")
			}
			next.Detector.ClassSwitchRatio = v
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

func validateMonitorWindows(windows []config.MonitorWindow) error {
	for i, w := range windows {
		if !isValidMonitorZone(w.Zone) {
			return fmt.Errorf("monitor_windows[%d] zone is invalid", i)
		}
		if math.IsNaN(w.Priority) || math.IsInf(w.Priority, 0) || w.Priority < -1 || w.Priority > 1 {
			return fmt.Errorf("monitor_windows[%d] priority must be between -1 and 1", i)
		}
		hasStart := w.StartHz != 0 || w.EndHz != 0
		if hasStart {
			if w.StartHz <= 0 || w.EndHz <= 0 || w.EndHz <= w.StartHz {
				return fmt.Errorf("monitor_windows[%d] requires start_hz < end_hz", i)
			}
			continue
		}
		if w.CenterHz <= 0 {
			return fmt.Errorf("monitor_windows[%d] requires center_hz when start/end not set", i)
		}
		if w.SpanHz <= 0 {
			return fmt.Errorf("monitor_windows[%d] requires span_hz > 0 when start/end not set", i)
		}
	}
	return nil
}

func isValidMonitorZone(zone string) bool {
	zone = strings.ToLower(strings.TrimSpace(zone))
	if zone == "" {
		return true
	}
	switch zone {
	case "neutral", "monitor", "default", "focus", "priority", "hot",
		"record", "recording", "record-only",
		"decode", "decoding", "decode-only",
		"background", "bg", "defer":
		return true
	default:
		return false
	}
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

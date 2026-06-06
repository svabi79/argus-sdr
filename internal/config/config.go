package config

import (
	"math"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Band struct {
	Name    string  `yaml:"name" json:"name"`
	StartHz float64 `yaml:"start_hz" json:"start_hz"`
	EndHz   float64 `yaml:"end_hz" json:"end_hz"`
}

type MonitorWindow struct {
	Label      string  `yaml:"label" json:"label"`
	Zone       string  `yaml:"zone" json:"zone"`
	StartHz    float64 `yaml:"start_hz" json:"start_hz"`
	EndHz      float64 `yaml:"end_hz" json:"end_hz"`
	CenterHz   float64 `yaml:"center_hz" json:"center_hz"`
	SpanHz     float64 `yaml:"span_hz" json:"span_hz"`
	Priority   float64 `yaml:"priority" json:"priority"`
	AutoRecord bool    `yaml:"auto_record" json:"auto_record"`
	AutoDecode bool    `yaml:"auto_decode" json:"auto_decode"`
}

type DetectorConfig struct {
	ThresholdDb      float64 `yaml:"threshold_db" json:"threshold_db"`
	MinDurationMs    int     `yaml:"min_duration_ms" json:"min_duration_ms"`
	HoldMs           int     `yaml:"hold_ms" json:"hold_ms"`
	EmaAlpha         float64 `yaml:"ema_alpha" json:"ema_alpha"`
	HysteresisDb     float64 `yaml:"hysteresis_db" json:"hysteresis_db"`
	MinStableFrames  int     `yaml:"min_stable_frames" json:"min_stable_frames"`
	GapToleranceMs   int     `yaml:"gap_tolerance_ms" json:"gap_tolerance_ms"`
	CFARMode         string  `yaml:"cfar_mode" json:"cfar_mode"`
	CFARGuardHz      float64 `yaml:"cfar_guard_hz" json:"cfar_guard_hz"`
	CFARTrainHz      float64 `yaml:"cfar_train_hz" json:"cfar_train_hz"`
	CFARGuardCells   int     `yaml:"cfar_guard_cells,omitempty" json:"cfar_guard_cells,omitempty"`
	CFARTrainCells   int     `yaml:"cfar_train_cells,omitempty" json:"cfar_train_cells,omitempty"`
	CFARRank         int     `yaml:"cfar_rank" json:"cfar_rank"`
	CFARScaleDb      float64 `yaml:"cfar_scale_db" json:"cfar_scale_db"`
	CFARWrapAround   bool    `yaml:"cfar_wrap_around" json:"cfar_wrap_around"`
	EdgeMarginDb     float64 `yaml:"edge_margin_db" json:"edge_margin_db"`
	MaxSignalBwHz    float64 `yaml:"max_signal_bw_hz" json:"max_signal_bw_hz"`
	MergeGapHz       float64 `yaml:"merge_gap_hz" json:"merge_gap_hz"`
	ClassHistorySize int     `yaml:"class_history_size" json:"class_history_size"`
	ClassSwitchRatio float64 `yaml:"class_switch_ratio" json:"class_switch_ratio"`

	// Deprecated (backward compatibility)
	CFAREnabled *bool `yaml:"cfar_enabled,omitempty" json:"cfar_enabled,omitempty"`
}

type LogConfig struct {
	Level       string   `yaml:"level" json:"level"`
	Categories  []string `yaml:"categories" json:"categories"`
	RateLimitMs int      `yaml:"rate_limit_ms" json:"rate_limit_ms"`
	Stdout      bool     `yaml:"stdout" json:"stdout"`
	StdoutColor bool     `yaml:"stdout_color" json:"stdout_color"`
	File        string   `yaml:"file" json:"file"`
	FileLevel   string   `yaml:"file_level" json:"file_level"`
	TimeFormat  string   `yaml:"time_format" json:"time_format"`
	DisableTime bool     `yaml:"disable_time" json:"disable_time"`
}

type RecorderConfig struct {
	Enabled     bool     `yaml:"enabled" json:"enabled"`
	MinSNRDb    float64  `yaml:"min_snr_db" json:"min_snr_db"`
	MinDuration string   `yaml:"min_duration" json:"min_duration"`
	MaxDuration string   `yaml:"max_duration" json:"max_duration"`
	PrerollMs   int      `yaml:"preroll_ms" json:"preroll_ms"`
	RecordIQ    bool     `yaml:"record_iq" json:"record_iq"`
	RecordAudio bool     `yaml:"record_audio" json:"record_audio"`
	AutoDemod   bool     `yaml:"auto_demod" json:"auto_demod"`
	AutoDecode  bool     `yaml:"auto_decode" json:"auto_decode"`
	MaxDiskMB   int      `yaml:"max_disk_mb" json:"max_disk_mb"`
	OutputDir   string   `yaml:"output_dir" json:"output_dir"`
	ClassFilter []string `yaml:"class_filter" json:"class_filter"`
	RingSeconds int      `yaml:"ring_seconds" json:"ring_seconds"`

	// Audio quality settings (AQ-2, AQ-3, AQ-5)
	DeemphasisUs     float64 `yaml:"deemphasis_us" json:"deemphasis_us"`             // De-emphasis time constant in µs. 50=Europe, 75=US/Japan, 0=disabled. Default: 50
	ExtractionTaps   int     `yaml:"extraction_fir_taps" json:"extraction_fir_taps"` // FIR tap count for extraction filter. Default: 101, max 301
	ExtractionBwMult float64 `yaml:"extraction_bw_mult" json:"extraction_bw_mult"`   // BW multiplier for extraction. Default: 1.2 (20% wider than detected)
	DebugLiveAudio   bool    `yaml:"debug_live_audio" json:"debug_live_audio"`
}

type DecoderConfig struct {
	FT8Cmd   string `yaml:"ft8_cmd" json:"ft8_cmd"`
	WSPRCmd  string `yaml:"wspr_cmd" json:"wspr_cmd"`
	DMRCmd   string `yaml:"dmr_cmd" json:"dmr_cmd"`
	DStarCmd string `yaml:"dstar_cmd" json:"dstar_cmd"`
	FSKCmd   string `yaml:"fsk_cmd" json:"fsk_cmd"`
	PSKCmd   string `yaml:"psk_cmd" json:"psk_cmd"`
}

type DebugConfig struct {
	AudioDumpEnabled bool `yaml:"audio_dump_enabled" json:"audio_dump_enabled"`
	CPUMonitoring    bool `yaml:"cpu_monitoring" json:"cpu_monitoring"`
	Telemetry        TelemetryConfig `yaml:"telemetry" json:"telemetry"`
}

type TelemetryConfig struct {
	Enabled           bool   `yaml:"enabled" json:"enabled"`
	HeavyEnabled      bool   `yaml:"heavy_enabled" json:"heavy_enabled"`
	HeavySampleEvery  int    `yaml:"heavy_sample_every" json:"heavy_sample_every"`
	MetricSampleEvery int    `yaml:"metric_sample_every" json:"metric_sample_every"`
	MetricHistoryMax  int    `yaml:"metric_history_max" json:"metric_history_max"`
	EventHistoryMax   int    `yaml:"event_history_max" json:"event_history_max"`
	RetentionSeconds  int    `yaml:"retention_seconds" json:"retention_seconds"`
	PersistEnabled    bool   `yaml:"persist_enabled" json:"persist_enabled"`
	PersistDir        string `yaml:"persist_dir" json:"persist_dir"`
	RotateMB          int    `yaml:"rotate_mb" json:"rotate_mb"`
	KeepFiles         int    `yaml:"keep_files" json:"keep_files"`
}

type PipelineGoalConfig struct {
	Intent            string          `yaml:"intent" json:"intent"`
	MonitorStartHz    float64         `yaml:"monitor_start_hz" json:"monitor_start_hz"`
	MonitorEndHz      float64         `yaml:"monitor_end_hz" json:"monitor_end_hz"`
	MonitorSpanHz     float64         `yaml:"monitor_span_hz" json:"monitor_span_hz"`
	MonitorWindows    []MonitorWindow `yaml:"monitor_windows" json:"monitor_windows"`
	SignalPriorities  []string        `yaml:"signal_priorities" json:"signal_priorities"`
	AutoRecordClasses []string        `yaml:"auto_record_classes" json:"auto_record_classes"`
	AutoDecodeClasses []string        `yaml:"auto_decode_classes" json:"auto_decode_classes"`
}

type PipelineConfig struct {
	Mode    string             `yaml:"mode" json:"mode"`
	Profile string             `yaml:"profile,omitempty" json:"profile,omitempty"`
	Goals   PipelineGoalConfig `yaml:"goals" json:"goals"`
}

type SurveillanceConfig struct {
	AnalysisFFTSize  int    `yaml:"analysis_fft_size" json:"analysis_fft_size"`
	FrameRate        int    `yaml:"frame_rate" json:"frame_rate"`
	Strategy         string `yaml:"strategy" json:"strategy"`
	DisplayBins      int    `yaml:"display_bins" json:"display_bins"`
	DisplayFPS       int    `yaml:"display_fps" json:"display_fps"`
	DerivedDetection string `yaml:"derived_detection" json:"derived_detection"`
	// WelchSegments averages this many overlapping FFT segments (Welch's method)
	// for the surveillance spectrum, cutting noise variance for better low-SNR
	// detection/bandwidth. 0/1 = single FFT (legacy, GPU-capable).
	WelchSegments int `yaml:"welch_segments" json:"welch_segments"`
}

type RefinementConfig struct {
	Enabled           bool    `yaml:"enabled" json:"enabled"`
	MaxConcurrent     int     `yaml:"max_concurrent" json:"max_concurrent"`
	DetailFFTSize     int     `yaml:"detail_fft_size" json:"detail_fft_size"`
	MinCandidateSNRDb float64 `yaml:"min_candidate_snr_db" json:"min_candidate_snr_db"`
	MinSpanHz         float64 `yaml:"min_span_hz" json:"min_span_hz"`
	MaxSpanHz         float64 `yaml:"max_span_hz" json:"max_span_hz"`
	AutoSpan          *bool   `yaml:"auto_span" json:"auto_span"`
	// OccupiedBwFraction enables occupied-bandwidth re-estimation in refinement
	// (power-containment fraction, e.g. 0.99). 0 = default (0.99); <0 disables.
	OccupiedBwFraction float64 `yaml:"occupied_bw_fraction" json:"occupied_bw_fraction"`
}

type ResourceConfig struct {
	PreferGPU           bool `yaml:"prefer_gpu" json:"prefer_gpu"`
	MaxRefinementJobs   int  `yaml:"max_refinement_jobs" json:"max_refinement_jobs"`
	MaxRecordingStreams int  `yaml:"max_recording_streams" json:"max_recording_streams"`
	MaxDecodeJobs       int  `yaml:"max_decode_jobs" json:"max_decode_jobs"`
	DecisionHoldMs      int  `yaml:"decision_hold_ms" json:"decision_hold_ms"`
}

type ProfileConfig struct {
	Name         string              `yaml:"name" json:"name"`
	Description  string              `yaml:"description" json:"description"`
	Pipeline     *PipelineConfig     `yaml:"pipeline,omitempty" json:"pipeline,omitempty"`
	Surveillance *SurveillanceConfig `yaml:"surveillance,omitempty" json:"surveillance,omitempty"`
	Refinement   *RefinementConfig   `yaml:"refinement,omitempty" json:"refinement,omitempty"`
	Resources    *ResourceConfig     `yaml:"resources,omitempty" json:"resources,omitempty"`
}

type Config struct {
	Bands          []Band             `yaml:"bands" json:"bands"`
	CenterHz       float64            `yaml:"center_hz" json:"center_hz"`
	SampleRate     int                `yaml:"sample_rate" json:"sample_rate"`
	FFTSize        int                `yaml:"fft_size" json:"fft_size"`
	GainDb         float64            `yaml:"gain_db" json:"gain_db"`
	TunerBwKHz     int                `yaml:"tuner_bw_khz" json:"tuner_bw_khz"`
	UseGPUFFT      bool               `yaml:"use_gpu_fft" json:"use_gpu_fft"`
	ClassifierMode string             `yaml:"classifier_mode" json:"classifier_mode"`
	AGC            bool               `yaml:"agc" json:"agc"`
	DCBlock        bool               `yaml:"dc_block" json:"dc_block"`
	IQBalance      bool               `yaml:"iq_balance" json:"iq_balance"`
	Pipeline       PipelineConfig     `yaml:"pipeline" json:"pipeline"`
	Surveillance   SurveillanceConfig `yaml:"surveillance" json:"surveillance"`
	Refinement     RefinementConfig   `yaml:"refinement" json:"refinement"`
	Resources      ResourceConfig     `yaml:"resources" json:"resources"`
	Profiles       []ProfileConfig    `yaml:"profiles" json:"profiles"`
	Detector       DetectorConfig     `yaml:"detector" json:"detector"`
	Recorder       RecorderConfig     `yaml:"recorder" json:"recorder"`
	Decoder        DecoderConfig      `yaml:"decoder" json:"decoder"`
	Debug          DebugConfig        `yaml:"debug" json:"debug"`
	Logging        LogConfig          `yaml:"logging" json:"logging"`
	WebAddr        string             `yaml:"web_addr" json:"web_addr"`
	EventPath      string             `yaml:"event_path" json:"event_path"`
	FrameRate      int                `yaml:"frame_rate" json:"frame_rate"`
	WaterfallLines int                `yaml:"waterfall_lines" json:"waterfall_lines"`
	WebRoot        string             `yaml:"web_root" json:"web_root"`
}

func Default() Config {
	return Config{
		Bands: []Band{
			{Name: "example", StartHz: 99.5e6, EndHz: 100.5e6},
		},
		CenterHz:       100.0e6,
		SampleRate:     2_048_000,
		FFTSize:        2048,
		GainDb:         30,
		TunerBwKHz:     1536,
		UseGPUFFT:      false,
		ClassifierMode: "combined",
		AGC:            false,
		DCBlock:        false,
		IQBalance:      false,
		Pipeline: PipelineConfig{
			Mode: "legacy",
			Goals: PipelineGoalConfig{
				Intent: "general-monitoring",
			},
		},
		Surveillance: SurveillanceConfig{
			AnalysisFFTSize:  2048,
			FrameRate:        15,
			Strategy:         "single-resolution",
			DisplayBins:      2048,
			DisplayFPS:       15,
			DerivedDetection: "auto",
		},
		Refinement: RefinementConfig{
			Enabled:           true,
			MaxConcurrent:     8,
			DetailFFTSize:     0,
			MinCandidateSNRDb: 0,
			MinSpanHz:         0,
			MaxSpanHz:         0,
			AutoSpan:          boolPtr(true),
		},
		Resources: ResourceConfig{
			PreferGPU:           true,
			MaxRefinementJobs:   8,
			MaxRecordingStreams: 16,
			MaxDecodeJobs:       16,
			DecisionHoldMs:      2000,
		},
		Profiles: []ProfileConfig{
			{
				Name:        "legacy",
				Description: "Current single-band pipeline behavior",
				Pipeline:    &PipelineConfig{Mode: "legacy", Profile: "legacy", Goals: PipelineGoalConfig{Intent: "general-monitoring"}},
				Surveillance: &SurveillanceConfig{
					AnalysisFFTSize:  2048,
					FrameRate:        15,
					Strategy:         "single-resolution",
					DisplayBins:      2048,
					DisplayFPS:       15,
					DerivedDetection: "auto",
				},
				Refinement: &RefinementConfig{
					Enabled:           true,
					MaxConcurrent:     8,
					DetailFFTSize:     0,
					MinCandidateSNRDb: 0,
					MinSpanHz:         0,
					MaxSpanHz:         0,
					AutoSpan:          boolPtr(true),
				},
				Resources: &ResourceConfig{
					PreferGPU:           false,
					MaxRefinementJobs:   8,
					MaxRecordingStreams: 16,
					MaxDecodeJobs:       16,
					DecisionHoldMs:      2000,
				},
			},
			{
				Name:        "wideband-balanced",
				Description: "Baseline multi-resolution wideband surveillance",
				Pipeline: &PipelineConfig{Mode: "wideband-balanced", Profile: "wideband-balanced", Goals: PipelineGoalConfig{
					Intent:           "wideband-surveillance",
					SignalPriorities: []string{"digital", "wfm"},
				}},
				Surveillance: &SurveillanceConfig{
					AnalysisFFTSize:  4096,
					FrameRate:        12,
					Strategy:         "multi-resolution",
					DisplayBins:      2048,
					DisplayFPS:       12,
					DerivedDetection: "auto",
				},
				Refinement: &RefinementConfig{
					Enabled:           true,
					MaxConcurrent:     16,
					DetailFFTSize:     0,
					MinCandidateSNRDb: 0,
					MinSpanHz:         4000,
					MaxSpanHz:         200000,
					AutoSpan:          boolPtr(true),
				},
				Resources: &ResourceConfig{
					PreferGPU:           true,
					MaxRefinementJobs:   16,
					MaxRecordingStreams: 16,
					MaxDecodeJobs:       12,
					DecisionHoldMs:      2000,
				},
			},
			{
				Name:        "wideband-aggressive",
				Description: "Higher surveillance/refinement budgets for dense wideband monitoring",
				Pipeline: &PipelineConfig{Mode: "wideband-aggressive", Profile: "wideband-aggressive", Goals: PipelineGoalConfig{
					Intent:           "high-density-wideband-surveillance",
					SignalPriorities: []string{"digital", "wfm", "trunk"},
				}},
				Surveillance: &SurveillanceConfig{
					AnalysisFFTSize:  8192,
					FrameRate:        10,
					Strategy:         "multi-resolution",
					DisplayBins:      4096,
					DisplayFPS:       10,
					DerivedDetection: "auto",
				},
				Refinement: &RefinementConfig{
					Enabled:           true,
					MaxConcurrent:     32,
					DetailFFTSize:     0,
					MinCandidateSNRDb: 0,
					MinSpanHz:         6000,
					MaxSpanHz:         250000,
					AutoSpan:          boolPtr(true),
				},
				Resources: &ResourceConfig{
					PreferGPU:           true,
					MaxRefinementJobs:   32,
					MaxRecordingStreams: 24,
					MaxDecodeJobs:       16,
					DecisionHoldMs:      2000,
				},
			},
			{
				Name:        "archive",
				Description: "Record-first monitoring profile",
				Pipeline: &PipelineConfig{Mode: "archive", Profile: "archive", Goals: PipelineGoalConfig{
					Intent:           "archive-and-triage",
					SignalPriorities: []string{"wfm", "nfm", "digital"},
				}},
				Surveillance: &SurveillanceConfig{
					AnalysisFFTSize:  4096,
					FrameRate:        12,
					Strategy:         "single-resolution",
					DisplayBins:      2048,
					DisplayFPS:       12,
					DerivedDetection: "auto",
				},
				Refinement: &RefinementConfig{
					Enabled:           true,
					MaxConcurrent:     12,
					DetailFFTSize:     0,
					MinCandidateSNRDb: 0,
					MinSpanHz:         4000,
					MaxSpanHz:         200000,
					AutoSpan:          boolPtr(true),
				},
				Resources: &ResourceConfig{
					PreferGPU:           true,
					MaxRefinementJobs:   12,
					MaxRecordingStreams: 24,
					MaxDecodeJobs:       12,
					DecisionHoldMs:      2500,
				},
			},
			{
				Name:        "digital-hunting",
				Description: "Digital-first refinement and decode focus",
				Pipeline: &PipelineConfig{Mode: "digital-hunting", Profile: "digital-hunting", Goals: PipelineGoalConfig{
					Intent:           "digital-surveillance",
					SignalPriorities: []string{"ft8", "wspr", "fsk", "psk", "dmr"},
				}},
				Surveillance: &SurveillanceConfig{
					AnalysisFFTSize:  4096,
					FrameRate:        12,
					Strategy:         "multi-resolution",
					DisplayBins:      2048,
					DisplayFPS:       12,
					DerivedDetection: "auto",
				},
				Refinement: &RefinementConfig{
					Enabled:           true,
					MaxConcurrent:     16,
					DetailFFTSize:     0,
					MinCandidateSNRDb: 0,
					MinSpanHz:         3000,
					MaxSpanHz:         120000,
					AutoSpan:          boolPtr(true),
				},
				Resources: &ResourceConfig{
					PreferGPU:           true,
					MaxRefinementJobs:   16,
					MaxRecordingStreams: 12,
					MaxDecodeJobs:       16,
					DecisionHoldMs:      2000,
				},
			},
		},
		Detector: DetectorConfig{
			ThresholdDb:      -20,
			MinDurationMs:    250,
			HoldMs:           500,
			EmaAlpha:         0.2,
			HysteresisDb:     3,
			MinStableFrames:  3,
			GapToleranceMs:   500,
			CFARMode:         "GOSCA",
			CFARGuardHz:      500,
			CFARTrainHz:      5000,
			CFARGuardCells:   3,
			CFARTrainCells:   24,
			CFARRank:         36,
			CFARScaleDb:      6,
			CFARWrapAround:   true,
			EdgeMarginDb:     3.0,
			MaxSignalBwHz:    150000,
			MergeGapHz:       5000,
			ClassHistorySize: 10,
			ClassSwitchRatio: 0.6,
		},
		Recorder: RecorderConfig{
			Enabled:          false,
			MinSNRDb:         10,
			MinDuration:      "1s",
			MaxDuration:      "300s",
			PrerollMs:        500,
			RecordIQ:         true,
			RecordAudio:      false,
			AutoDemod:        true,
			AutoDecode:       false,
			MaxDiskMB:        0,
			OutputDir:        "data/recordings",
			RingSeconds:      8,
			DeemphasisUs:     50,
			ExtractionTaps:   101,
			ExtractionBwMult: 1.2,
		},
		Decoder:        DecoderConfig{},
		Debug: DebugConfig{
			AudioDumpEnabled: false,
			CPUMonitoring:    false,
			Telemetry: TelemetryConfig{
				Enabled:           true,
				HeavyEnabled:      false,
				HeavySampleEvery:  12,
				MetricSampleEvery: 2,
				MetricHistoryMax:  12000,
				EventHistoryMax:   4000,
				RetentionSeconds:  900,
				PersistEnabled:    false,
				PersistDir:        "debug/telemetry",
				RotateMB:          16,
				KeepFiles:         8,
			},
		},
		Logging: LogConfig{
			Level:       "informal",
			Categories:  []string{},
			RateLimitMs: 500,
			Stdout:      true,
			StdoutColor: true,
			File:        "logs/trace.log",
			FileLevel:   "",
			TimeFormat:  "15:04:05",
			DisableTime: false,
		},
		WebAddr: ":8080",
		EventPath:      "data/events.jsonl",
		FrameRate:      15,
		WaterfallLines: 200,
		WebRoot:        "web",
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if b, err := os.ReadFile(autosavePath(path)); err == nil {
		if err := yaml.Unmarshal(b, &cfg); err == nil {
			return applyDefaults(cfg), nil
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return applyDefaults(cfg), nil
}

func applyDefaults(cfg Config) Config {
	if cfg.Detector.MinDurationMs <= 0 {
		cfg.Detector.MinDurationMs = 250
	}
	if cfg.Detector.HoldMs <= 0 {
		cfg.Detector.HoldMs = 500
	}
	if cfg.Detector.MinStableFrames <= 0 {
		cfg.Detector.MinStableFrames = 3
	}
	if cfg.Detector.GapToleranceMs <= 0 {
		cfg.Detector.GapToleranceMs = cfg.Detector.HoldMs
	}
	if cfg.Detector.CFARMode == "" {
		if cfg.Detector.CFAREnabled != nil {
			if *cfg.Detector.CFAREnabled {
				cfg.Detector.CFARMode = "OS"
			} else {
				cfg.Detector.CFARMode = "OFF"
			}
		} else {
			cfg.Detector.CFARMode = "GOSCA"
		}
	}
	if cfg.Detector.CFARGuardHz <= 0 && cfg.Detector.CFARGuardCells > 0 {
		cfg.Detector.CFARGuardHz = float64(cfg.Detector.CFARGuardCells) * 62.5
	}
	if cfg.Detector.CFARTrainHz <= 0 && cfg.Detector.CFARTrainCells > 0 {
		cfg.Detector.CFARTrainHz = float64(cfg.Detector.CFARTrainCells) * 62.5
	}
	if cfg.Detector.CFARGuardHz <= 0 {
		cfg.Detector.CFARGuardHz = 500
	}
	if cfg.Detector.CFARTrainHz <= 0 {
		cfg.Detector.CFARTrainHz = 5000
	}
	if cfg.Detector.CFARGuardCells <= 0 {
		cfg.Detector.CFARGuardCells = 3
	}
	if cfg.Detector.CFARTrainCells <= 0 {
		cfg.Detector.CFARTrainCells = 24
	}
	if cfg.Detector.CFARRank <= 0 || cfg.Detector.CFARRank > 2*cfg.Detector.CFARTrainCells {
		cfg.Detector.CFARRank = int(math.Round(0.75 * float64(2*cfg.Detector.CFARTrainCells)))
		if cfg.Detector.CFARRank <= 0 {
			cfg.Detector.CFARRank = 1
		}
	}
	if cfg.Detector.CFARScaleDb <= 0 {
		cfg.Detector.CFARScaleDb = 6
	}
	if cfg.Detector.EdgeMarginDb <= 0 {
		cfg.Detector.EdgeMarginDb = 3.0
	}
	if cfg.Detector.MaxSignalBwHz <= 0 {
		cfg.Detector.MaxSignalBwHz = 150000
	}
	if cfg.Detector.MergeGapHz <= 0 {
		cfg.Detector.MergeGapHz = 5000
	}
	if cfg.Detector.ClassHistorySize <= 0 {
		cfg.Detector.ClassHistorySize = 10
	}
	if cfg.Detector.ClassSwitchRatio <= 0 || cfg.Detector.ClassSwitchRatio > 1 {
		cfg.Detector.ClassSwitchRatio = 0.6
	}
	if cfg.Pipeline.Mode == "" {
		cfg.Pipeline.Mode = "legacy"
	}
	if cfg.Pipeline.Goals.Intent == "" {
		cfg.Pipeline.Goals.Intent = "general-monitoring"
	}
	if cfg.Pipeline.Goals.MonitorSpanHz <= 0 && cfg.Pipeline.Goals.MonitorStartHz != 0 && cfg.Pipeline.Goals.MonitorEndHz != 0 && cfg.Pipeline.Goals.MonitorEndHz > cfg.Pipeline.Goals.MonitorStartHz {
		cfg.Pipeline.Goals.MonitorSpanHz = cfg.Pipeline.Goals.MonitorEndHz - cfg.Pipeline.Goals.MonitorStartHz
	}
	if cfg.Surveillance.AnalysisFFTSize <= 0 {
		cfg.Surveillance.AnalysisFFTSize = cfg.FFTSize
	}
	if cfg.Surveillance.FrameRate <= 0 {
		cfg.Surveillance.FrameRate = cfg.FrameRate
	}
	if cfg.Surveillance.Strategy == "" {
		cfg.Surveillance.Strategy = "single-resolution"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "informal"
	}
	if cfg.Logging.RateLimitMs <= 0 {
		cfg.Logging.RateLimitMs = 500
	}
	if cfg.Logging.File == "" {
		cfg.Logging.File = "logs/trace.log"
	}
	if cfg.Surveillance.DerivedDetection == "" {
		cfg.Surveillance.DerivedDetection = "auto"
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Surveillance.DerivedDetection)) {
	case "auto", "on", "off", "true", "false", "enabled", "disabled", "enable", "disable":
	default:
		cfg.Surveillance.DerivedDetection = "auto"
	}
	if cfg.Surveillance.DisplayBins <= 0 {
		cfg.Surveillance.DisplayBins = cfg.FFTSize
	}
	if cfg.Surveillance.DisplayFPS <= 0 {
		cfg.Surveillance.DisplayFPS = cfg.FrameRate
	}
	if !cfg.Refinement.Enabled {
		// keep explicit false if user disabled it; enable by default only when unset-like zero config
		if cfg.Refinement.MaxConcurrent == 0 && cfg.Refinement.MinCandidateSNRDb == 0 {
			cfg.Refinement.Enabled = true
		}
	}
	if cfg.Refinement.MaxConcurrent <= 0 {
		cfg.Refinement.MaxConcurrent = 8
	}
	if cfg.Refinement.DetailFFTSize <= 0 {
		cfg.Refinement.DetailFFTSize = cfg.Surveillance.AnalysisFFTSize
	}
	if cfg.Refinement.DetailFFTSize&(cfg.Refinement.DetailFFTSize-1) != 0 {
		cfg.Refinement.DetailFFTSize = cfg.Surveillance.AnalysisFFTSize
	}
	if cfg.Refinement.MinSpanHz < 0 {
		cfg.Refinement.MinSpanHz = 0
	}
	if cfg.Refinement.MaxSpanHz < 0 {
		cfg.Refinement.MaxSpanHz = 0
	}
	if cfg.Refinement.MaxSpanHz > 0 && cfg.Refinement.MinSpanHz > cfg.Refinement.MaxSpanHz {
		cfg.Refinement.MaxSpanHz = cfg.Refinement.MinSpanHz
	}
	if cfg.Refinement.AutoSpan == nil {
		cfg.Refinement.AutoSpan = boolPtr(true)
	}
	if cfg.Resources.MaxRefinementJobs <= 0 {
		cfg.Resources.MaxRefinementJobs = cfg.Refinement.MaxConcurrent
	}
	if cfg.Resources.MaxRecordingStreams <= 0 {
		cfg.Resources.MaxRecordingStreams = 16
	}
	if cfg.Resources.DecisionHoldMs < 0 {
		cfg.Resources.DecisionHoldMs = 0
	}
	if cfg.Resources.DecisionHoldMs == 0 {
		cfg.Resources.DecisionHoldMs = 2000
	}
	if cfg.FrameRate <= 0 {
		cfg.FrameRate = 15
	}
	if cfg.WaterfallLines <= 0 {
		cfg.WaterfallLines = 200
	}
	if cfg.WebRoot == "" {
		cfg.WebRoot = "web"
	}
	if cfg.WebAddr == "" {
		cfg.WebAddr = ":8080"
	}
	if cfg.EventPath == "" {
		cfg.EventPath = "data/events.jsonl"
	}
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 2_048_000
	}
	if cfg.ClassifierMode == "" {
		cfg.ClassifierMode = "combined"
	}
	switch cfg.ClassifierMode {
	case "rule", "math", "combined":
	default:
		cfg.ClassifierMode = "combined"
	}
	if cfg.FFTSize <= 0 {
		cfg.FFTSize = 2048
	}
	if cfg.Surveillance.AnalysisFFTSize > 0 {
		cfg.FFTSize = cfg.Surveillance.AnalysisFFTSize
	} else {
		cfg.Surveillance.AnalysisFFTSize = cfg.FFTSize
	}
	if cfg.TunerBwKHz <= 0 {
		cfg.TunerBwKHz = 1536
	}
	if cfg.CenterHz == 0 {
		cfg.CenterHz = 100.0e6
	}
	if cfg.Recorder.OutputDir == "" {
		cfg.Recorder.OutputDir = "data/recordings"
	}
	if cfg.Recorder.RingSeconds <= 0 {
		cfg.Recorder.RingSeconds = 8
	}
	if cfg.Recorder.DeemphasisUs == 0 {
		cfg.Recorder.DeemphasisUs = 50
	}
	if cfg.Recorder.ExtractionTaps <= 0 {
		cfg.Recorder.ExtractionTaps = 101
	}
	if cfg.Recorder.ExtractionTaps > 301 {
		cfg.Recorder.ExtractionTaps = 301
	}
	if cfg.Recorder.ExtractionTaps%2 == 0 {
		cfg.Recorder.ExtractionTaps++ // must be odd
	}
	if cfg.Recorder.ExtractionBwMult <= 0 {
		cfg.Recorder.ExtractionBwMult = 1.2
	}
	if cfg.Debug.Telemetry.HeavySampleEvery <= 0 {
		cfg.Debug.Telemetry.HeavySampleEvery = 12
	}
	if cfg.Debug.Telemetry.MetricSampleEvery <= 0 {
		cfg.Debug.Telemetry.MetricSampleEvery = 2
	}
	if cfg.Debug.Telemetry.MetricHistoryMax <= 0 {
		cfg.Debug.Telemetry.MetricHistoryMax = 12000
	}
	if cfg.Debug.Telemetry.EventHistoryMax <= 0 {
		cfg.Debug.Telemetry.EventHistoryMax = 4000
	}
	if cfg.Debug.Telemetry.RetentionSeconds <= 0 {
		cfg.Debug.Telemetry.RetentionSeconds = 900
	}
	if cfg.Debug.Telemetry.PersistDir == "" {
		cfg.Debug.Telemetry.PersistDir = "debug/telemetry"
	}
	if cfg.Debug.Telemetry.RotateMB <= 0 {
		cfg.Debug.Telemetry.RotateMB = 16
	}
	if cfg.Debug.Telemetry.KeepFiles <= 0 {
		cfg.Debug.Telemetry.KeepFiles = 8
	}
	return cfg
}

func (c Config) FrameInterval() time.Duration {
	fps := c.FrameRate
	if fps <= 0 {
		fps = 15
	}
	return time.Second / time.Duration(fps)
}

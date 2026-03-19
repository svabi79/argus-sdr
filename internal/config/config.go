package config

import (
	"math"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Band struct {
	Name    string  `yaml:"name" json:"name"`
	StartHz float64 `yaml:"start_hz" json:"start_hz"`
	EndHz   float64 `yaml:"end_hz" json:"end_hz"`
}

type DetectorConfig struct {
	ThresholdDb     float64 `yaml:"threshold_db" json:"threshold_db"`
	MinDurationMs   int     `yaml:"min_duration_ms" json:"min_duration_ms"`
	HoldMs          int     `yaml:"hold_ms" json:"hold_ms"`
	EmaAlpha        float64 `yaml:"ema_alpha" json:"ema_alpha"`
	HysteresisDb    float64 `yaml:"hysteresis_db" json:"hysteresis_db"`
	MinStableFrames int     `yaml:"min_stable_frames" json:"min_stable_frames"`
	GapToleranceMs  int     `yaml:"gap_tolerance_ms" json:"gap_tolerance_ms"`
	CFARMode        string  `yaml:"cfar_mode" json:"cfar_mode"`
	CFARGuardHz     float64 `yaml:"cfar_guard_hz" json:"cfar_guard_hz"`
	CFARTrainHz     float64 `yaml:"cfar_train_hz" json:"cfar_train_hz"`
	CFARGuardCells  int     `yaml:"cfar_guard_cells,omitempty" json:"cfar_guard_cells,omitempty"`
	CFARTrainCells  int     `yaml:"cfar_train_cells,omitempty" json:"cfar_train_cells,omitempty"`
	CFARRank        int     `yaml:"cfar_rank" json:"cfar_rank"`
	CFARScaleDb     float64 `yaml:"cfar_scale_db" json:"cfar_scale_db"`
	CFARWrapAround  bool    `yaml:"cfar_wrap_around" json:"cfar_wrap_around"`
	EdgeMarginDb    float64 `yaml:"edge_margin_db" json:"edge_margin_db"`
	MaxSignalBwHz   float64 `yaml:"max_signal_bw_hz" json:"max_signal_bw_hz"`
	MergeGapHz      float64 `yaml:"merge_gap_hz" json:"merge_gap_hz"`

	// Deprecated (backward compatibility)
	CFAREnabled *bool `yaml:"cfar_enabled,omitempty" json:"cfar_enabled,omitempty"`
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
}

type DecoderConfig struct {
	FT8Cmd   string `yaml:"ft8_cmd" json:"ft8_cmd"`
	WSPRCmd  string `yaml:"wspr_cmd" json:"wspr_cmd"`
	DMRCmd   string `yaml:"dmr_cmd" json:"dmr_cmd"`
	DStarCmd string `yaml:"dstar_cmd" json:"dstar_cmd"`
	FSKCmd   string `yaml:"fsk_cmd" json:"fsk_cmd"`
	PSKCmd   string `yaml:"psk_cmd" json:"psk_cmd"`
}

type Config struct {
	Bands          []Band         `yaml:"bands" json:"bands"`
	CenterHz       float64        `yaml:"center_hz" json:"center_hz"`
	SampleRate     int            `yaml:"sample_rate" json:"sample_rate"`
	FFTSize        int            `yaml:"fft_size" json:"fft_size"`
	GainDb         float64        `yaml:"gain_db" json:"gain_db"`
	TunerBwKHz     int            `yaml:"tuner_bw_khz" json:"tuner_bw_khz"`
	UseGPUFFT      bool           `yaml:"use_gpu_fft" json:"use_gpu_fft"`
	AGC            bool           `yaml:"agc" json:"agc"`
	DCBlock        bool           `yaml:"dc_block" json:"dc_block"`
	IQBalance      bool           `yaml:"iq_balance" json:"iq_balance"`
	Detector       DetectorConfig `yaml:"detector" json:"detector"`
	Recorder       RecorderConfig `yaml:"recorder" json:"recorder"`
	Decoder        DecoderConfig  `yaml:"decoder" json:"decoder"`
	WebAddr        string         `yaml:"web_addr" json:"web_addr"`
	EventPath      string         `yaml:"event_path" json:"event_path"`
	FrameRate      int            `yaml:"frame_rate" json:"frame_rate"`
	WaterfallLines int            `yaml:"waterfall_lines" json:"waterfall_lines"`
	WebRoot        string         `yaml:"web_root" json:"web_root"`
}

func Default() Config {
	return Config{
		Bands: []Band{
			{Name: "example", StartHz: 99.5e6, EndHz: 100.5e6},
		},
		CenterHz:   100.0e6,
		SampleRate: 2_048_000,
		FFTSize:    2048,
		GainDb:     30,
		TunerBwKHz: 1536,
		UseGPUFFT:  false,
		AGC:        false,
		DCBlock:    false,
		IQBalance:  false,
		Detector: DetectorConfig{
			ThresholdDb:     -20,
			MinDurationMs:   250,
			HoldMs:          500,
			EmaAlpha:        0.2,
			HysteresisDb:    3,
			MinStableFrames: 3,
			GapToleranceMs:  500,
			CFARMode:        "GOSCA",
			CFARGuardHz:     500,
			CFARTrainHz:     5000,
			CFARGuardCells:  3,
			CFARTrainCells:  24,
			CFARRank:        36,
			CFARScaleDb:     6,
			CFARWrapAround:  true,
			EdgeMarginDb:    3.0,
			MaxSignalBwHz:   150000,
			MergeGapHz:      5000,
		},
		Recorder: RecorderConfig{
			Enabled:     false,
			MinSNRDb:    10,
			MinDuration: "1s",
			MaxDuration: "300s",
			PrerollMs:   500,
			RecordIQ:    true,
			RecordAudio: false,
			AutoDemod:   true,
			AutoDecode:  false,
			MaxDiskMB:   0,
			OutputDir:   "data/recordings",
			RingSeconds: 8,
		},
		Decoder:        DecoderConfig{},
		WebAddr:        ":8080",
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
	if cfg.FFTSize <= 0 {
		cfg.FFTSize = 2048
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
	return cfg
}

func (c Config) FrameInterval() time.Duration {
	fps := c.FrameRate
	if fps <= 0 {
		fps = 15
	}
	return time.Second / time.Duration(fps)
}

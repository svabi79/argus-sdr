package config

import (
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
	ThresholdDb   float64 `yaml:"threshold_db" json:"threshold_db"`
	MinDurationMs int     `yaml:"min_duration_ms" json:"min_duration_ms"`
	HoldMs        int     `yaml:"hold_ms" json:"hold_ms"`
}

type RecorderConfig struct {
	Enabled     bool     `yaml:"enabled" json:"enabled"`
	MinSNRDb    float64  `yaml:"min_snr_db" json:"min_snr_db"`
	MinDuration string   `yaml:"min_duration" json:"min_duration"`
	MaxDuration string   `yaml:"max_duration" json:"max_duration"`
	PrerollMs   int      `yaml:"preroll_ms" json:"preroll_ms"`
	RecordIQ    bool     `yaml:"record_iq" json:"record_iq"`
	OutputDir   string   `yaml:"output_dir" json:"output_dir"`
	ClassFilter []string `yaml:"class_filter" json:"class_filter"`
	RingSeconds int      `yaml:"ring_seconds" json:"ring_seconds"`
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
		Detector:   DetectorConfig{ThresholdDb: -20, MinDurationMs: 250, HoldMs: 500},
		Recorder: RecorderConfig{
			Enabled:     false,
			MinSNRDb:    10,
			MinDuration: "1s",
			MaxDuration: "300s",
			PrerollMs:   500,
			RecordIQ:    true,
			OutputDir:   "data/recordings",
			RingSeconds: 8,
		},
		WebAddr:        ":8080",
		EventPath:      "data/events.jsonl",
		FrameRate:      15,
		WaterfallLines: 200,
		WebRoot:        "web",
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Detector.MinDurationMs <= 0 {
		cfg.Detector.MinDurationMs = 250
	}
	if cfg.Detector.HoldMs <= 0 {
		cfg.Detector.HoldMs = 500
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
	return cfg, nil
}

func (c Config) FrameInterval() time.Duration {
	fps := c.FrameRate
	if fps <= 0 {
		fps = 15
	}
	return time.Second / time.Duration(fps)
}

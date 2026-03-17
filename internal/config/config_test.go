package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	data := []byte("center_hz: 100.0e6\nfft_size: 1024\n")
	f, err := os.CreateTemp(t.TempDir(), "cfg*.yaml")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.CenterHz != 100.0e6 {
		t.Fatalf("center hz: %v", cfg.CenterHz)
	}
	if cfg.FFTSize != 1024 {
		t.Fatalf("fft size: %v", cfg.FFTSize)
	}
	if cfg.FrameRate <= 0 {
		t.Fatalf("frame rate default not applied")
	}
	if cfg.EventPath == "" {
		t.Fatalf("event path default not applied")
	}
}

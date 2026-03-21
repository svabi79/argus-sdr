package main

import (
	"testing"

	"sdr-wideband-suite/internal/config"
)

func TestSurveillanceDisplayDefaults(t *testing.T) {
	cfg := config.Default()
	if cfg.Surveillance.DisplayBins != cfg.FFTSize {
		t.Fatalf("expected display bins to default to fft size, got %d vs %d", cfg.Surveillance.DisplayBins, cfg.FFTSize)
	}
	if cfg.Surveillance.DisplayFPS != cfg.FrameRate {
		t.Fatalf("expected display fps to default to frame rate, got %d vs %d", cfg.Surveillance.DisplayFPS, cfg.FrameRate)
	}
}

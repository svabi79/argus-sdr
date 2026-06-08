package pipeline

import (
	"testing"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/detector"
)

func gsig(hz float64) detector.Signal { return detector.Signal{CenterHz: hz} }

func TestGateSignalsToBands(t *testing.T) {
	// 40m band of interest; the +50 dB AM at 7.88 and its OOB neighbours must go.
	bands := []config.Band{{Name: "40m", StartHz: 7.0e6, EndHz: 7.45e6}}
	in := []detector.Signal{gsig(6.4e6), gsig(7.0e6), gsig(7.1e6), gsig(7.45e6), gsig(7.88e6), gsig(8.0e6)}
	out := GateSignalsToBands(in, bands)
	if len(out) != 3 {
		t.Fatalf("expected 3 in-band (7.0/7.1/7.45 MHz inclusive edges), got %d: %+v", len(out), out)
	}
	for _, s := range out {
		if s.CenterHz < 7.0e6 || s.CenterHz > 7.45e6 {
			t.Errorf("out-of-band signal survived gate: %.3f MHz", s.CenterHz/1e6)
		}
	}
}

func TestGateSignalsToBandsEmptyIsNoOp(t *testing.T) {
	in := []detector.Signal{gsig(6.4e6), gsig(7.88e6)}
	if out := GateSignalsToBands(in, nil); len(out) != len(in) {
		t.Fatalf("empty bands must be a no-op: got %d want %d", len(out), len(in))
	}
}

func TestGateSignalsToBandsMultiBand(t *testing.T) {
	bands := []config.Band{
		{Name: "40m", StartHz: 7.0e6, EndHz: 7.2e6},
		{Name: "20m", StartHz: 14.0e6, EndHz: 14.35e6},
	}
	in := []detector.Signal{gsig(7.1e6), gsig(10e6), gsig(14.1e6), gsig(7.88e6)}
	if out := GateSignalsToBands(in, bands); len(out) != 2 {
		t.Fatalf("expected 2 (7.1 + 14.1 MHz), got %d", len(out))
	}
}

func TestGateSignalsToBandsIgnoresInvalidBand(t *testing.T) {
	// An inverted/zero-width band must match nothing, not everything (fail-closed).
	bands := []config.Band{{Name: "inverted", StartHz: 9e6, EndHz: 5e6}}
	if out := GateSignalsToBands([]detector.Signal{gsig(7e6)}, bands); len(out) != 0 {
		t.Fatalf("inverted band should match nothing, got %d", len(out))
	}
}

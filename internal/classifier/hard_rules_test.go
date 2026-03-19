package classifier

import (
	"math"
	"testing"
)

func TestHardRulesFMBroadcast(t *testing.T) {
	cls := TryHardRule(100.0e6, 120000)
	if cls == nil {
		t.Fatal("expected hard rule match for FM broadcast")
	}
	if cls.ModType != ClassWFM {
		t.Errorf("expected WFM, got %s", cls.ModType)
	}
	if cls.Confidence < 0.95 {
		t.Errorf("confidence too low: %.2f", cls.Confidence)
	}
	cls2 := TryHardRule(434.0e6, 120000)
	if cls2 == nil || cls2.ModType != ClassWFM {
		t.Errorf("expected WFM for >100kHz signal")
	}
}

func TestHardRulesAirband(t *testing.T) {
	cls := TryHardRule(121.5e6, 8000)
	if cls == nil || cls.ModType != ClassAM {
		t.Fatalf("expected AM for airband, got %v", cls)
	}
}

func TestHardRulesCW(t *testing.T) {
	cls := TryHardRule(7.020e6, 100)
	if cls == nil || cls.ModType != ClassCW {
		t.Fatalf("expected CW for <500Hz, got %v", cls)
	}
}

func TestWFMPilotDetection(t *testing.T) {
	sampleRate := 192000
	n := sampleRate * 2
	iq := make([]complex64, n)
	phase := 0.0
	for i := range iq {
		pilot := math.Sin(2 * math.Pi * 19000 * float64(i) / float64(sampleRate))
		modulation := pilot * 0.1
		freqDev := modulation * 75000 / float64(sampleRate)
		phase += 2 * math.Pi * freqDev
		iq[i] = complex(float32(math.Cos(phase)), float32(math.Sin(phase)))
	}
	result := EstimateExactFrequency(iq, sampleRate, 102.1e6, ClassWFM)
	if !result.Locked {
		t.Fatal("PLL should lock on pilot")
	}
	if math.Abs(result.OffsetHz) > 5 {
		t.Errorf("offset too large: %.1f Hz", result.OffsetHz)
	}
}

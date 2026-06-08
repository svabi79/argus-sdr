package detector

import (
	"math"
	"testing"
	"time"

	"sdr-wideband-suite/internal/config"
)

// TestMultiScaleDetectsAllWidths checks the multi-scale detector finds a narrow
// carrier (CW), a diffuse mid-width hump (SSB-like), and a wide flat-topped signal
// (WFM-like) on one spectrum with a single sensitivity setting — and keeps the
// wide signal as ONE detection rather than fragmenting it.
func TestMultiScaleDetectsAllWidths(t *testing.T) {
	const (
		fft        = 8192
		sampleRate = 2_000_000
	)
	binWidth := float64(sampleRate) / float64(fft) // ~244 Hz
	spec := make([]float64, fft)
	rng := uint32(12345)
	for i := range spec {
		// deterministic pseudo-random noise floor around -90 dB (~1 dB spread),
		// uncorrelated (a periodic floor would create a correlated residual).
		rng = rng*1664525 + 1013904223
		spec[i] = -90 + (float64(rng>>8&0xffff)/65535.0-0.5)*2.0
	}
	addPeak := func(centerBin, halfWidth int, peakDb float64, flat bool) {
		for o := -halfWidth; o <= halfWidth; o++ {
			b := centerBin + o
			if b < 0 || b >= fft {
				continue
			}
			var v float64
			if flat {
				v = peakDb // flat-topped (WFM MPX-like, filled)
			} else {
				// rounded hump
				x := float64(o) / float64(halfWidth+1)
				v = peakDb * (1 - x*x)
			}
			lin := math.Pow(10, spec[b]/10) + math.Pow(10, (-90+v)/10)
			spec[b] = 10 * math.Log10(lin)
		}
	}
	// CW: 1 bin, +35 dB. SSB: ~3 kHz (12 bins), +12 dB diffuse. WFM: ~200 kHz
	// (820 bins), +25 dB filled.
	cwBin, ssbBin, wfmBin := 1500, 3000, 5500
	addPeak(cwBin, 0, 35, false)
	addPeak(ssbBin, 6, 12, false)
	addPeak(wfmBin, 410, 25, true)

	cfg := config.DetectorConfig{
		ThresholdDb: -120, EmaAlpha: 1.0, HysteresisDb: 6, MinStableFrames: 1,
		MultiScale: true,
	}
	d := New(cfg, sampleRate, fft)
	_, sigs := d.Process(time.Unix(0, 0), spec, 0)

	if len(sigs) == 0 {
		t.Fatal("multi-scale found no signals")
	}
	if len(sigs) > 8 {
		t.Errorf("too many detections (flood): %d", len(sigs))
	}

	binOf := func(centerHz float64) int { return int(math.Round(centerHz/binWidth + float64(fft)/2)) }
	type hit struct {
		found bool
		bwHz  float64
	}
	near := func(targetBin, tol int) hit {
		for _, s := range sigs {
			b := binOf(s.CenterHz)
			if b >= targetBin-tol && b <= targetBin+tol {
				return hit{true, s.BWHz}
			}
		}
		return hit{}
	}

	cw := near(cwBin, 5)
	ssb := near(ssbBin, 8)
	wfm := near(wfmBin, 60)

	if !cw.found {
		t.Error("CW (narrow) not detected")
	}
	if !ssb.found {
		t.Error("SSB (diffuse mid-width) not detected — the case CFAR misses")
	} else if ssb.bwHz < 1000 || ssb.bwHz > 8000 {
		t.Errorf("SSB bandwidth off: got %.0f Hz, want ~3 kHz", ssb.bwHz)
	}
	if !wfm.found {
		t.Error("WFM (wide) not detected")
	} else if wfm.bwHz < 100000 {
		t.Errorf("WFM fragmented: bw %.0f Hz, want ~200 kHz (one signal)", wfm.bwHz)
	}
	t.Logf("detections=%d  CW=%v SSB=%vHz WFM=%vHz", len(sigs), cw.found, ssb.bwHz, wfm.bwHz)
}

// TestMultiScaleQuietWhenEmpty: pure noise should yield few/no detections.
func TestMultiScaleQuietWhenEmpty(t *testing.T) {
	const fft, sampleRate = 8192, 2_000_000
	spec := make([]float64, fft)
	for i := range spec {
		spec[i] = -90 + 0.8*math.Sin(float64(i)*0.37) + 0.6*math.Cos(float64(i)*1.13)
	}
	cfg := config.DetectorConfig{ThresholdDb: -120, EmaAlpha: 1.0, MinStableFrames: 1, MultiScale: true}
	d := New(cfg, sampleRate, fft)
	_, sigs := d.Process(time.Unix(0, 0), spec, 0)
	if len(sigs) > 3 {
		t.Errorf("multi-scale on pure noise produced %d detections (want ~0)", len(sigs))
	}
}

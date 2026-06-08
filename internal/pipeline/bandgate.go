package pipeline

import (
	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/detector"
)

// GateSignalsToBands drops detections whose center frequency falls outside every
// configured band. cfg.Bands is the operator's band(s) of interest; when set, this
// keeps the detector output — and the refinement/classification budget it feeds —
// focused there instead of being flooded by out-of-band emissions. The most
// damaging case is a strong adjacent-band carrier that the peak detectors shatter
// into dozens of fragments (on the hf_40m oracle a +50 dB AM broadcast just outside
// the 40m band is carved into ~50 detections). Those fragments sit at the same SNR
// as real in-band signals, so the discriminator must be FREQUENCY, not strength —
// only a band gate removes them. See issue #80.
//
// When bands is empty the input is returned unchanged (full-span surveillance, the
// historical behaviour). A signal is in-band if its center lies within
// [StartHz, EndHz] of ANY band; zero-width or inverted bands are ignored. Filtering
// is done in place (the input is freshly produced per frame and single-use) to stay
// allocation-free on the DSP hot path.
func GateSignalsToBands(sigs []detector.Signal, bands []config.Band) []detector.Signal {
	if len(bands) == 0 || len(sigs) == 0 {
		return sigs
	}
	out := sigs[:0]
	for _, s := range sigs {
		if bandContains(bands, s.CenterHz) {
			out = append(out, s)
		}
	}
	return out
}

// bandContains reports whether hz lies inside any valid configured band.
func bandContains(bands []config.Band, hz float64) bool {
	for _, b := range bands {
		if b.EndHz <= b.StartHz {
			continue // zero-width or inverted band: ignore rather than match all
		}
		if hz >= b.StartHz && hz <= b.EndHz {
			return true
		}
	}
	return false
}

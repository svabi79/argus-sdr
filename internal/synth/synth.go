// Package synth generates deterministic synthetic IQ scenes with known ground
// truth (modulation kind, center frequency, occupied bandwidth, SNR) for
// benchmarking the detection / estimation / classification pipeline.
//
// It is the measurement spine of the detection rework: every DSP change can be
// scored against scenes whose truth is known, instead of tuning thresholds by
// eye. See docs/detection-rework-plan-2026-06-06.md (Phase R, step R0).
//
// Conventions match the rest of the engine: IQ is []complex64 at complex
// baseband, frequencies are offsets from DC in Hz (a signal at +50e3 sits
// 50 kHz above center), and the spectrum produced by internal/fft is fftshifted
// so DC is at index n/2.
package synth

import (
	"math"
	"math/rand"
)

// Kind is a modulation family. The string values intentionally line up with the
// classifier's class names where they overlap, so the benchmark can map truth to
// predicted classes directly.
type Kind string

const (
	KindCW      Kind = "CW"
	KindAM      Kind = "AM"
	KindSSB     Kind = "SSB" // upper sideband
	KindNFM     Kind = "NFM"
	KindWFM     Kind = "WFM"
	KindFSK     Kind = "FSK"
	KindPSK     Kind = "PSK"
	KindDigital Kind = "DIGITAL" // generic flat occupied band
)

// SignalSpec is one ground-truth signal placed in a scene.
//
//   - CenterHz    offset from baseband DC, in Hz
//   - BandwidthHz intended occupied bandwidth, in Hz
//   - SNRdB       in-band signal-to-noise ratio (signal power over the noise
//     power contained in BandwidthHz)
//   - Duty        fraction of the buffer the signal is active for (0 or 1 means
//     always on); used later for bursty-signal modeling
type SignalSpec struct {
	Kind        Kind
	CenterHz    float64
	BandwidthHz float64
	SNRdB       float64
	Duty        float64
}

// Scene is a reproducible collection of signals plus a noise level. Generate is
// deterministic for a given Seed.
type Scene struct {
	SampleRate int
	Signals    []SignalSpec
	Seed       int64
	NoiseStd   float64 // complex-gaussian per-component std; default 1.0
}

// Generate renders n complex baseband samples for the scene. The result is
// deterministic given Seed.
func (sc Scene) Generate(n int) []complex64 {
	fs := float64(sc.SampleRate)
	if fs <= 0 {
		fs = 1
	}
	rng := rand.New(rand.NewSource(sc.Seed))
	sigma := sc.NoiseStd
	if sigma <= 0 {
		sigma = 1.0
	}
	noisePower := 2 * sigma * sigma // E[|n|^2] for complex gaussian

	acc := make([]complex128, n)
	for _, s := range sc.Signals {
		buf := genSignal(s, fs, n, rng)
		normalizeUnitPower(buf)
		b := s.BandwidthHz
		if b <= 0 || b > fs {
			b = fs
		}
		pnInband := noisePower * b / fs
		ps := math.Pow(10, s.SNRdB/10) * pnInband
		g := math.Sqrt(ps)
		for i := 0; i < n; i++ {
			acc[i] += complex(real(buf[i])*g, imag(buf[i])*g)
		}
	}

	out := make([]complex64, n)
	for i := 0; i < n; i++ {
		re := real(acc[i]) + sigma*rng.NormFloat64()
		im := imag(acc[i]) + sigma*rng.NormFloat64()
		out[i] = complex(float32(re), float32(im))
	}
	return out
}

// genSignal renders one (not yet power-normalized) signal at complex baseband,
// already shifted to CenterHz.
func genSignal(s SignalSpec, fs float64, n int, rng *rand.Rand) []complex128 {
	out := make([]complex128, n)
	const twoPi = 2 * math.Pi
	fc := s.CenterHz
	b := s.BandwidthHz
	if b <= 0 {
		b = 1e3
	}

	switch s.Kind {
	case KindCW:
		// Narrow carrier; bandwidth is effectively a few Hz.
		for i := 0; i < n; i++ {
			ph := twoPi * fc * float64(i) / fs
			out[i] = complex(math.Cos(ph), math.Sin(ph))
		}

	case KindAM:
		// Carrier with a single audio tone -> energy at fc and fc±fa.
		fa := b / 2
		if fa <= 0 {
			fa = 1e3
		}
		const m = 0.7
		for i := 0; i < n; i++ {
			t := float64(i) / fs
			env := 1 + m*math.Cos(twoPi*fa*t)
			ph := twoPi * fc * t
			out[i] = complex(env*math.Cos(ph), env*math.Sin(ph))
		}

	case KindSSB:
		// Upper sideband: a band of audio tones in (0, b] placed above fc.
		const ntones = 8
		freqs := make([]float64, ntones)
		phs := make([]float64, ntones)
		for k := 0; k < ntones; k++ {
			freqs[k] = b * float64(k+1) / float64(ntones)
			phs[k] = rng.Float64() * twoPi
		}
		for i := 0; i < n; i++ {
			t := float64(i) / fs
			var re, im float64
			for k := 0; k < ntones; k++ {
				ph := twoPi*(fc+freqs[k])*t + phs[k]
				re += math.Cos(ph)
				im += math.Sin(ph)
			}
			out[i] = complex(re, im)
		}

	case KindNFM, KindWFM:
		// FM with deviation chosen so Carson bandwidth ~ b: b = 2(beta+1)fm.
		beta := 2.0
		if s.Kind == KindWFM {
			beta = 5.0
		}
		fm := b / (2 * (beta + 1))
		if fm <= 0 {
			fm = 1e3
		}
		for i := 0; i < n; i++ {
			t := float64(i) / fs
			ph := twoPi*fc*t + beta*math.Sin(twoPi*fm*t)
			out[i] = complex(math.Cos(ph), math.Sin(ph))
		}

	default:
		// DIGITAL / FSK / PSK: a flat occupied band of width b centered on fc,
		// built as many equally spaced tones with random complex amplitudes and
		// phases. This gives noise-like content with a sharply defined occupied
		// bandwidth (unlike a moving-average filter, whose sinc sidelobes make
		// the bandwidth ill-defined). Distinct per-kind shapes are deferred.
		const ktones = 24
		freqs := make([]float64, ktones)
		amps := make([]float64, ktones)
		phs := make([]float64, ktones)
		for k := 0; k < ktones; k++ {
			freqs[k] = fc - b/2 + (float64(k)+0.5)*b/float64(ktones)
			amps[k] = 0.5 + rng.Float64()
			phs[k] = rng.Float64() * twoPi
		}
		for i := 0; i < n; i++ {
			t := float64(i) / fs
			var re, im float64
			for k := 0; k < ktones; k++ {
				ph := twoPi*freqs[k]*t + phs[k]
				re += amps[k] * math.Cos(ph)
				im += amps[k] * math.Sin(ph)
			}
			out[i] = complex(re, im)
		}
	}

	if s.Duty > 0 && s.Duty < 1 {
		active := int(float64(n) * s.Duty)
		for i := active; i < n; i++ {
			out[i] = 0
		}
	}
	return out
}

// normalizeUnitPower scales b in place to unit average power over the buffer.
func normalizeUnitPower(b []complex128) {
	if len(b) == 0 {
		return
	}
	var p float64
	for _, v := range b {
		p += real(v)*real(v) + imag(v)*imag(v)
	}
	mean := p / float64(len(b))
	if mean <= 0 {
		return
	}
	g := 1 / math.Sqrt(mean)
	for i := range b {
		b[i] = complex(real(b[i])*g, imag(b[i])*g)
	}
}

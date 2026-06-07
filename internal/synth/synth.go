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
	// Dynamic, for WFM, selects a non-stationary broadcast-MPX program model
	// (breathing occupied bandwidth) instead of the default stationary multitone
	// FM. See genWFMDynamic. Off by default so existing stationary scenes/tests
	// are unaffected.
	Dynamic bool
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
		// Carrier with a multi-tone audio message (continuous double sideband,
		// occupied ~ 2*fa = b), rather than a single tone's discrete sidebands.
		fa := b / 2
		if fa <= 0 {
			fa = 1e3
		}
		const (
			ktones = 10
			m      = 0.7
		)
		af := make([]float64, ktones)
		ap := make([]float64, ktones)
		for k := 0; k < ktones; k++ {
			af[k] = fa * float64(k+1) / float64(ktones)
			ap[k] = rng.Float64() * twoPi
		}
		for i := 0; i < n; i++ {
			t := float64(i) / fs
			var msg float64
			for k := 0; k < ktones; k++ {
				msg += math.Cos(twoPi*af[k]*t + ap[k])
			}
			env := 1 + m*msg/float64(ktones)
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
		if s.Kind == KindWFM && s.Dynamic {
			out = genWFMDynamic(s, fs, n, rng)
			break
		}
		// FM with a multi-tone audio message so the spectrum is continuous and
		// fills the Carson bandwidth (real broadcast/voice FM), rather than the
		// discrete Bessel lines a single tone would give. Peak deviation and
		// message bandwidth are chosen so Carson 2*(dev+W) ~ b.
		w := b / 6
		if w <= 0 {
			w = 1e3
		}
		dev := b/2 - w
		if dev < 0 {
			dev = b / 2
		}
		const ktones = 12
		mf := make([]float64, ktones)
		mp := make([]float64, ktones)
		for k := 0; k < ktones; k++ {
			mf[k] = w * float64(k+1) / float64(ktones)
			mp[k] = rng.Float64() * twoPi
		}
		msg := make([]float64, n)
		peak := 0.0
		for i := 0; i < n; i++ {
			t := float64(i) / fs
			var m float64
			for k := 0; k < ktones; k++ {
				m += math.Cos(twoPi*mf[k]*t + mp[k])
			}
			msg[i] = m
			if a := math.Abs(m); a > peak {
				peak = a
			}
		}
		if peak <= 0 {
			peak = 1
		}
		phase := 0.0
		for i := 0; i < n; i++ {
			inst := fc + dev*(msg[i]/peak) // peak deviation = dev
			phase += twoPi * inst / fs
			out[i] = complex(math.Cos(phase), math.Sin(phase))
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

// genWFMDynamic renders a broadcast WFM station with a NON-STATIONARY program,
// so its occupied bandwidth breathes from generation to generation (frame to
// frame in the benchmark). It is modelled after a real stereo-MPX FM encoder
// (cf. D:/Code/fm-rds-tx): a randomized multitone L/R program with a
// per-realization loudness and message bandwidth drives the mono and stereo
// (38 kHz DSB-SC) paths, on top of a constant 19 kHz pilot and a 57 kHz RDS BPSK
// subcarrier that set a realistic occupancy floor; the composite frequency-
// modulates the carrier at broadcast peak deviation (±75 kHz). Quiet frames
// occupy ~the pilot/RDS floor; loud frames open up to the full ~±75 kHz
// multiplex — the live behaviour OI-23 must reproduce offline (Constitution IV).
//
// Determinism: all randomness is drawn from the passed rng, so a given Scene.Seed
// reproduces exactly. The benchmark re-seeds per frame, so each frame is a
// different (loud/quiet, narrow/wide) realization — that is the breathing.
func genWFMDynamic(s SignalSpec, fs float64, n int, rng *rand.Rand) []complex128 {
	const (
		twoPi    = 2 * math.Pi
		peakDev  = 75000.0 // broadcast WFM peak deviation
		pilotHz  = 19000.0
		subHz    = 38000.0 // stereo subcarrier (2x pilot)
		rdsHz    = 57000.0 // RDS subcarrier (3x pilot)
		rdsBaud  = 1187.5  // RDS bit rate
		pilotAmp = 0.09    // ~9% deviation, broadcast-typical
		rdsAmp   = 0.04    // ~4% deviation
		ktones   = 12
	)
	out := make([]complex128, n)
	fc := s.CenterHz

	// Per-realization program dynamics: loudness 0.25..1.0 and message bandwidth
	// 4..15 kHz, so the occupied band varies between quiet/narrow and loud/wide.
	loud := 0.25 + 0.75*rng.Float64()
	progBW := 4000.0 + 11000.0*rng.Float64()
	lf := make([]float64, ktones)
	la := make([]float64, ktones)
	lp := make([]float64, ktones)
	ra := make([]float64, ktones)
	rp := make([]float64, ktones)
	for k := 0; k < ktones; k++ {
		lf[k] = progBW * float64(k+1) / float64(ktones)
		la[k] = rng.Float64()
		lp[k] = rng.Float64() * twoPi
		ra[k] = rng.Float64()
		rp[k] = rng.Float64() * twoPi
	}

	// RDS NRZ bit stream (random, deterministic from rng), small contribution.
	nbits := int(float64(n)/fs*rdsBaud) + 2
	bits := make([]float64, nbits)
	for i := range bits {
		if rng.Float64() < 0.5 {
			bits[i] = -1
		} else {
			bits[i] = 1
		}
	}

	// First pass: build the L/R program and find its peak, to normalize loudness
	// so loud==1.0 maps to ~full program deviation.
	lbuf := make([]float64, n)
	rbuf := make([]float64, n)
	peak := 1e-9
	for i := 0; i < n; i++ {
		t := float64(i) / fs
		var l, r float64
		for k := 0; k < ktones; k++ {
			l += la[k] * math.Cos(twoPi*lf[k]*t+lp[k])
			r += ra[k] * math.Cos(twoPi*lf[k]*t+rp[k])
		}
		lbuf[i] = l
		rbuf[i] = r
		if a := math.Abs(l); a > peak {
			peak = a
		}
		if a := math.Abs(r); a > peak {
			peak = a
		}
	}
	g := loud / peak

	// Second pass: assemble the MPX composite and FM-modulate it.
	phase := 0.0
	for i := 0; i < n; i++ {
		t := float64(i) / fs
		l := lbuf[i] * g
		r := rbuf[i] * g
		mono := 0.5 * (l + r)
		diff := 0.5 * (l - r)
		pilot := pilotAmp * math.Sin(twoPi*pilotHz*t)
		stereo := diff * math.Sin(twoPi*subHz*t)
		bi := int(t * rdsBaud)
		if bi >= len(bits) {
			bi = len(bits) - 1
		}
		rds := rdsAmp * bits[bi] * math.Sin(twoPi*rdsHz*t)
		composite := mono + stereo + pilot + rds
		inst := fc + peakDev*composite
		phase += twoPi * inst / fs
		out[i] = complex(math.Cos(phase), math.Sin(phase))
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

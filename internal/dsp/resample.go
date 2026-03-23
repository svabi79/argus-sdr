package dsp

import "math"

// ---------------------------------------------------------------------------
// Rational Polyphase Resampler
// ---------------------------------------------------------------------------
//
// Converts sample rate by a rational factor L/M (upsample by L, then
// downsample by M) using a polyphase FIR implementation. The polyphase
// decomposition avoids computing intermediate upsampled samples that
// would be discarded, making it efficient even for large L/M.
//
// The resampler is stateful: it preserves its internal delay line and
// phase index between calls to Process(), enabling click-free streaming
// across frame boundaries.
//
// Usage:
//
//	r := dsp.NewResampler(51200, 48000, 64) // 64 taps per phase
//	for each frame {
//	    out := r.Process(audio)  // or r.ProcessStereo(interleaved)
//	}
//
// ---------------------------------------------------------------------------

// Resampler performs rational polyphase sample rate conversion.
type Resampler struct {
	l         int         // upsample factor
	m         int         // downsample factor
	tapsPerPh int         // taps per polyphase arm
	polyBank  [][]float64 // polyBank[phase][tap]
	delay     []float64   // delay line, length = tapsPerPh
	// outTime is the position (in upsampled-rate units) of the next output
	// sample, relative to the next input sample to be consumed. It is
	// always in [0, L). Between calls it persists so that the fractional
	// position is perfectly continuous.
	outTime int
}

// NewResampler creates a polyphase resampler converting from inRate to
// outRate. tapsPerPhase controls the filter quality (16 = basic, 32 =
// good, 64 = high quality). The total prototype filter length is
// L * tapsPerPhase.
func NewResampler(inRate, outRate, tapsPerPhase int) *Resampler {
	if inRate <= 0 || outRate <= 0 {
		inRate, outRate = 1, 1
	}
	if tapsPerPhase < 4 {
		tapsPerPhase = 4
	}

	g := gcd(inRate, outRate)
	l := outRate / g // upsample factor
	m := inRate / g  // downsample factor

	// Prototype lowpass: cutoff at min(1/L, 1/M) * Nyquist of the
	// upsampled rate, with some margin for the transition band.
	protoLen := l * tapsPerPhase
	if protoLen%2 == 0 {
		protoLen++ // ensure odd length for symmetric filter
	}

	// Normalized cutoff: passband edge relative to upsampled rate.
	// 0.90 passes up to ~95% of output Nyquist (≈22.8kHz at 48kHz out),
	// providing full 15kHz FM stereo bandwidth. The Kaiser window (β=6)
	// gives ≈-60dB sidelobe suppression for clean anti-alias rejection.
	fc := 0.90 / float64(max(l, m))
	proto := windowedSinc(protoLen, fc, float64(l))

	// Decompose prototype into L polyphase arms
	actualTapsPerPh := (protoLen + l - 1) / l
	bank := make([][]float64, l)
	for p := 0; p < l; p++ {
		arm := make([]float64, actualTapsPerPh)
		for t := 0; t < actualTapsPerPh; t++ {
			idx := p + t*l
			if idx < protoLen {
				arm[t] = proto[idx]
			}
		}
		bank[p] = arm
	}

	return &Resampler{
		l:         l,
		m:         m,
		tapsPerPh: actualTapsPerPh,
		polyBank:  bank,
		delay:     make([]float64, actualTapsPerPh),
		outTime:   0,
	}
}

// Process resamples a mono float32 buffer and returns the resampled output.
// State is preserved between calls for seamless streaming.
//
// The key insight: we conceptually interleave L-1 zeros between each input
// sample (upsampled rate = L * Fs_in), then pick every M-th sample from
// the filtered result (output rate = L/M * Fs_in).
//
// outTime tracks the sub-sample position of the next output within the
// current input sample's L phases. When outTime wraps past L, we consume
// the next input sample. This single counter gives exact, chunk-independent
// output.
func (r *Resampler) Process(in []float32) []float32 {
	if len(in) == 0 {
		return nil
	}
	if r.l == r.m {
		out := make([]float32, len(in))
		copy(out, in)
		return out
	}

	L := r.l
	M := r.m
	taps := r.tapsPerPh
	estOut := int(float64(len(in))*float64(L)/float64(M)) + 4
	out := make([]float32, 0, estOut)

	inPos := 0
	t := r.outTime

	for inPos < len(in) {
		// Consume input samples until outTime < L
		for t >= L {
			t -= L
			if inPos >= len(in) {
				r.outTime = t
				return out
			}
			copy(r.delay[1:], r.delay[:taps-1])
			r.delay[0] = float64(in[inPos])
			inPos++
		}

		// Produce output at phase = t
		arm := r.polyBank[t]
		var acc float64
		for k := 0; k < taps; k++ {
			acc += r.delay[k] * arm[k]
		}
		out = append(out, float32(acc))

		// Advance to next output position
		t += M
	}

	r.outTime = t
	return out
}

// Reset clears the delay line and phase state.
func (r *Resampler) Reset() {
	for i := range r.delay {
		r.delay[i] = 0
	}
	r.outTime = 0
}

// OutputRate returns the effective output sample rate given an input rate.
func (r *Resampler) OutputRate(inRate int) int {
	return inRate * r.l / r.m
}

// Ratio returns L and M.
func (r *Resampler) Ratio() (int, int) {
	return r.l, r.m
}

// ---------------------------------------------------------------------------
// StereoResampler — two synchronised mono resamplers
// ---------------------------------------------------------------------------

// StereoResampler wraps two Resampler instances sharing the same L/M ratio
// for click-free stereo resampling with independent delay lines.
type StereoResampler struct {
	left  *Resampler
	right *Resampler
}

// NewStereoResampler creates a pair of synchronised resamplers.
func NewStereoResampler(inRate, outRate, tapsPerPhase int) *StereoResampler {
	return &StereoResampler{
		left:  NewResampler(inRate, outRate, tapsPerPhase),
		right: NewResampler(inRate, outRate, tapsPerPhase),
	}
}

// Process takes interleaved stereo [L0,R0,L1,R1,...] and returns
// resampled interleaved stereo.
func (sr *StereoResampler) Process(in []float32) []float32 {
	nFrames := len(in) / 2
	if nFrames == 0 {
		return nil
	}
	left := make([]float32, nFrames)
	right := make([]float32, nFrames)
	for i := 0; i < nFrames; i++ {
		left[i] = in[i*2]
		if i*2+1 < len(in) {
			right[i] = in[i*2+1]
		}
	}

	outL := sr.left.Process(left)
	outR := sr.right.Process(right)

	// Interleave — use shorter length if they differ by 1 sample
	n := len(outL)
	if len(outR) < n {
		n = len(outR)
	}
	out := make([]float32, n*2)
	for i := 0; i < n; i++ {
		out[i*2] = outL[i]
		out[i*2+1] = outR[i]
	}
	return out
}

// Reset clears both delay lines.
func (sr *StereoResampler) Reset() {
	sr.left.Reset()
	sr.right.Reset()
}

// OutputRate returns the resampled output rate.
func (sr *StereoResampler) OutputRate(inRate int) int {
	return sr.left.OutputRate(inRate)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// windowedSinc generates a windowed-sinc prototype lowpass filter.
// fc is the normalised cutoff (0..0.5 of the upsampled rate).
// gain is the scaling factor (= L for polyphase interpolation).
func windowedSinc(length int, fc float64, gain float64) []float64 {
	out := make([]float64, length)
	mid := float64(length-1) / 2.0
	for n := 0; n < length; n++ {
		x := float64(n) - mid
		// Sinc
		var s float64
		if math.Abs(x) < 1e-12 {
			s = 2 * math.Pi * fc
		} else {
			s = math.Sin(2*math.Pi*fc*x) / x
		}
		// Kaiser window (beta=6 gives ~-60dB sidelobe, good for audio)
		w := kaiserWindow(n, length, 6.0)
		out[n] = s * w * gain
	}
	return out
}

// kaiserWindow computes the Kaiser window value for sample n of N total.
func kaiserWindow(n, N int, beta float64) float64 {
	mid := float64(N-1) / 2.0
	x := (float64(n) - mid) / mid
	return bessel0(beta*math.Sqrt(1-x*x)) / bessel0(beta)
}

// bessel0 is the zeroth-order modified Bessel function of the first kind.
func bessel0(x float64) float64 {
	// Series expansion — converges rapidly for typical beta values
	sum := 1.0
	term := 1.0
	for k := 1; k < 30; k++ {
		term *= (x / (2 * float64(k))) * (x / (2 * float64(k)))
		sum += term
		if term < 1e-12*sum {
			break
		}
	}
	return sum
}

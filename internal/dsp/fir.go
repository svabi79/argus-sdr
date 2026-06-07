package dsp

import "math"

// LowpassFIR returns windowed-sinc lowpass taps (Hann).
func LowpassFIR(cutoffHz float64, sampleRate int, taps int) []float64 {
	if taps%2 == 0 {
		taps++
	}
	out := make([]float64, taps)
	fc := cutoffHz / float64(sampleRate)
	if fc <= 0 {
		return out
	}
	m := float64(taps-1) / 2.0
	for n := 0; n < taps; n++ {
		x := float64(n) - m
		var sinc float64
		if x == 0 {
			sinc = 2 * fc
		} else {
			sinc = math.Sin(2*math.Pi*fc*x) / (math.Pi * x)
		}
		w := 0.5 * (1 - math.Cos(2*math.Pi*float64(n)/float64(taps-1)))
		out[n] = float64(sinc) * w
	}
	return out
}

// ApplyFIR applies real FIR taps to complex IQ.
func ApplyFIR(iq []complex64, taps []float64) []complex64 {
	if len(iq) == 0 || len(taps) == 0 {
		return nil
	}
	out := make([]complex64, len(iq))
	n := len(taps)
	for i := 0; i < len(iq); i++ {
		var accR, accI float64
		for k := 0; k < n; k++ {
			idx := i - k
			if idx < 0 {
				break
			}
			v := iq[idx]
			w := taps[k]
			accR += float64(real(v)) * w
			accI += float64(imag(v)) * w
		}
		out[i] = complex(float32(accR), float32(accI))
	}
	return out
}

// ApplyFIRInto is ApplyFIR but writes into out (grown if needed) and returns the
// used slice, so callers with a stable scratch buffer avoid a per-call allocation.
// out must NOT alias iq (the filter reads past input samples).
func ApplyFIRInto(iq []complex64, taps []float64, out []complex64) []complex64 {
	if len(iq) == 0 || len(taps) == 0 {
		return out[:0]
	}
	if cap(out) < len(iq) {
		out = make([]complex64, len(iq))
	}
	out = out[:len(iq)]
	n := len(taps)
	for i := 0; i < len(iq); i++ {
		var accR, accI float64
		for k := 0; k < n; k++ {
			idx := i - k
			if idx < 0 {
				break
			}
			v := iq[idx]
			w := taps[k]
			accR += float64(real(v)) * w
			accI += float64(imag(v)) * w
		}
		out[i] = complex(float32(accR), float32(accI))
	}
	return out
}

// Decimate keeps every nth sample.
func Decimate(iq []complex64, factor int) []complex64 {
	if factor <= 1 {
		out := make([]complex64, len(iq))
		copy(out, iq)
		return out
	}
	out := make([]complex64, 0, len(iq)/factor+1)
	for i := 0; i < len(iq); i += factor {
		out = append(out, iq[i])
	}
	return out
}

// DecimateStateful keeps every nth sample, preserving the decimation phase
// across calls. *phase is the index offset into the next frame where the
// first output sample should be taken. It is updated on return so that
// consecutive calls produce a continuous, gap-free decimated stream.
//
// Initial value of *phase should be 0.
func DecimateStateful(iq []complex64, factor int, phase *int) []complex64 {
	if factor <= 1 || phase == nil {
		out := make([]complex64, len(iq))
		copy(out, iq)
		return out
	}
	n := len(iq)
	p := *phase
	if p < 0 {
		p = 0
	}
	out := make([]complex64, 0, (n-p)/factor+1)
	for i := p; i < n; i += factor {
		out = append(out, iq[i])
	}
	// Compute phase for next frame: how many samples past the end of this
	// frame until the next decimation point.
	if n > 0 && p < n {
		lastTaken := p + ((n-1-p)/factor)*factor
		*phase = lastTaken + factor - n
	} else if p >= n {
		// Entire frame was skipped (very short snippet)
		*phase = p - n
	}
	return out
}

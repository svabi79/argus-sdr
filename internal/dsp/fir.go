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

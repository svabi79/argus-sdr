package dsp

// ApplyFIRReal applies real FIR taps to real signal.
func ApplyFIRReal(x []float32, taps []float64) []float32 {
	if len(x) == 0 || len(taps) == 0 {
		return nil
	}
	out := make([]float32, len(x))
	for i := 0; i < len(x); i++ {
		var acc float64
		for k := 0; k < len(taps); k++ {
			idx := i - k
			if idx < 0 {
				break
			}
			acc += float64(x[idx]) * taps[k]
		}
		out[i] = float32(acc)
	}
	return out
}

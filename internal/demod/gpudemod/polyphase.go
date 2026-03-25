package gpudemod

// BuildPolyphaseTapsPhaseMajor builds a phase-major polyphase tap layout:
// tapsByPhase[p][k] = h[p + k*D]
// Flattened as: [phase0 taps..., phase1 taps..., ...]
func BuildPolyphaseTapsPhaseMajor(base []float32, decim int) []float32 {
	if decim <= 0 || len(base) == 0 {
		return nil
	}
	maxPhaseLen := (len(base) + decim - 1) / decim
	out := make([]float32, decim*maxPhaseLen)
	for p := 0; p < decim; p++ {
		for k := 0; k < maxPhaseLen; k++ {
			src := p + k*decim
			if src < len(base) {
				out[p*maxPhaseLen+k] = base[src]
			}
		}
	}
	return out
}

func PolyphasePhaseLen(baseLen int, decim int) int {
	if decim <= 0 || baseLen <= 0 {
		return 0
	}
	return (baseLen + decim - 1) / decim
}

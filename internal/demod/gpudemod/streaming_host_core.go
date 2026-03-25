package gpudemod

import "math"

func runStreamingPolyphaseHostCore(
	iqNew []complex64,
	sampleRate int,
	offsetHz float64,
	stateNCOPhase float64,
	statePhaseCount int,
	stateNumTaps int,
	stateDecim int,
	stateHistory []complex64,
	polyphaseTaps []float32,
) ([]complex64, float64, int, []complex64) {
	out := make([]complex64, 0, len(iqNew)/maxInt(1, stateDecim)+2)
	phase := stateNCOPhase
	phaseCount := statePhaseCount
	hist := append([]complex64(nil), stateHistory...)
	phaseLen := PolyphasePhaseLen(len(polyphaseTaps)/maxInt(1, stateDecim)*maxInt(1, stateDecim), stateDecim)
	if phaseLen == 0 {
		phaseLen = PolyphasePhaseLen(len(polyphaseTaps), stateDecim)
	}
	phaseInc := -2.0 * math.Pi * offsetHz / float64(sampleRate)
	for _, x := range iqNew {
		rot := complex64(complex(math.Cos(phase), math.Sin(phase)))
		s := x * rot
		hist = append(hist, s)
		phaseCount++
		if phaseCount == stateDecim {
			var y complex64
			for p := 0; p < stateDecim; p++ {
				for k := 0; k < phaseLen; k++ {
					idxTap := p*phaseLen + k
					if idxTap >= len(polyphaseTaps) {
						continue
					}
					tap := polyphaseTaps[idxTap]
					if tap == 0 {
						continue
					}
					srcBack := p + k*stateDecim
					idx := len(hist) - 1 - srcBack
					if idx < 0 {
						continue
					}
					y += complex(tap, 0) * hist[idx]
				}
			}
			out = append(out, y)
			phaseCount = 0
		}
		if len(hist) > stateNumTaps-1 {
			hist = hist[len(hist)-(stateNumTaps-1):]
		}
		phase += phaseInc
		if phase >= math.Pi {
			phase -= 2 * math.Pi
		} else if phase < -math.Pi {
			phase += 2 * math.Pi
		}
	}
	return out, phase, phaseCount, append([]complex64(nil), hist...)
}

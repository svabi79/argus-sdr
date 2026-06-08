package recorder

import (
	"math"
	"math/rand"
	"testing"
)

// genMPX builds a phase-continuous FM stereo composite (the demodulated MPX that
// stereoDecodeStateful consumes): a 1 kHz L+R program tone at `audioAmp`, a
// CONSTANT-amplitude 19 kHz pilot, and a 2 kHz L-R tone DSB-modulated at 38 kHz.
// The program loudness varies per block; the pilot does not.
func genMPX(n, sampleRate int, audioAmp, pilotAmp float64, phase *float64) []float32 {
	out := make([]float32, n)
	dt := 1.0 / float64(sampleRate)
	for i := 0; i < n; i++ {
		t := *phase + float64(i)*dt
		audio := audioAmp * math.Sin(2*math.Pi*1000*t)
		pilot := pilotAmp * math.Sin(2*math.Pi*19000*t)
		lr := 0.3 * audioAmp * math.Sin(2*math.Pi*2000*t) * math.Sin(2*math.Pi*38000*t)
		out[i] = float32(audio + pilot + lr)
	}
	*phase += float64(n) * dt
	return out
}

// TestStereoLockHoldsThroughLoudAudio is the regression for the live "rauscht
// zwischendurch": the old lock metric pilotPower/totalPower dropped on loud program
// even with a perfect pilot, flapping the lock and switching the audio stereo<->mono.
// With the audio-independent coherence metric the lock must hold through loud blocks.
func TestStereoLockHoldsThroughLoudAudio(t *testing.T) {
	const sr = 256000
	const blk = 4096
	sess := &streamSession{}
	var phase float64
	const pilot = 0.08 // ~realistic: pilot is a small fraction of peak deviation
	// warmup quiet, then alternate quiet / LOUD program.
	amps := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.9, 0.15, 0.95, 0.1, 0.9, 0.95, 0.12, 0.9, 0.95, 0.9, 0.1, 0.95}
	lockedPost, loud, loudLocked := 0, 0, 0
	for b, amp := range amps {
		mpx := genMPX(blk, sr, amp, pilot, &phase)
		_, locked := sess.stereoDecodeStateful(mpx, sr)
		if b >= 6 { // post-warmup (PLL settled)
			if locked {
				lockedPost++
			}
			if amp >= 0.8 {
				loud++
				if locked {
					loudLocked++
				}
			}
		}
	}
	t.Logf("post-warmup blocks locked=%d; loud blocks=%d locked=%d", lockedPost, loud, loudLocked)
	if loud == 0 {
		t.Fatal("no loud blocks in pattern")
	}
	if loudLocked < loud {
		t.Errorf("stereo lock DROPPED on loud audio: %d/%d loud blocks stayed locked (must be all)", loudLocked, loud)
	}
}

// TestStereoNoFalseLockOnNoise: with no pilot (pure noise in the pilot band) the
// coherence must stay low so we don't lock onto noise.
func TestStereoNoFalseLockOnNoise(t *testing.T) {
	const sr = 256000
	const blk = 4096
	sess := &streamSession{}
	rng := rand.New(rand.NewSource(1))
	falseLocks := 0
	for b := 0; b < 15; b++ {
		mpx := make([]float32, blk)
		for i := range mpx {
			mpx[i] = float32(rng.NormFloat64() * 0.3)
		}
		if _, locked := sess.stereoDecodeStateful(mpx, sr); locked {
			falseLocks++
		}
	}
	if falseLocks > 0 {
		t.Errorf("false stereo lock on noise: %d/15 blocks locked (must be 0)", falseLocks)
	}
}

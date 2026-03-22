package recorder

import (
	"math"
	"testing"
)

func TestStereoDecodeStatefulPilotLock(t *testing.T) {
	const (
		sampleRate = 192000
		blockSize  = 4096
		blocks     = 10
		pilotAmp   = 0.1
		toneL      = 440.0
		toneR      = 880.0
	)

	sess := &streamSession{}
	var out []float32
	locked := false

	for b := 0; b < blocks; b++ {
		mono := make([]float32, blockSize)
		base := b * blockSize
		for i := 0; i < blockSize; i++ {
			t := float64(base+i) / float64(sampleRate)
			l := math.Sin(2 * math.Pi * toneL * t)
			r := math.Sin(2 * math.Pi * toneR * t)
			lpr := 0.5 * (l + r)
			lmr := 0.5 * (l - r)
			composite := lpr + lmr*math.Cos(2*math.Pi*38000*t) + pilotAmp*math.Sin(2*math.Pi*19000*t)
			mono[i] = float32(composite)
		}
		out, locked = sess.stereoDecodeStateful(mono, sampleRate)
	}

	if !locked {
		t.Fatalf("expected pilot lock after warmup blocks")
	}
	if len(out) != blockSize*2 {
		t.Fatalf("unexpected output size: got %d, want %d", len(out), blockSize*2)
	}

	left := make([]float32, blockSize)
	right := make([]float32, blockSize)
	for i := 0; i < blockSize; i++ {
		left[i] = out[i*2]
		right[i] = out[i*2+1]
	}

	magL440 := toneMagnitude(left, toneL, sampleRate)
	magL880 := toneMagnitude(left, toneR, sampleRate)
	magR440 := toneMagnitude(right, toneL, sampleRate)
	magR880 := toneMagnitude(right, toneR, sampleRate)

	if magL440 < 0.05 || magR880 < 0.05 {
		t.Fatalf("decoded tones too weak: L440=%.3f R880=%.3f", magL440, magR880)
	}
	leftIsL := magL440 >= magL880*1.3 && magR880 >= magR440*1.3
	rightIsL := magL880 >= magL440*1.3 && magR440 >= magR880*1.3
	if !leftIsL && !rightIsL {
		t.Fatalf(
			"channels not cleanly separated: L440=%.3f L880=%.3f R440=%.3f R880=%.3f",
			magL440, magL880, magR440, magR880,
		)
	}
}

func toneMagnitude(x []float32, freq float64, sampleRate int) float64 {
	var iSum, qSum float64
	for n, v := range x {
		angle := 2 * math.Pi * freq * float64(n) / float64(sampleRate)
		iSum += float64(v) * math.Cos(angle)
		qSum += float64(v) * math.Sin(angle)
	}
	return math.Hypot(iSum, qSum) / float64(len(x))
}

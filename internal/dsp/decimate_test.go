package dsp

import (
	"testing"
)

func TestDecimateStateful_ContinuousPhase(t *testing.T) {
	// Simulate what happens in the streaming pipeline:
	// Two consecutive frames with non-divisible lengths decimated by 3.
	// Stateful version must produce the same output as decimating the
	// concatenated input in one go.

	factor := 3
	// Frame lengths that don't divide evenly (like real WFM: 41666 % 3 = 2)
	frame1 := make([]complex64, 41666)
	frame2 := make([]complex64, 41666)
	for i := range frame1 {
		frame1[i] = complex(float32(i), 0)
	}
	for i := range frame2 {
		frame2[i] = complex(float32(len(frame1)+i), 0)
	}

	// Reference: concatenate and decimate in one shot
	combined := make([]complex64, len(frame1)+len(frame2))
	copy(combined, frame1)
	copy(combined[len(frame1):], frame2)
	ref := Decimate(combined, factor)

	// Stateful: decimate frame by frame
	phase := 0
	out1 := DecimateStateful(frame1, factor, &phase)
	out2 := DecimateStateful(frame2, factor, &phase)

	got := make([]complex64, len(out1)+len(out2))
	copy(got, out1)
	copy(got[len(out1):], out2)

	if len(got) != len(ref) {
		t.Fatalf("length mismatch: stateful=%d reference=%d", len(got), len(ref))
	}
	for i := range ref {
		if got[i] != ref[i] {
			t.Fatalf("sample %d: got %v want %v", i, got[i], ref[i])
		}
	}
}

func TestDecimateStateful_Factor4_NFM(t *testing.T) {
	// NFM scenario: 200kHz/48kHz → decim=4, frame=16666 samples
	// 16666 % 4 = 2 → phase slip every frame with stateless decimation
	factor := 4
	frameLen := 16666
	nFrames := 10

	// Build continuous signal
	total := make([]complex64, frameLen*nFrames)
	for i := range total {
		total[i] = complex(float32(i), float32(-i))
	}
	ref := Decimate(total, factor)

	// Stateful frame-by-frame
	phase := 0
	var got []complex64
	for f := 0; f < nFrames; f++ {
		chunk := total[f*frameLen : (f+1)*frameLen]
		out := DecimateStateful(chunk, factor, &phase)
		got = append(got, out...)
	}

	if len(got) != len(ref) {
		t.Fatalf("length mismatch: stateful=%d reference=%d (frames=%d)", len(got), len(ref), nFrames)
	}
	for i := range ref {
		if got[i] != ref[i] {
			t.Fatalf("frame-boundary glitch at sample %d: got %v want %v", i, got[i], ref[i])
		}
	}
}

func TestDecimateStateful_Factor1_Passthrough(t *testing.T) {
	in := []complex64{1 + 2i, 3 + 4i, 5 + 6i}
	phase := 0
	out := DecimateStateful(in, 1, &phase)
	if len(out) != len(in) {
		t.Fatalf("passthrough: got len %d want %d", len(out), len(in))
	}
}

func TestDecimateStateful_ExactDivisible(t *testing.T) {
	// When frame length is exactly divisible, phase should stay 0
	factor := 4
	frame := make([]complex64, 100) // 100 % 4 = 0
	for i := range frame {
		frame[i] = complex(float32(i), 0)
	}
	phase := 0
	out := DecimateStateful(frame, factor, &phase)
	if phase != 0 {
		t.Fatalf("exact divisible: phase should be 0, got %d", phase)
	}
	if len(out) != 25 {
		t.Fatalf("exact divisible: got %d samples, want 25", len(out))
	}
}

func TestDecimateStateful_VaryingFrameSizes(t *testing.T) {
	// Real-world: buffer jitter causes varying frame sizes
	factor := 3
	frameSizes := []int{41600, 41700, 41666, 41650, 41680}

	// Build total
	totalLen := 0
	for _, s := range frameSizes {
		totalLen += s
	}
	total := make([]complex64, totalLen)
	for i := range total {
		total[i] = complex(float32(i), float32(i*2))
	}
	ref := Decimate(total, factor)

	phase := 0
	var got []complex64
	offset := 0
	for _, sz := range frameSizes {
		chunk := total[offset : offset+sz]
		out := DecimateStateful(chunk, factor, &phase)
		got = append(got, out...)
		offset += sz
	}

	if len(got) != len(ref) {
		t.Fatalf("varying frames: stateful=%d reference=%d", len(got), len(ref))
	}
	for i := range ref {
		if got[i] != ref[i] {
			t.Fatalf("varying frames: mismatch at %d: got %v want %v", i, got[i], ref[i])
		}
	}
}

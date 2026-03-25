package dsp

import (
	"math/cmplx"
	"testing"
)

func TestStatefulDecimatingFIRComplexStreamContinuity(t *testing.T) {
	taps := LowpassFIR(90000, 512000, 101)
	factor := 2

	input := make([]complex64, 8192)
	for i := range input {
		input[i] = complex(float32((i%17)-8)/8.0, float32((i%11)-5)/8.0)
	}

	one := NewStatefulDecimatingFIRComplex(taps, factor)
	whole := one.Process(input)

	chunkedProc := NewStatefulDecimatingFIRComplex(taps, factor)
	var chunked []complex64
	for i := 0; i < len(input); i += 733 {
		end := i + 733
		if end > len(input) {
			end = len(input)
		}
		chunked = append(chunked, chunkedProc.Process(input[i:end])...)
	}

	if len(whole) != len(chunked) {
		t.Fatalf("length mismatch whole=%d chunked=%d", len(whole), len(chunked))
	}
	for i := range whole {
		if cmplx.Abs(complex128(whole[i]-chunked[i])) > 1e-5 {
			t.Fatalf("sample %d mismatch whole=%v chunked=%v", i, whole[i], chunked[i])
		}
	}
}

func TestStatefulDecimatingFIRComplexMatchesBlockPipelineLength(t *testing.T) {
	taps := LowpassFIR(90000, 512000, 101)
	factor := 2
	input := make([]complex64, 48640)
	for i := range input {
		input[i] = complex(float32((i%13)-6)/8.0, float32((i%7)-3)/8.0)
	}

	stateful := NewStatefulDecimatingFIRComplex(taps, factor)
	out := stateful.Process(input)

	filtered := ApplyFIR(input, taps)
	dec := Decimate(filtered, factor)

	if len(out) != len(dec) {
		t.Fatalf("unexpected output len got=%d want=%d", len(out), len(dec))
	}
}

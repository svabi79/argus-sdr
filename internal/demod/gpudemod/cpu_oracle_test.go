package gpudemod

import (
	"math"
	"math/cmplx"
	"testing"
)

func makeDeterministicIQ(n int) []complex64 {
	out := make([]complex64, n)
	for i := 0; i < n; i++ {
		a := 0.017 * float64(i)
		b := 0.031 * float64(i)
		out[i] = complex64(complex(math.Cos(a)+0.2*math.Cos(b), math.Sin(a)+0.15*math.Sin(b)))
	}
	return out
}

func makeLowpassTaps(n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = 1.0 / float32(n)
	}
	return out
}

func requireComplexSlicesClose(t *testing.T, a []complex64, b []complex64, tol float64) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("length mismatch: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if cmplx.Abs(complex128(a[i]-b[i])) > tol {
			t.Fatalf("slice mismatch at %d: %v vs %v (tol=%f)", i, a[i], b[i], tol)
		}
	}
}

func TestCPUOracleMonolithicVsChunked(t *testing.T) {
	iq := makeDeterministicIQ(200000)
	mk := func() *CPUOracleState {
		return &CPUOracleState{
			SignalID:       1,
			ConfigHash:     123,
			NCOPhase:       0,
			Decim:          20,
			PhaseCount:     0,
			NumTaps:        65,
			ShiftedHistory: make([]complex64, 0, 64),
			BaseTaps:       makeLowpassTaps(65),
		}
	}
	phaseInc := 0.017
	monoState := mk()
	mono := CPUOracleExtract(iq, monoState, phaseInc)
	chunked := RunChunkedCPUOracle(iq, []int{4096, 5000, 8192, 27307}, mk, phaseInc)
	requireComplexSlicesClose(t, mono, chunked, 1e-5)
}

func TestExactIntegerDecimation(t *testing.T) {
	if d, err := ExactIntegerDecimation(4000000, 200000); err != nil || d != 20 {
		t.Fatalf("unexpected exact decim result: d=%d err=%v", d, err)
	}
	if _, err := ExactIntegerDecimation(4000000, 192000); err == nil {
		t.Fatalf("expected non-integer decimation error")
	}
}

func TestCPUOracleDirectVsPolyphase(t *testing.T) {
	iq := makeDeterministicIQ(50000)
	mk := func() *CPUOracleState {
		taps := makeLowpassTaps(65)
		return &CPUOracleState{
			SignalID:       1,
			ConfigHash:     123,
			NCOPhase:       0,
			Decim:          20,
			PhaseCount:     0,
			NumTaps:        65,
			ShiftedHistory: make([]complex64, 0, 64),
			BaseTaps:       taps,
			PolyphaseTaps:  BuildPolyphaseTapsPhaseMajor(taps, 20),
		}
	}
	phaseInc := 0.017
	direct := CPUOracleExtract(iq, mk(), phaseInc)
	poly := CPUOracleExtractPolyphase(iq, mk(), phaseInc)
	requireComplexSlicesClose(t, direct, poly, 1e-5)
}

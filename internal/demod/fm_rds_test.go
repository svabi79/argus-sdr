package demod

import (
	"math/rand"
	"testing"
)

// RDSBasebandComplexInto must be byte-for-byte identical to RDSBasebandComplex,
// including across repeated calls on the same reused scratch (no stale data).
func TestRDSBasebandComplexIntoMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, sr := range []int{200000, 250000, 240000} {
		for _, n := range []int{0, 1, 2, 17, 999, 4096} {
			iq := make([]complex64, n)
			for i := range iq {
				iq[i] = complex(float32(rng.NormFloat64()), float32(rng.NormFloat64()))
			}
			want := RDSBasebandComplex(iq, sr)
			var s RDSScratch
			RDSBasebandComplexInto(iq, sr, &s)        // first call: allocate scratch
			got := RDSBasebandComplexInto(iq, sr, &s) // second call: reuse scratch
			if want.SampleRate != got.SampleRate {
				t.Fatalf("sr=%d n=%d rate: %d vs %d", sr, n, want.SampleRate, got.SampleRate)
			}
			if len(want.Samples) != len(got.Samples) {
				t.Fatalf("sr=%d n=%d len: %d vs %d", sr, n, len(want.Samples), len(got.Samples))
			}
			for i := range want.Samples {
				if want.Samples[i] != got.Samples[i] {
					t.Fatalf("sr=%d n=%d sample %d: %v vs %v", sr, n, i, want.Samples[i], got.Samples[i])
				}
			}
		}
	}
}

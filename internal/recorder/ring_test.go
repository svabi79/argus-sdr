package recorder

import (
	"testing"
	"time"
)

func TestRingSampleCapacityPartialTrim(t *testing.T) {
	r := NewRing(10, 2, 2) // 20 samples capacity
	base := time.Unix(1700000000, 0)

	push := func(start int, n int, t0 time.Time) {
		s := make([]complex64, n)
		for i := range s {
			s[i] = complex(float32(start+i), 0)
		}
		r.Push(t0, s)
	}

	push(0, 8, base)
	push(8, 8, base.Add(800*time.Millisecond))
	push(16, 8, base.Add(1600*time.Millisecond))

	out := r.Slice(base, base.Add(4*time.Second))
	if got, want := len(out), 20; got != want {
		t.Fatalf("len mismatch: got %d want %d", got, want)
	}
	if got, want := int(real(out[0])), 4; got != want {
		t.Fatalf("first sample mismatch: got %d want %d", got, want)
	}
	if got, want := int(real(out[len(out)-1])), 23; got != want {
		t.Fatalf("last sample mismatch: got %d want %d", got, want)
	}
}

func TestRingSampleCapacityVariablePushSizes(t *testing.T) {
	r := NewRing(100, 10, 1) // 100 samples capacity
	base := time.Unix(1700001000, 0)
	offset := 0
	for i := 0; i < 10; i++ {
		block := make([]complex64, 15)
		for j := range block {
			block[j] = complex(float32(offset+j), 0)
		}
		t0 := base.Add(time.Duration(float64(offset) / 100.0 * float64(time.Second)))
		r.Push(t0, block)
		offset += len(block)
	}

	out := r.Slice(base, base.Add(3*time.Second))
	if got, want := len(out), 100; got != want {
		t.Fatalf("len mismatch: got %d want %d", got, want)
	}
	if got, want := int(real(out[0])), 50; got != want {
		t.Fatalf("first sample mismatch: got %d want %d", got, want)
	}
	if got, want := int(real(out[len(out)-1])), 149; got != want {
		t.Fatalf("last sample mismatch: got %d want %d", got, want)
	}
}

// Pushing many blocks forces eviction and buffer recycling (#26). Mutating the
// caller's slice right after Push verifies Push still copies (no alias to the
// caller) and that recycled buffers do not corrupt retained data.
func TestRingPushCopiesAndRecyclesSafely(t *testing.T) {
	r := NewRing(100, 10, 1) // 100 samples capacity
	base := time.Unix(1700002000, 0)
	const block = 15
	offset := 0
	for i := 0; i < 40; i++ {
		s := make([]complex64, block)
		for j := range s {
			s[j] = complex(float32(offset+j), 0)
		}
		t0 := base.Add(time.Duration(float64(offset) / 100.0 * float64(time.Second)))
		r.Push(t0, s)
		for j := range s { // poison the caller buffer after Push
			s[j] = complex(-999, -999)
		}
		offset += block
	}
	out := r.Slice(base, base.Add(time.Hour))
	if got, want := len(out), 100; got != want {
		t.Fatalf("len: got %d want %d", got, want)
	}
	wantFirst := offset - 100 // retained window = last maxSamples
	for i, v := range out {
		if got, want := int(real(v)), wantFirst+i; got != want {
			t.Fatalf("sample %d: got %d want %d (recycling aliased or corrupted data)", i, got, want)
		}
	}
}

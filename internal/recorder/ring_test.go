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

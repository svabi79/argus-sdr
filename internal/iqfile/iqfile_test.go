package iqfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	iq := make([]complex64, 1000)
	for i := range iq {
		iq[i] = complex(float32(i)*0.001, float32(-i)*0.002)
	}
	path := filepath.Join(t.TempDir(), "snap.cf32")
	if err := Write(path, iq, 4_096_000, 101.5e6); err != nil {
		t.Fatal(err)
	}
	got, meta, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if meta.SampleRate != 4_096_000 || meta.CenterHz != 101.5e6 || meta.Samples != len(iq) {
		t.Fatalf("meta mismatch: %+v", meta)
	}
	for i := range iq {
		if got[i] != iq[i] {
			t.Fatalf("sample %d: %v != %v", i, got[i], iq[i])
		}
	}
}

func TestReplaySourceLoops(t *testing.T) {
	iq := []complex64{1, 2, 3}
	path := filepath.Join(t.TempDir(), "s.cf32")
	if err := Write(path, iq, 1000, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	s, _, err := NewSource(path)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := s.ReadIQ(7) // 3 samples, looped
	want := []complex64{1, 2, 3, 1, 2, 3, 1}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("loop sample %d: %v != %v", i, out[i], want[i])
		}
	}
}

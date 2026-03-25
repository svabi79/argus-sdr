package gpudemod

import "testing"

func TestCompareComplexSlices(t *testing.T) {
	a := []complex64{1 + 1i, 2 + 2i, 3 + 3i}
	b := []complex64{1 + 1i, 2.1 + 2i, 2.9 + 3.2i}
	stats := CompareComplexSlices(a, b)
	if stats.Count != 3 {
		t.Fatalf("unexpected count: %d", stats.Count)
	}
	if stats.MaxAbsErr <= 0 {
		t.Fatalf("expected positive max abs error")
	}
	if stats.RMSErr <= 0 {
		t.Fatalf("expected positive rms error")
	}
}

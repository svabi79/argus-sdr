package gpudemod

import "testing"

func TestBuildPolyphaseTapsPhaseMajor(t *testing.T) {
	base := []float32{1, 2, 3, 4, 5, 6, 7}
	got := BuildPolyphaseTapsPhaseMajor(base, 3)
	// phase-major with phase len ceil(7/3)=3
	want := []float32{
		1, 4, 7,
		2, 5, 0,
		3, 6, 0,
	}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mismatch at %d: got %v want %v", i, got[i], want[i])
		}
	}
}

package pipeline

import (
	"math"
	"testing"
)

func cand(centerHz, bwHz, snrDb float64) Candidate {
	return Candidate{CenterHz: centerHz, BandwidthHz: bwHz, SNRDb: snrDb}
}

// centers returns the sorted centers of a candidate set for easy assertions.
func centersOf(cs []Candidate) []float64 {
	out := make([]float64, len(cs))
	for i, c := range cs {
		out[i] = c.CenterHz
	}
	return out
}

func TestFuseScaleAware_SplitsBridgedPair(t *testing.T) {
	// One coarse blob bridging two stations; the fine pass resolves both.
	coarse := []Candidate{cand(0, 400e3, 55)} // spans roughly -200k..+200k
	fine := []Candidate{cand(-100e3, 120e3, 50), cand(100e3, 120e3, 50)}

	got := FuseScaleAware(coarse, fine, ScaleFuseOptions{})
	if len(got) != 2 {
		t.Fatalf("expected 2 resolved emissions, got %d (%v)", len(got), centersOf(got))
	}
	if got[0].CenterHz != -100e3 || got[1].CenterHz != 100e3 {
		t.Errorf("centers = %v, want [-100000 100000]", centersOf(got))
	}
	// Split at the midpoint (0): each child ~ half the coarse span.
	for _, c := range got {
		if c.BandwidthHz <= 0 || c.BandwidthHz > 400e3 {
			t.Errorf("child bw %.0f out of range for a split slice", c.BandwidthHz)
		}
	}
}

func TestFuseScaleAware_KeepsLoneWideWhole(t *testing.T) {
	// A single wide emission: the fine pass sees one (or fragments of one); it
	// must NOT be fragmented — keep the coarse width.
	coarse := []Candidate{cand(0, 180e3, 55)}
	fine := []Candidate{cand(2e3, 60e3, 50)} // one fine center inside
	got := FuseScaleAware(coarse, fine, ScaleFuseOptions{})
	if len(got) != 1 {
		t.Fatalf("expected 1 emission, got %d (%v)", len(got), centersOf(got))
	}
	if got[0].BandwidthHz != 180e3 {
		t.Errorf("lone bw = %.0f, want coarse 180000 (not fragmented)", got[0].BandwidthHz)
	}
	if got[0].CenterHz != 2e3 {
		t.Errorf("center adopted from fine should be 2000, got %.0f", got[0].CenterHz)
	}
}

func TestFuseScaleAware_IgnoresWeakFineSplit(t *testing.T) {
	// A weak fine candidate (sidelobe) below MinSplitSNRDb must not cause a split.
	coarse := []Candidate{cand(0, 180e3, 55)}
	fine := []Candidate{cand(-40e3, 60e3, 50), cand(40e3, 60e3, 4)} // 2nd too weak
	got := FuseScaleAware(coarse, fine, ScaleFuseOptions{MinSplitSNRDb: 8})
	if len(got) != 1 {
		t.Fatalf("weak fine should not split: got %d (%v)", len(got), centersOf(got))
	}
}

func TestFuseScaleAware_IgnoresUnseparatedFine(t *testing.T) {
	// Two fine centers closer than MinSplitSeparationHz = the same station twice.
	coarse := []Candidate{cand(0, 180e3, 55)}
	fine := []Candidate{cand(-5e3, 60e3, 50), cand(5e3, 60e3, 49)} // 10k apart
	got := FuseScaleAware(coarse, fine, ScaleFuseOptions{MinSplitSeparationHz: 30000})
	if len(got) != 1 {
		t.Fatalf("unseparated fine should not split: got %d (%v)", len(got), centersOf(got))
	}
}

func TestFuseScaleAware_EmitsFineOnly(t *testing.T) {
	// A fine emission outside any coarse candidate is carried through.
	coarse := []Candidate{cand(0, 180e3, 55)}
	fine := []Candidate{cand(0, 60e3, 50), cand(800e3, 12e3, 30)} // 2nd is elsewhere
	got := FuseScaleAware(coarse, fine, ScaleFuseOptions{})
	if len(got) != 2 {
		t.Fatalf("expected coarse + fine-only = 2, got %d (%v)", len(got), centersOf(got))
	}
	if math.Abs(got[1].CenterHz-800e3) > 1 {
		t.Errorf("fine-only emission missing; centers = %v", centersOf(got))
	}
}

func TestFuseScaleAware_NoCoarseReturnsFine(t *testing.T) {
	fine := []Candidate{cand(0, 60e3, 50)}
	got := FuseScaleAware(nil, fine, ScaleFuseOptions{})
	if len(got) != 1 {
		t.Fatalf("with no coarse, fine should pass through: got %d", len(got))
	}
}

package pipeline

import "testing"

func TestWantsClass(t *testing.T) {
	if !WantsClass([]string{"WFM", "DMR"}, "wfm") {
		t.Fatalf("expected case-insensitive match")
	}
	if WantsClass([]string{"DMR"}, "WFM") {
		t.Fatalf("unexpected match")
	}
}

func TestCandidatePriorityBoost(t *testing.T) {
	p := Policy{SignalPriorities: []string{"voice", "digital", "cw"}}
	if boost := CandidatePriorityBoost(p, "digital-burst"); boost <= 0 {
		t.Fatalf("expected positive boost, got %v", boost)
	}
}

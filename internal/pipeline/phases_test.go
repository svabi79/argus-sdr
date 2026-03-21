package pipeline

import (
	"testing"

	"sdr-wideband-suite/internal/detector"
)

func TestRefinementResultCarriesDecisions(t *testing.T) {
	res := RefinementResult{
		Signals:   []detector.Signal{{ID: 1}},
		Decisions: []SignalDecision{{ShouldRecord: true}},
	}
	if len(res.Signals) != 1 || len(res.Decisions) != 1 {
		t.Fatalf("unexpected refinement result: %+v", res)
	}
}

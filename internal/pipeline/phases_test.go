package pipeline

import (
	"testing"

	"sdr-wideband-suite/internal/detector"
)

func TestRefinementResultCarriesDecisions(t *testing.T) {
	res := RefinementResult{
		Signals:    []detector.Signal{{ID: 1}},
		Decisions:  []SignalDecision{{ShouldRecord: true}},
		Candidates: []Candidate{{ID: 1}},
	}
	if len(res.Signals) != 1 || len(res.Decisions) != 1 || len(res.Candidates) != 1 {
		t.Fatalf("unexpected refinement result: %+v", res)
	}
}

func TestSurveillanceResultCarriesScheduledCandidates(t *testing.T) {
	res := SurveillanceResult{
		Candidates: []Candidate{{ID: 1}},
		Scheduled:  []ScheduledCandidate{{Candidate: Candidate{ID: 1}, Priority: 10}},
	}
	if len(res.Candidates) != 1 || len(res.Scheduled) != 1 {
		t.Fatalf("unexpected surveillance result: %+v", res)
	}
}

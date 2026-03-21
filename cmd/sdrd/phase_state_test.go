package main

import (
	"testing"

	"sdr-wideband-suite/internal/pipeline"
)

func TestPhaseStateCarriesPhaseResults(t *testing.T) {
	ps := &phaseState{
		surveillance: pipeline.SurveillanceResult{NoiseFloor: -90, Scheduled: []pipeline.ScheduledCandidate{{Candidate: pipeline.Candidate{ID: 1}, Priority: 5}}},
		refinement:   pipeline.RefinementResult{Decisions: []pipeline.SignalDecision{{ShouldRecord: true}}, Candidates: []pipeline.Candidate{{ID: 1}}},
	}
	if ps.surveillance.NoiseFloor != -90 || len(ps.surveillance.Scheduled) != 1 {
		t.Fatalf("unexpected surveillance state: %+v", ps.surveillance)
	}
	if len(ps.refinement.Decisions) != 1 || !ps.refinement.Decisions[0].ShouldRecord || len(ps.refinement.Candidates) != 1 {
		t.Fatalf("unexpected refinement state: %+v", ps.refinement)
	}
}

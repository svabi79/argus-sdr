package main

import (
	"testing"

	"sdr-wideband-suite/internal/pipeline"
)

func TestPhaseStateCarriesPhaseResults(t *testing.T) {
	ps := &phaseState{
		surveillance: pipeline.SurveillanceResult{NoiseFloor: -90},
		refinement:   pipeline.RefinementResult{Decisions: []pipeline.SignalDecision{{ShouldRecord: true}}},
	}
	if ps.surveillance.NoiseFloor != -90 {
		t.Fatalf("unexpected surveillance state: %+v", ps.surveillance)
	}
	if len(ps.refinement.Decisions) != 1 || !ps.refinement.Decisions[0].ShouldRecord {
		t.Fatalf("unexpected refinement state: %+v", ps.refinement)
	}
}

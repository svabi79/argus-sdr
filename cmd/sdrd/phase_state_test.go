package main

import (
	"testing"

	"sdr-wideband-suite/internal/pipeline"
)

func TestPhaseStateCarriesPhaseResults(t *testing.T) {
	ps := &phaseState{
		surveillance:    pipeline.SurveillanceResult{Level: pipeline.AnalysisLevel{Name: "surveillance"}, NoiseFloor: -90, Scheduled: []pipeline.ScheduledCandidate{{Candidate: pipeline.Candidate{ID: 1}, Priority: 5}}},
		refinementInput: pipeline.RefinementInput{Scheduled: []pipeline.ScheduledCandidate{{Candidate: pipeline.Candidate{ID: 1}, Priority: 5}}, SampleRate: 2048000, FFTSize: 2048, CenterHz: 7.1e6},
		refinement:      pipeline.RefinementResult{Level: pipeline.AnalysisLevel{Name: "refinement"}, Decisions: []pipeline.SignalDecision{{ShouldRecord: true}}, Candidates: []pipeline.Candidate{{ID: 1}}},
		queueStats:      decisionQueueStats{RecordQueued: 1},
		presentation:    pipeline.AnalysisLevel{Name: "presentation"},
	}
	if ps.surveillance.NoiseFloor != -90 || len(ps.surveillance.Scheduled) != 1 {
		t.Fatalf("unexpected surveillance state: %+v", ps.surveillance)
	}
	if len(ps.refinementInput.Scheduled) != 1 || ps.refinementInput.SampleRate != 2048000 {
		t.Fatalf("unexpected refinement input: %+v", ps.refinementInput)
	}
	if len(ps.refinement.Decisions) != 1 || !ps.refinement.Decisions[0].ShouldRecord || len(ps.refinement.Candidates) != 1 {
		t.Fatalf("unexpected refinement state: %+v", ps.refinement)
	}
}

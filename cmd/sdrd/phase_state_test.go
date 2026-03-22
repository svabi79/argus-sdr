package main

import (
	"testing"

	"sdr-wideband-suite/internal/pipeline"
)

func TestPhaseStateCarriesPhaseResults(t *testing.T) {
	ps := &phaseState{
		surveillance: pipeline.SurveillanceResult{Level: pipeline.AnalysisLevel{Name: "surveillance"}, NoiseFloor: -90, Scheduled: []pipeline.ScheduledCandidate{{Candidate: pipeline.Candidate{ID: 1}, Priority: 5}}},
		refinement: pipeline.RefinementStep{
			Input:  pipeline.RefinementInput{Scheduled: []pipeline.ScheduledCandidate{{Candidate: pipeline.Candidate{ID: 1}, Priority: 5}}, SampleRate: 2048000, FFTSize: 2048, CenterHz: 7.1e6},
			Result: pipeline.RefinementResult{Level: pipeline.AnalysisLevel{Name: "refinement"}, Decisions: []pipeline.SignalDecision{{ShouldRecord: true}}, Candidates: []pipeline.Candidate{{ID: 1}}},
		},
		queueStats:   decisionQueueStats{RecordQueued: 1},
		presentation: pipeline.AnalysisLevel{Name: "presentation"},
	}
	if ps.surveillance.NoiseFloor != -90 || len(ps.surveillance.Scheduled) != 1 {
		t.Fatalf("unexpected surveillance state: %+v", ps.surveillance)
	}
	if len(ps.refinement.Input.Scheduled) != 1 || ps.refinement.Input.SampleRate != 2048000 {
		t.Fatalf("unexpected refinement input: %+v", ps.refinement.Input)
	}
	if len(ps.refinement.Result.Decisions) != 1 || !ps.refinement.Result.Decisions[0].ShouldRecord || len(ps.refinement.Result.Candidates) != 1 {
		t.Fatalf("unexpected refinement state: %+v", ps.refinement.Result)
	}
}

package pipeline

import (
	"testing"

	"sdr-wideband-suite/internal/detector"
)

func TestRefinementResultCarriesDecisions(t *testing.T) {
	res := RefinementResult{
		Level:      AnalysisLevel{Name: "refinement"},
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

func TestRefinementInputCarriesScheduledCandidates(t *testing.T) {
	res := RefinementInput{
		Level:      AnalysisLevel{Name: "refinement"},
		Candidates: []Candidate{{ID: 2}},
		Scheduled:  []ScheduledCandidate{{Candidate: Candidate{ID: 2}, Priority: 4}},
		Plan: RefinementPlan{
			TotalCandidates: 1,
			Budget:          4,
		},
		SampleRate: 2048000,
		FFTSize:    2048,
		CenterHz:   7.1e6,
		Source:     "surveillance-detector",
	}
	if len(res.Candidates) != 1 || len(res.Scheduled) != 1 {
		t.Fatalf("unexpected refinement input: %+v", res)
	}
	if res.SampleRate != 2048000 || res.FFTSize != 2048 || res.CenterHz != 7.1e6 {
		t.Fatalf("unexpected refinement input fields: %+v", res)
	}
	if res.Plan.TotalCandidates != 1 || res.Plan.Budget != 4 {
		t.Fatalf("unexpected refinement plan fields: %+v", res.Plan)
	}
}

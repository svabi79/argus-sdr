package main

import "sdr-wideband-suite/internal/pipeline"

type phaseState struct {
	surveillance    pipeline.SurveillanceResult
	refinementInput pipeline.RefinementInput
	refinement      pipeline.RefinementResult
	queueStats      decisionQueueStats
	presentation    pipeline.AnalysisLevel
}

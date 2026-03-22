package main

import "sdr-wideband-suite/internal/pipeline"

type phaseState struct {
	surveillance pipeline.SurveillanceResult
	refinement   pipeline.RefinementStep
	queueStats   decisionQueueStats
	presentation pipeline.AnalysisLevel
}

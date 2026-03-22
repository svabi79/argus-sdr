package main

import "sdr-wideband-suite/internal/pipeline"

type phaseState struct {
	surveillance pipeline.SurveillanceResult
	refinement   pipeline.RefinementStep
	arbitration  pipeline.ArbitrationState
	presentation pipeline.AnalysisLevel
}

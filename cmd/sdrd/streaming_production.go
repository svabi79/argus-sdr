package main

import (
	"fmt"

	"sdr-wideband-suite/internal/demod/gpudemod"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/telemetry"
)

func extractForStreamingProduction(
	extractMgr *extractionManager,
	allIQ []complex64,
	sampleRate int,
	centerHz float64,
	signals []detector.Signal,
	aqCfg extractionConfig,
	coll *telemetry.Collector,
) ([][]complex64, []int, error) {
	out := make([][]complex64, len(signals))
	rates := make([]int, len(signals))
	jobs, err := buildStreamingJobs(sampleRate, centerHz, signals, aqCfg)
	if err != nil {
		return nil, nil, err
	}
	runner := extractMgr.get(len(allIQ), sampleRate)
	if runner == nil {
		return nil, nil, fmt.Errorf("streaming production path unavailable: no batch runner")
	}
	results, err := runner.StreamingExtractGPU(allIQ, jobs)
	if err != nil {
		return nil, nil, err
	}
	var oracleResults []gpudemod.StreamingExtractResult
	if useStreamingOraclePath {
		if streamingOracleRunner == nil || streamingOracleRunner.SampleRate != sampleRate {
			streamingOracleRunner = gpudemod.NewCPUOracleRunner(sampleRate)
		}
		oracleResults, _ = streamingOracleRunner.StreamingExtract(allIQ, jobs)
	}
	for i, res := range results {
		out[i] = res.IQ
		rates[i] = res.Rate
		observeStreamingResult(coll, "streaming.production", res)
		if i < len(oracleResults) {
			observeStreamingComparison(coll, oracleResults[i], res)
		}
	}
	return out, rates, nil
}

package main

import (
	"math"

	"sdr-wideband-suite/internal/demod/gpudemod"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/telemetry"
)

const useStreamingOraclePath = false // temporarily disable oracle during bring-up to isolate production-path runtime behavior
const useStreamingProductionPath = true // route top-level extraction through the new production path during bring-up/validation

var streamingOracleRunner *gpudemod.CPUOracleRunner

func buildStreamingJobs(sampleRate int, centerHz float64, signals []detector.Signal, aqCfg extractionConfig) ([]gpudemod.StreamingExtractJob, error) {
	jobs := make([]gpudemod.StreamingExtractJob, len(signals))
	bwMult := aqCfg.bwMult
	if bwMult <= 0 {
		bwMult = 1.0
	}
	firTaps := aqCfg.firTaps
	if firTaps <= 0 {
		firTaps = 101
	}
	for i, sig := range signals {
		bw := sig.BWHz * bwMult
		sigMHz := sig.CenterHz / 1e6
		isWFM := (sigMHz >= 87.5 && sigMHz <= 108.0) ||
			(sig.Class != nil && (sig.Class.ModType == "WFM" || sig.Class.ModType == "WFM_STEREO"))
		var outRate int
		if isWFM {
			outRate = wfmStreamOutRate
			if bw < wfmStreamMinBW {
				bw = wfmStreamMinBW
			}
		} else {
			// Non-WFM target: must be an exact integer divisor of sampleRate.
			// The old hardcoded 200000 fails for common SDR rates (e.g. 4096000/200000=20.48).
			// Find the nearest valid rate >= 128000 (enough for NFM/AM/SSB).
			outRate = nearestExactDecimationRate(sampleRate, 200000, 128000)
			if bw < 20000 {
				bw = 20000
			}
		}
		if _, err := gpudemod.ExactIntegerDecimation(sampleRate, outRate); err != nil {
			return nil, err
		}
		offset := sig.CenterHz - centerHz
		jobs[i] = gpudemod.StreamingExtractJob{
			SignalID:   sig.ID,
			OffsetHz:   offset,
			Bandwidth:  bw,
			OutRate:    outRate,
			NumTaps:    firTaps,
			ConfigHash: gpudemod.StreamingConfigHash(sig.ID, offset, bw, outRate, firTaps, sampleRate),
		}
	}
	return jobs, nil
}

func resetStreamingOracleRunner() {
	if streamingOracleRunner != nil {
		streamingOracleRunner.ResetAllStates()
	}
}

func extractForStreamingOracle(
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
	if streamingOracleRunner == nil || streamingOracleRunner.SampleRate != sampleRate {
		streamingOracleRunner = gpudemod.NewCPUOracleRunner(sampleRate)
	}
	results, err := streamingOracleRunner.StreamingExtract(allIQ, jobs)
	if err != nil {
		return nil, nil, err
	}
	for i, res := range results {
		out[i] = res.IQ
		rates[i] = res.Rate
		observeStreamingResult(coll, "streaming.oracle", res)
	}
	return out, rates, nil
}

func phaseIncForOffset(sampleRate int, offsetHz float64) float64 {
	return -2.0 * math.Pi * offsetHz / float64(sampleRate)
}

// nearestExactDecimationRate finds the output rate closest to targetRate
// (but not below minRate) that is an exact integer divisor of sampleRate.
// This avoids the ExactIntegerDecimation check failing for rates like
// 4096000/200000=20.48 which silently killed the entire streaming batch.
func nearestExactDecimationRate(sampleRate int, targetRate int, minRate int) int {
	if sampleRate <= 0 || targetRate <= 0 {
		return targetRate
	}
	if sampleRate%targetRate == 0 {
		return targetRate // already exact
	}
	// Try decimation factors near the target
	targetDecim := sampleRate / targetRate // floor
	bestRate := 0
	bestDist := sampleRate // impossibly large
	for d := max(1, targetDecim-2); d <= targetDecim+2; d++ {
		rate := sampleRate / d
		if rate < minRate {
			continue
		}
		if sampleRate%rate != 0 {
			continue // not exact (shouldn't happen since rate = sampleRate/d, but guard)
		}
		dist := targetRate - rate
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			bestDist = dist
			bestRate = rate
		}
	}
	if bestRate > 0 {
		return bestRate
	}
	return targetRate // fallback — will fail ExactIntegerDecimation and surface the error
}

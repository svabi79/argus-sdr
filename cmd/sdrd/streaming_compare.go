package main

import (
	"fmt"

	"sdr-wideband-suite/internal/demod/gpudemod"
	"sdr-wideband-suite/internal/telemetry"
)

func observeStreamingComparison(coll *telemetry.Collector, oracle gpudemod.StreamingExtractResult, prod gpudemod.StreamingExtractResult) {
	if coll == nil {
		return
	}
	metrics, stats := gpudemod.CompareOracleAndGPUHostOracle(oracle, prod)
	tags := telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", oracle.SignalID), "path", "streaming_compare")
	coll.SetGauge("streaming.compare.n_out", float64(metrics.NOut), tags)
	coll.SetGauge("streaming.compare.phase_count", float64(metrics.PhaseCount), tags)
	coll.SetGauge("streaming.compare.history_len", float64(metrics.HistoryLen), tags)
	coll.Observe("streaming.compare.ref_max_abs_err", metrics.RefMaxAbsErr, tags)
	coll.Observe("streaming.compare.ref_rms_err", metrics.RefRMSErr, tags)
	coll.SetGauge("streaming.compare.compare_count", float64(stats.Count), tags)
	coll.SetGauge("streaming.compare.oracle_rate", float64(oracle.Rate), tags)
	coll.SetGauge("streaming.compare.production_rate", float64(prod.Rate), tags)
	coll.SetGauge("streaming.compare.oracle_output_len", float64(len(oracle.IQ)), tags)
	coll.SetGauge("streaming.compare.production_output_len", float64(len(prod.IQ)), tags)
	if len(oracle.IQ) > 0 {
		oracleStats := computeIQHeadStats(oracle.IQ, 64)
		coll.Observe("streaming.compare.oracle_head_mean_mag", oracleStats.meanMag, tags)
		coll.Observe("streaming.compare.oracle_head_max_step", oracleStats.maxStep, tags)
	}
	if len(prod.IQ) > 0 {
		prodStats := computeIQHeadStats(prod.IQ, 64)
		coll.Observe("streaming.compare.production_head_mean_mag", prodStats.meanMag, tags)
		coll.Observe("streaming.compare.production_head_max_step", prodStats.maxStep, tags)
	}
	coll.Event("streaming_compare_snapshot", "info", "streaming comparison snapshot", tags, map[string]any{
		"oracle_rate":           oracle.Rate,
		"production_rate":       prod.Rate,
		"oracle_output_len":     len(oracle.IQ),
		"production_output_len": len(prod.IQ),
		"ref_max_abs_err":       metrics.RefMaxAbsErr,
		"ref_rms_err":           metrics.RefRMSErr,
		"compare_count":         stats.Count,
	})
}

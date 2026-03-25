package main

import (
	"fmt"

	"sdr-wideband-suite/internal/demod/gpudemod"
	"sdr-wideband-suite/internal/telemetry"
)

func observeStreamingResult(coll *telemetry.Collector, prefix string, res gpudemod.StreamingExtractResult) {
	if coll == nil {
		return
	}
	tags := telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", res.SignalID), "path", prefix)
	coll.SetGauge(prefix+".n_out", float64(res.NOut), tags)
	coll.SetGauge(prefix+".phase_count", float64(res.PhaseCount), tags)
	coll.SetGauge(prefix+".history_len", float64(res.HistoryLen), tags)
	coll.SetGauge(prefix+".rate", float64(res.Rate), tags)
	coll.SetGauge(prefix+".output_len", float64(len(res.IQ)), tags)
	if len(res.IQ) > 0 {
		stats := computeIQHeadStats(res.IQ, 64)
		coll.Observe(prefix+".head_mean_mag", stats.meanMag, tags)
		coll.Observe(prefix+".head_max_step", stats.maxStep, tags)
		coll.Observe(prefix+".head_p95_step", stats.p95Step, tags)
		coll.SetGauge(prefix+".head_low_magnitude_count", float64(stats.lowMag), tags)
	}
}

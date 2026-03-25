package gpudemod

func BuildOracleDebugMetrics(res StreamingExtractResult) ExtractDebugMetrics {
	return ExtractDebugMetrics{
		SignalID:   res.SignalID,
		PhaseCount: res.PhaseCount,
		HistoryLen: res.HistoryLen,
		NOut:       res.NOut,
	}
}

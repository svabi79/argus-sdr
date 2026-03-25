package gpudemod

func BuildGPUStubDebugMetrics(res StreamingExtractResult) ExtractDebugMetrics {
	return ExtractDebugMetrics{
		SignalID:   res.SignalID,
		PhaseCount: res.PhaseCount,
		HistoryLen: res.HistoryLen,
		NOut:       res.NOut,
	}
}

func BuildGPUHostOracleDebugMetrics(res StreamingExtractResult) ExtractDebugMetrics {
	return ExtractDebugMetrics{
		SignalID:   res.SignalID,
		PhaseCount: res.PhaseCount,
		HistoryLen: res.HistoryLen,
		NOut:       res.NOut,
	}
}

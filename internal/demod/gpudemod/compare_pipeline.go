package gpudemod

func CompareOracleAndGPUStub(oracle StreamingExtractResult, gpu StreamingExtractResult) (ExtractDebugMetrics, CompareStats) {
	stats := CompareComplexSlices(oracle.IQ, gpu.IQ)
	metrics := ExtractDebugMetrics{
		SignalID:     oracle.SignalID,
		PhaseCount:   gpu.PhaseCount,
		HistoryLen:   gpu.HistoryLen,
		NOut:         gpu.NOut,
		RefMaxAbsErr: stats.MaxAbsErr,
		RefRMSErr:    stats.RMSErr,
	}
	return metrics, stats
}

func CompareOracleAndGPUHostOracle(oracle StreamingExtractResult, gpu StreamingExtractResult) (ExtractDebugMetrics, CompareStats) {
	stats := CompareComplexSlices(oracle.IQ, gpu.IQ)
	metrics := ExtractDebugMetrics{
		SignalID:     oracle.SignalID,
		PhaseCount:   gpu.PhaseCount,
		HistoryLen:   gpu.HistoryLen,
		NOut:         gpu.NOut,
		RefMaxAbsErr: stats.MaxAbsErr,
		RefRMSErr:    stats.RMSErr,
	}
	return metrics, stats
}

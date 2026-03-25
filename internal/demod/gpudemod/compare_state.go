package gpudemod

type ExtractDebugMetrics struct {
	SignalID      int64
	PhaseCount    int
	HistoryLen    int
	NOut          int
	RefMaxAbsErr  float64
	RefRMSErr     float64
	BoundaryDelta float64
	BoundaryD2    float64
}

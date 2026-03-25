package gpudemod

type StreamingGPUExecutionMode string

const (
	StreamingGPUExecUnavailable StreamingGPUExecutionMode = "unavailable"
	StreamingGPUExecHostOracle StreamingGPUExecutionMode = "host_oracle"
	StreamingGPUExecCUDA      StreamingGPUExecutionMode = "cuda"
)

type StreamingGPUInvocation struct {
	SignalID       int64
	ConfigHash     uint64
	OffsetHz       float64
	OutRate        int
	Bandwidth      float64
	SampleRate     int
	NumTaps        int
	Decim          int
	PhaseCountIn   int
	NCOPhaseIn     float64
	HistoryLen     int
	BaseTaps       []float32
	PolyphaseTaps  []float32
	ShiftedHistory []complex64
	IQNew          []complex64
}

type StreamingGPUExecutionResult struct {
	SignalID       int64
	Mode           StreamingGPUExecutionMode
	IQ             []complex64
	Rate           int
	NOut           int
	PhaseCountOut  int
	NCOPhaseOut    float64
	HistoryOut     []complex64
	HistoryLenOut  int
}

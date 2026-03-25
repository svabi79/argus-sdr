package gpudemod

import (
	"fmt"
	"hash/fnv"
)

type StreamingExtractJob struct {
	SignalID   int64
	OffsetHz   float64
	Bandwidth  float64
	OutRate    int
	NumTaps    int
	ConfigHash uint64
}

type StreamingExtractResult struct {
	SignalID   int64
	IQ         []complex64
	Rate       int
	NOut       int
	PhaseCount int
	HistoryLen int
}

type ExtractStreamState struct {
	SignalID       int64
	ConfigHash     uint64
	NCOPhase       float64
	Decim          int
	PhaseCount     int
	NumTaps        int
	ShiftedHistory []complex64
	BaseTaps       []float32
	PolyphaseTaps  []float32
	Initialized    bool
}

func ResetExtractStreamState(state *ExtractStreamState, cfgHash uint64) {
	if state == nil {
		return
	}
	state.ConfigHash = cfgHash
	state.NCOPhase = 0
	state.PhaseCount = 0
	state.ShiftedHistory = state.ShiftedHistory[:0]
	state.Initialized = false
}

func StreamingConfigHash(signalID int64, offsetHz float64, bandwidth float64, outRate int, numTaps int, sampleRate int) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("sig=%d|off=%.9f|bw=%.9f|out=%d|taps=%d|sr=%d", signalID, offsetHz, bandwidth, outRate, numTaps, sampleRate)))
	return h.Sum64()
}

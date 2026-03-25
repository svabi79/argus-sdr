package gpudemod

import (
	"fmt"
	"hash/fnv"
	"math"
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
	// Quantize offset and bandwidth to 1 kHz resolution before hashing.
	// The detector's exponential smoothing causes CenterHz (and therefore offsetHz)
	// to jitter by fractions of a Hz every frame. With %.9f formatting, this
	// produced a new hash every frame → full state reset (NCOPhase=0, History=[],
	// PhaseCount=0) → FIR settling + phase discontinuity → audible clicks.
	//
	// The NCO phase_inc is computed from the exact offset each frame, so small
	// frequency changes are tracked smoothly without a reset. Only structural
	// changes (bandwidth affecting FIR taps, decimation, tap count) need a reset.
	qOff := math.Round(offsetHz / 1000) * 1000
	qBW := math.Round(bandwidth / 1000) * 1000
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("sig=%d|off=%.0f|bw=%.0f|out=%d|taps=%d|sr=%d", signalID, qOff, qBW, outRate, numTaps, sampleRate)))
	return h.Sum64()
}

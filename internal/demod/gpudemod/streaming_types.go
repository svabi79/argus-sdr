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
	// Tap cache (#20 / OI-07): BaseTaps/PolyphaseTaps are rebuilt only when a
	// tap-relevant input changes. Identical inputs yield identical taps, so this
	// is byte-for-byte equivalent to rebuilding every frame.
	tapsCutoff     float64
	tapsNumTaps    int
	tapsDecim      int
	tapsSampleRate int
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
	state.BaseTaps = nil // force tap rebuild: a config reset changes the tap geometry
}

func StreamingConfigHash(signalID int64, offsetHz float64, bandwidth float64, outRate int, numTaps int, sampleRate int) uint64 {
	// Hash only structural parameters that change the FIR/decimation geometry.
	// Offset is NOT included because the NCO phase_inc tracks it smoothly each frame.
	// Bandwidth is NOT included because taps are rebuilt every frame in getOrInitExtractState.
	// A state reset (zeroing NCO phase, history, phase count) is only needed when
	// decimation factor, tap count, or sample rate changes — all of which affect
	// buffer sizes and polyphase structure.
	//
	// Previous bug: offset and bandwidth were formatted at %.9f precision, causing
	// a new hash (and full state reset) every single frame because the detector's
	// exponential smoothing changes CenterHz by sub-Hz fractions each frame.
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("sig=%d|out=%d|taps=%d|sr=%d", signalID, outRate, numTaps, sampleRate)))
	return h.Sum64()
}

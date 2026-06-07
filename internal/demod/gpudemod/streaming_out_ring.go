package gpudemod

// streamOutRingDepth must exceed the maximum number of extracted snippets that
// can be in flight across the async streamer feed channel at once. The recorder
// feed channel is buffered 2 plus one frame being processed, so depth >= 4 keeps
// a producer from overwriting a buffer a consumer may still be reading; the extra
// margin guards against scheduling jitter (#20).
const streamOutRingDepth = 8

// streamOutRing is a small ring of reusable output buffers for one signal. The
// streaming extractor's output snippet crosses the async feed channel, so it
// cannot be a single reused buffer — but cycling through streamOutRingDepth
// buffers makes the per-frame output allocation-free while guaranteeing the
// producer never reuses a slot the consumer still holds.
//
// Kept on the BatchRunner (keyed by stable signal ID), NOT on the
// cgo-adjacent nativeStreamingSignalState: a Go slice on that struct makes
// passing &state.dTaps to cudaMalloc trip cgocheck.
type streamOutRing struct {
	bufs [streamOutRingDepth][]complex64
	idx  int
}

// next returns the next ring buffer grown to length n (reused across frames) and
// advances the ring.
func (ring *streamOutRing) next(n int) []complex64 {
	b := ring.bufs[ring.idx]
	if cap(b) < n {
		b = make([]complex64, n)
	} else {
		b = b[:n]
	}
	ring.bufs[ring.idx] = b
	ring.idx++
	if ring.idx >= streamOutRingDepth {
		ring.idx = 0
	}
	return b
}

func (r *BatchRunner) outRingFor(signalID int64) *streamOutRing {
	if r.outRings == nil {
		r.outRings = make(map[int64]*streamOutRing)
	}
	ring := r.outRings[signalID]
	if ring == nil {
		ring = &streamOutRing{}
		r.outRings[signalID] = ring
	}
	return ring
}

// pruneOutRings drops the output rings of signals that are no longer active.
func (r *BatchRunner) pruneOutRings(active map[int64]struct{}) {
	for id := range r.outRings {
		if _, ok := active[id]; !ok {
			delete(r.outRings, id)
		}
	}
}

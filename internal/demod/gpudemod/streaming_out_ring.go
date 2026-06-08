package gpudemod

// streamOutRingDepth must exceed the maximum number of extracted snippets that
// can be in flight across the async streamer feed channel at once. The recorder
// feed channel is buffered 2 plus one frame being processed, so depth >= 4 keeps
// a producer from overwriting a buffer a consumer may still be reading; the extra
// margin guards against scheduling jitter (#20). Depth 4 is the safe minimum
// (feed buffered 2 + one in flight); kept at 4 (not 8) because the per-signal ring
// is a dominant heap consumer under the multi-scale detector's higher signal count
// — 8 buffers x ~210 kB x ~200 signals scanned every GC was a major CPU driver.
const streamOutRingDepth = 4

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
// advances the ring. A grown buffer is drawn from the BatchRunner's cross-signal
// free pool first, so signal-ID churn (transient detections coming and going)
// recycles buffers instead of allocating a fresh ring's worth each time.
func (ring *streamOutRing) next(n int, pool *outBufPool) []complex64 {
	b := ring.bufs[ring.idx]
	if cap(b) < n {
		b = pool.get(n)
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

// outBufPool is a free list of output buffers recycled across signal IDs so that
// transient signals (which create and prune per-ID rings) do not churn the heap
// — the dominant streaming allocator under the multi-scale detector's higher,
// more transient signal count (Constitution XI: make the per-signal path cheap,
// do not decode fewer signals).
type outBufPool struct {
	free [][]complex64
}

// get returns a buffer with cap >= n, reusing a pooled one when available.
func (p *outBufPool) get(n int) []complex64 {
	// reuse the first pooled buffer that already fits; else grow the largest.
	bestIdx := -1
	for i, b := range p.free {
		if cap(b) >= n {
			bestIdx = i
			break
		}
		if bestIdx < 0 || cap(b) > cap(p.free[bestIdx]) {
			bestIdx = i
		}
	}
	if bestIdx >= 0 {
		b := p.free[bestIdx]
		p.free[bestIdx] = p.free[len(p.free)-1]
		p.free = p.free[:len(p.free)-1]
		if cap(b) >= n {
			return b[:n]
		}
		// largest pooled buffer still too small: drop it, allocate a fitting one
	}
	return make([]complex64, n)
}

// put returns buffers to the pool (bounded to avoid unbounded retention).
func (p *outBufPool) put(bufs []([]complex64)) {
	const maxPooled = streamOutRingDepth * 32 // ~32 concurrent signals' worth
	for _, b := range bufs {
		if b == nil || len(p.free) >= maxPooled {
			continue
		}
		p.free = append(p.free, b[:0])
	}
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

// pruneOutRings drops the output rings of signals that are no longer active,
// returning their buffers to the cross-signal free pool for reuse.
func (r *BatchRunner) pruneOutRings(active map[int64]struct{}) {
	for id, ring := range r.outRings {
		if _, ok := active[id]; !ok {
			r.outPool.put(ring.bufs[:])
			delete(r.outRings, id)
		}
	}
}

package recorder

import (
	"sync"
	"time"
)

type iqBlock struct {
	t0      time.Time
	samples []complex64 // live data; after a partial trim this is a sub-slice of buf
	buf     []complex64 // full-capacity backing buffer, recycled on full eviction (#26)
}

// Ring keeps recent IQ blocks for preroll capture.
type Ring struct {
	mu         sync.RWMutex
	blocks     []iqBlock
	maxSamples int
	total      int
	sampleRate int
	free       [][]complex64 // recycled block buffers, reused by Push to avoid a per-ingest allocation (#26)
}

// ringFreeListMax bounds the recycled-buffer pool. In steady state the ring
// pushes ~one block and evicts ~one per frame, so a small pool suffices; the
// bound caps memory if frame sizes vary.
const ringFreeListMax = 8

// getBufLocked returns a length-0 buffer with cap >= n, reusing a recycled block
// buffer when one fits. The caller must hold r.mu.
func (r *Ring) getBufLocked(n int) []complex64 {
	for i := len(r.free) - 1; i >= 0; i-- {
		b := r.free[i]
		if cap(b) >= n {
			r.free[i] = r.free[len(r.free)-1]
			r.free = r.free[:len(r.free)-1]
			return b[:0]
		}
	}
	return make([]complex64, 0, n)
}

// putBufLocked recycles a block buffer. The caller must hold r.mu and must have
// already removed the block from r.blocks, so nothing references the buffer
// (external reads go through Slice/SliceInto, which copy and hold the RLock).
func (r *Ring) putBufLocked(b []complex64) {
	if cap(b) == 0 || len(r.free) >= ringFreeListMax {
		return
	}
	r.free = append(r.free, b[:0])
}

func NewRing(sampleRate int, blockSize int, seconds int) *Ring {
	if seconds <= 0 {
		seconds = 5
	}
	if sampleRate <= 0 {
		sampleRate = 2_048_000
	}
	if blockSize <= 0 {
		blockSize = 2048
	}
	maxSamples := sampleRate * seconds
	minSamples := blockSize * 2
	if minSamples < blockSize {
		minSamples = blockSize
	}
	if maxSamples < minSamples {
		maxSamples = minSamples
	}
	return &Ring{maxSamples: maxSamples, sampleRate: sampleRate}
}

func (r *Ring) Reset(sampleRate int, blockSize int, seconds int) {
	*r = *NewRing(sampleRate, blockSize, seconds)
}

func (r *Ring) Push(t0 time.Time, samples []complex64) {
	if r == nil || len(samples) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := r.getBufLocked(len(samples))[:len(samples)]
	copy(cp, samples)
	r.blocks = append(r.blocks, iqBlock{t0: t0, samples: cp, buf: cp})
	r.total += len(cp)
	for r.total > r.maxSamples && len(r.blocks) > 0 {
		overflow := r.total - r.maxSamples
		head := r.blocks[0]
		if overflow >= len(head.samples) {
			r.total -= len(head.samples)
			r.blocks = r.blocks[1:]
			// Recycle the full backing buffer, not the (possibly trimmed) live
			// slice, so the pool keeps full-capacity buffers that fit new pushes.
			r.putBufLocked(head.buf)
			continue
		}
		trim := overflow
		advance := time.Duration(float64(trim) / float64(r.sampleRate) * float64(time.Second))
		head.t0 = head.t0.Add(advance)
		head.samples = head.samples[trim:]
		r.blocks[0] = head
		r.total -= trim
	}
}

func (r *Ring) MaxSamples() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.maxSamples
}

// Slice returns IQ samples between [start,end] (best-effort).
func (r *Ring) Slice(start, end time.Time) []complex64 {
	return r.SliceInto(start, end, nil)
}

// SliceInto is like Slice but appends into dst (reset to length 0, capacity
// reused), so callers that slice repeatedly (e.g. the per-signal RDS path every
// few seconds) can recycle one large buffer instead of allocating ~100+ MB per
// call — which otherwise dominates allocation/GC and makes stutter scale with the
// number of signals.
func (r *Ring) SliceInto(start, end time.Time, dst []complex64) []complex64 {
	if r == nil || end.Before(start) {
		return dst[:0]
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := dst[:0]
	for _, b := range r.blocks {
		blockDur := time.Duration(float64(len(b.samples)) / float64(r.sampleRate) * float64(time.Second))
		bEnd := b.t0.Add(blockDur)
		if bEnd.Before(start) || b.t0.After(end) {
			continue
		}
		// compute overlap
		oStart := maxTime(start, b.t0)
		oEnd := minTime(end, bEnd)
		if oEnd.Before(oStart) {
			continue
		}
		startIdx := int(float64(oStart.Sub(b.t0)) / float64(time.Second) * float64(r.sampleRate))
		endIdx := int(float64(oEnd.Sub(b.t0)) / float64(time.Second) * float64(r.sampleRate))
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx > len(b.samples) {
			endIdx = len(b.samples)
		}
		if endIdx > startIdx {
			out = append(out, b.samples[startIdx:endIdx]...)
		}
	}
	return out
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

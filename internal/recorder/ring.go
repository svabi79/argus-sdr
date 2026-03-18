package recorder

import (
	"sync"
	"time"
)

type iqBlock struct {
	t0      time.Time
	samples []complex64
}

// Ring keeps recent IQ blocks for preroll capture.
type Ring struct {
	mu         sync.RWMutex
	blocks     []iqBlock
	maxBlocks  int
	sampleRate int
	blockSize  int
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
	blocksPerSec := sampleRate / blockSize
	if blocksPerSec <= 0 {
		blocksPerSec = 1
	}
	maxBlocks := blocksPerSec * seconds
	if maxBlocks < 2 {
		maxBlocks = 2
	}
	return &Ring{maxBlocks: maxBlocks, sampleRate: sampleRate, blockSize: blockSize}
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
	r.blocks = append(r.blocks, iqBlock{t0: t0, samples: append([]complex64(nil), samples...)})
	if len(r.blocks) > r.maxBlocks {
		drop := len(r.blocks) - r.maxBlocks
		r.blocks = r.blocks[drop:]
	}
}

// Slice returns IQ samples between [start,end] (best-effort).
func (r *Ring) Slice(start, end time.Time) []complex64 {
	if r == nil || end.Before(start) {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []complex64
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

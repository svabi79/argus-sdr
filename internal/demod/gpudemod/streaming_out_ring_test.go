package gpudemod

import "testing"

func TestOutBufPoolRecyclesAcrossSignals(t *testing.T) {
	r := &BatchRunner{}
	// signal A fills its ring, then is pruned -> buffers go to pool
	ringA := r.outRingFor(1)
	for i := 0; i < streamOutRingDepth; i++ {
		ringA.next(1000, &r.outPool)
	}
	r.pruneOutRings(map[int64]struct{}{}) // A inactive -> recycled
	if len(r.outPool.free) == 0 {
		t.Fatal("pruned ring buffers not returned to pool")
	}
	pooledBefore := len(r.outPool.free)
	// signal B (new ID, churn) should draw from the pool, not allocate fresh
	ringB := r.outRingFor(2)
	got := ringB.next(1000, &r.outPool)
	if cap(got) < 1000 {
		t.Fatalf("buffer too small: cap=%d", cap(got))
	}
	if len(r.outPool.free) != pooledBefore-1 {
		t.Errorf("new signal did not reuse a pooled buffer (free %d -> %d)", pooledBefore, len(r.outPool.free))
	}
}

func TestOutBufPoolGrowsWhenTooSmall(t *testing.T) {
	p := &outBufPool{}
	p.put([][]complex64{make([]complex64, 100)}) // small pooled buffer
	b := p.get(5000)                             // needs bigger
	if cap(b) < 5000 || len(b) != 5000 {
		t.Fatalf("get(5000) returned cap=%d len=%d", cap(b), len(b))
	}
}

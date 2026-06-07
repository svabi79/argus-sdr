package telemetry

import "testing"

// The metric-history trim must keep the buffer bounded, retain the most recent
// entries, and never reallocate the backing array (it compacts in place) — the
// previous append+copy-the-whole-window-per-record was the top GC allocator (#29).
func TestMetricsHistoryTrimBoundedAndAllocFree(t *testing.T) {
	c, err := New(Config{
		Enabled:           true,
		MetricHistoryMax:  100,
		EventHistoryMax:   100,
		MetricSampleEvery: 1,
		PersistEnabled:    false,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	wantCap := historyCap(100) // 200
	if got := cap(c.metricsHistory); got != wantCap {
		t.Fatalf("initial cap = %d, want %d", got, wantCap)
	}
	for i := 0; i < 1000; i++ {
		c.Observe("m", float64(i), nil)
	}
	if n := len(c.metricsHistory); n < 100 || n > 200 {
		t.Fatalf("history len = %d, want in [100,200]", n)
	}
	if got := cap(c.metricsHistory); got != wantCap {
		t.Fatalf("cap grew to %d (backing reallocated — not alloc-free), want stable %d", got, wantCap)
	}
	if last := c.metricsHistory[len(c.metricsHistory)-1].Value; last != 999 {
		t.Fatalf("most recent value = %v, want 999", last)
	}
	if first := c.metricsHistory[0].Value; first < 700 {
		t.Fatalf("oldest retained value = %v, expected old entries dropped (>=700)", first)
	}
}

func TestEventsHistoryTrimBoundedAndAllocFree(t *testing.T) {
	c, err := New(Config{
		Enabled:          true,
		MetricHistoryMax: 50,
		EventHistoryMax:  50,
		PersistEnabled:   false,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	for i := 0; i < 500; i++ {
		c.Event("e", "info", "msg", nil, nil)
	}
	if n := len(c.events); n < 50 || n > 100 {
		t.Fatalf("events len = %d, want in [50,100]", n)
	}
	if got, want := cap(c.events), historyCap(50); got != want {
		t.Fatalf("events cap = %d (reallocated), want %d", got, want)
	}
}

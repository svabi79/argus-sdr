package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sdr-wideband-suite/internal/detector"
)

func TestReadRecent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	start := time.Now().Add(-5 * time.Minute).UTC().Truncate(time.Millisecond)
	events := []detector.Event{
		{ID: 1, Start: start, End: start.Add(2 * time.Second), CenterHz: 100, Bandwidth: 5, PeakDb: -10, SNRDb: 12},
		{ID: 2, Start: start.Add(10 * time.Second), End: start.Add(12 * time.Second), CenterHz: 200, Bandwidth: 10, PeakDb: -5, SNRDb: 15},
		{ID: 3, Start: start.Add(20 * time.Second), End: start.Add(22 * time.Second), CenterHz: 300, Bandwidth: 20, PeakDb: -3, SNRDb: 18},
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	enc := json.NewEncoder(file)
	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	got, err := ReadRecent(path, 2, time.Time{})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("unexpected IDs: %v, %v", got[0].ID, got[1].ID)
	}

	since := start.Add(15 * time.Second)
	got, err = ReadRecent(path, 10, since)
	if err != nil {
		t.Fatalf("read since: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].ID != 3 {
		t.Fatalf("expected ID 3, got %d", got[0].ID)
	}
}

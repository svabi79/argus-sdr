package recorder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Recording struct {
	ID       string    `json:"id"`
	Start    time.Time `json:"start"`
	CenterHz float64   `json:"center_hz"`
	EventID  int64     `json:"event_id"`
	Path     string    `json:"path"`
}

func ListRecordings(root string) ([]Recording, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Recording
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		metaPath := filepath.Join(root, id, "meta.json")
		b, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var m Meta
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		out = append(out, Recording{ID: id, Start: m.Start, CenterHz: m.CenterHz, EventID: m.EventID, Path: filepath.Join(root, id)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.After(out[j].Start) })
	return out, nil
}

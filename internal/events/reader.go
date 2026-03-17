package events

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"sdr-visual-suite/internal/detector"
)

const (
	defaultLimit = 200
	maxLimit     = 2000
	readChunk    = 64 * 1024
)

// ReadRecent reads the newest events from a JSONL file.
// If since is non-zero, older events (by End time) are skipped.
func ReadRecent(path string, limit int, since time.Time) ([]detector.Event, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		return nil, nil
	}

	lines, err := readLinesReverse(file, info.Size(), limit)
	if err != nil {
		return nil, err
	}

	events := make([]detector.Event, 0, len(lines))
	for _, line := range lines {
		var ev detector.Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if !since.IsZero() && ev.End.Before(since) {
			break
		}
		events = append(events, ev)
	}

	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events, nil
}

func readLinesReverse(file *os.File, size int64, limit int) ([]string, error) {
	pos := size
	remainder := []byte{}
	lines := make([]string, 0, limit)

	for pos > 0 && len(lines) < limit {
		chunkSize := int64(readChunk)
		if chunkSize > pos {
			chunkSize = pos
		}
		pos -= chunkSize
		buf := make([]byte, chunkSize)
		n, err := file.ReadAt(buf, pos)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		buf = buf[:n]
		data := append(buf, remainder...)

		i := len(data)
		for i > 0 && len(lines) < limit {
			j := bytes.LastIndexByte(data[:i], '\n')
			if j == -1 {
				break
			}
			line := bytes.TrimSpace(data[j+1 : i])
			if len(line) > 0 {
				lines = append(lines, string(line))
			}
			i = j
		}
		remainder = data[:i]
	}

	if len(lines) < limit {
		line := bytes.TrimSpace(remainder)
		if len(line) > 0 {
			lines = append(lines, string(line))
		}
	}
	return lines, nil
}

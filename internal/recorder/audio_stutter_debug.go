package recorder

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	audioStutterDebugDir    = "debug/audio-stutter"
	serverStreamSummaryFile = "server_stream_summary.jsonl"
	browserAudioSummaryFile = "browser_audio_summary.jsonl"
)

type audioStutterDebugLogger struct {
	mu            sync.Mutex
	serverFile    *os.File
	serverWriter  *bufio.Writer
	browserFile   *os.File
	browserWriter *bufio.Writer
}

func newAudioStutterDebugLogger() *audioStutterDebugLogger {
	return &audioStutterDebugLogger{}
}

func (l *audioStutterDebugLogger) WriteServerSummary(v any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.ensureServerWriterLocked(); err != nil {
		return err
	}
	return writeJSONLLineLocked(l.serverWriter, v)
}

func (l *audioStutterDebugLogger) WriteBrowserSummary(v any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.ensureBrowserWriterLocked(); err != nil {
		return err
	}
	envelope := map[string]any{
		"ts_server": time.Now().UTC().Format(time.RFC3339Nano),
		"payload":   v,
	}
	return writeJSONLLineLocked(l.browserWriter, envelope)
}

func (l *audioStutterDebugLogger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.serverWriter != nil {
		_ = l.serverWriter.Flush()
	}
	if l.serverFile != nil {
		_ = l.serverFile.Close()
	}
	if l.browserWriter != nil {
		_ = l.browserWriter.Flush()
	}
	if l.browserFile != nil {
		_ = l.browserFile.Close()
	}
	l.serverWriter = nil
	l.serverFile = nil
	l.browserWriter = nil
	l.browserFile = nil
}

func (l *audioStutterDebugLogger) ensureServerWriterLocked() error {
	if l.serverWriter != nil {
		return nil
	}
	if err := os.MkdirAll(audioStutterDebugDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(audioStutterDebugDir, serverStreamSummaryFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	l.serverFile = f
	l.serverWriter = bufio.NewWriterSize(f, 16*1024)
	return nil
}

func (l *audioStutterDebugLogger) ensureBrowserWriterLocked() error {
	if l.browserWriter != nil {
		return nil
	}
	if err := os.MkdirAll(audioStutterDebugDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(audioStutterDebugDir, browserAudioSummaryFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	l.browserFile = f
	l.browserWriter = bufio.NewWriterSize(f, 16*1024)
	return nil
}

func writeJSONLLineLocked(w *bufio.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	if err := w.WriteByte('\n'); err != nil {
		return err
	}
	return w.Flush()
}

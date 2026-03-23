package logging

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type Config struct {
	Level        string   `yaml:"level" json:"level"`
	Categories   []string `yaml:"categories" json:"categories"`
	RateLimitMs  int      `yaml:"rate_limit_ms" json:"rate_limit_ms"`
	Stdout       bool     `yaml:"stdout" json:"stdout"`
	StdoutColor  bool     `yaml:"stdout_color" json:"stdout_color"`
	File         string   `yaml:"file" json:"file"`
	FileLevel    string   `yaml:"file_level" json:"file_level"`
	TimeFormat   string   `yaml:"time_format" json:"time_format"`
	DisableTime  bool     `yaml:"disable_time" json:"disable_time"`
}

type rateLimiter struct {
	mu    sync.Mutex
	last  map[string]time.Time
	limit time.Duration
}

func (r *rateLimiter) allow(key string) bool {
	if r.limit <= 0 {
		return true
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.last[key]; ok {
		if now.Sub(t) < r.limit {
			return false
		}
	}
	r.last[key] = now
	return true
}

var (
	logger     zerolog.Logger
	fileLogger zerolog.Logger
	cfg        Config
	cats       map[string]bool
	rlim       = &rateLimiter{last: map[string]time.Time{}}
	fileHandle *os.File
)

func Init(c Config) error {
	cfg = c
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = "15:04:05"
	}
	cats = map[string]bool{}
	for _, c := range cfg.Categories {
		cats[strings.ToLower(strings.TrimSpace(c))] = true
	}
	rl := time.Duration(cfg.RateLimitMs) * time.Millisecond
	rlim.limit = rl

	level := parseLevel(cfg.Level)
	writers := make([]io.Writer, 0, 2)
	if cfg.Stdout {
		cw := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: cfg.TimeFormat, NoColor: !cfg.StdoutColor}
		if cfg.DisableTime {
			cw.PartsExclude = append(cw.PartsExclude, zerolog.TimestampFieldName)
		}
		writers = append(writers, cw)
	}
	if cfg.File != "" {
		dir := filepath.Dir(cfg.File)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
		fh, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		fileHandle = fh
		writers = append(writers, fh)
		fileLevel := parseLevel(cfg.FileLevel)
		fileLogger = zerolog.New(fh).Level(fileLevel).With().Timestamp().Logger()
	}
	if len(writers) == 0 {
		return errors.New("logging: no outputs enabled")
	}
	mw := io.MultiWriter(writers...)
	logger = zerolog.New(mw).Level(level).With().Timestamp().Logger()
	return nil
}

func Close() {
	if fileHandle != nil {
		_ = fileHandle.Close()
		fileHandle = nil
	}
}

func EnabledCategory(cat string) bool {
	if len(cats) == 0 {
		return true
	}
	_, ok := cats[strings.ToLower(cat)]
	return ok
}

func logf(level zerolog.Level, cat, msg string, kv ...any) {
	if !EnabledCategory(cat) {
		return
	}
	key := cat + ":" + level.String()
	if !rlim.allow(key) {
		return
	}
	if level < logger.GetLevel() {
		return
	}
	l := logger.With().Str("cat", cat).Logger()
	e := (&l).WithLevel(level)
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			continue
		}
		switch v := kv[i+1].(type) {
		case string:
			e = e.Str(k, v)
		case int:
			e = e.Int(k, v)
		case int64:
			e = e.Int64(k, v)
		case float64:
			e = e.Float64(k, v)
		case bool:
			e = e.Bool(k, v)
		default:
			e = e.Interface(k, v)
		}
	}
	e.Msg(msg)
}

func Debug(cat, msg string, kv ...any) { logf(zerolog.DebugLevel, cat, msg, kv...) }
func Info(cat, msg string, kv ...any)  { logf(zerolog.InfoLevel, cat, msg, kv...) }
func Warn(cat, msg string, kv ...any)  { logf(zerolog.WarnLevel, cat, msg, kv...) }
func Error(cat, msg string, kv ...any) { logf(zerolog.ErrorLevel, cat, msg, kv...) }

func parseLevel(raw string) zerolog.Level {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "debug":
		return zerolog.DebugLevel
	case "info", "informal":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

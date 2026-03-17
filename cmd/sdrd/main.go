package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"sdr-visual-suite/internal/config"
	"sdr-visual-suite/internal/detector"
	"sdr-visual-suite/internal/events"
	fftutil "sdr-visual-suite/internal/fft"
	"sdr-visual-suite/internal/mock"
	"sdr-visual-suite/internal/sdr"
	"sdr-visual-suite/internal/sdrplay"
)

type SpectrumFrame struct {
	Timestamp int64             `json:"ts"`
	CenterHz  float64           `json:"center_hz"`
	SampleHz  int               `json:"sample_rate"`
	FFTSize   int               `json:"fft_size"`
	Spectrum  []float64         `json:"spectrum_db"`
	Signals   []detector.Signal `json:"signals"`
}

type hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

func newHub() *hub {
	return &hub{clients: map[*websocket.Conn]struct{}{}}
}

func (h *hub) add(c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
}

func (h *hub) remove(c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
}

func (h *hub) broadcast(frame SpectrumFrame) {
	h.mu.Lock()
	defer h.mu.Unlock()
	b, _ := json.Marshal(frame)
	for c := range h.clients {
		_ = c.WriteMessage(websocket.TextMessage, b)
	}
}

func main() {
	var cfgPath string
	var mockFlag bool
	flag.StringVar(&cfgPath, "config", "config.yaml", "path to config YAML")
	flag.BoolVar(&mockFlag, "mock", false, "use synthetic IQ source")
	flag.Parse()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	var src sdr.Source
	if mockFlag {
		src = mock.New(cfg.SampleRate)
	} else {
		src, err = sdrplay.New(cfg.SampleRate, cfg.CenterHz, cfg.GainDb)
		if err != nil {
			log.Fatalf("sdrplay init failed: %v (try --mock or build with -tags sdrplay)", err)
		}
	}
	if err := src.Start(); err != nil {
		log.Fatalf("source start: %v", err)
	}
	defer src.Stop()

	if err := os.MkdirAll(filepath.Dir(cfg.EventPath), 0o755); err != nil {
		log.Fatalf("event path: %v", err)
	}

	eventFile, err := os.OpenFile(cfg.EventPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("open events: %v", err)
	}
	defer eventFile.Close()

	det := detector.New(cfg.Detector.ThresholdDb, cfg.SampleRate, cfg.FFTSize,
		time.Duration(cfg.Detector.MinDurationMs)*time.Millisecond,
		time.Duration(cfg.Detector.HoldMs)*time.Millisecond)

	window := fftutil.Hann(cfg.FFTSize)
	h := newHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runDSP(ctx, src, cfg, det, window, h, eventFile)

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		h.add(c)
		defer func() {
			h.remove(c)
			_ = c.Close()
		}()
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				return
			}
		}
	})

	http.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cfg)
	})

	http.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		limit := 200
		if v := r.URL.Query().Get("limit"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil {
				limit = parsed
			}
		}
		var since time.Time
		if v := r.URL.Query().Get("since"); v != "" {
			if parsed, err := parseSince(v); err == nil {
				since = parsed
			} else {
				http.Error(w, "invalid since", http.StatusBadRequest)
				return
			}
		}
		evs, err := events.ReadRecent(cfg.EventPath, limit, since)
		if err != nil {
			http.Error(w, "failed to read events", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(evs)
	})

	http.Handle("/", http.FileServer(http.Dir(cfg.WebRoot)))

	server := &http.Server{Addr: cfg.WebAddr}
	go func() {
		log.Printf("web listening on %s", cfg.WebAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()
	_ = server.Shutdown(ctxTimeout)
}

func runDSP(ctx context.Context, src sdr.Source, cfg config.Config, det *detector.Detector, window []float64, h *hub, eventFile *os.File) {
	ticker := time.NewTicker(cfg.FrameInterval())
	defer ticker.Stop()
	enc := json.NewEncoder(eventFile)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			iq, err := src.ReadIQ(cfg.FFTSize)
			if err != nil {
				log.Printf("read IQ: %v", err)
				continue
			}
			spectrum := fftutil.Spectrum(iq, window)
			now := time.Now()
			finished, signals := det.Process(now, spectrum, cfg.CenterHz)
			for _, ev := range finished {
				_ = enc.Encode(ev)
			}
			h.broadcast(SpectrumFrame{
				Timestamp: now.UnixMilli(),
				CenterHz:  cfg.CenterHz,
				SampleHz:  cfg.SampleRate,
				FFTSize:   cfg.FFTSize,
				Spectrum:  spectrum,
				Signals:   signals,
			})
		}
	}
}

func parseSince(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	if ms, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if ms > 1e12 {
			return time.UnixMilli(ms), nil
		}
		return time.Unix(ms, 0), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, raw)
}

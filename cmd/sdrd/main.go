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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"sdr-visual-suite/internal/config"
	"sdr-visual-suite/internal/detector"
	"sdr-visual-suite/internal/dsp"
	"sdr-visual-suite/internal/events"
	fftutil "sdr-visual-suite/internal/fft"
	"sdr-visual-suite/internal/fft/gpufft"
	"sdr-visual-suite/internal/mock"
	"sdr-visual-suite/internal/runtime"
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

type gpuStatus struct {
	mu        sync.RWMutex
	Available bool   `json:"available"`
	Active    bool   `json:"active"`
	Error     string `json:"error"`
}

func (g *gpuStatus) set(active bool, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Active = active
	if err != nil {
		g.Error = err.Error()
	} else {
		g.Error = ""
	}
}

func (g *gpuStatus) snapshot() gpuStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return gpuStatus{Available: g.Available, Active: g.Active, Error: g.Error}
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
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	b, _ := json.Marshal(frame)
	for _, c := range clients {
		_ = c.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
		if err := c.WriteMessage(websocket.TextMessage, b); err != nil {
			h.remove(c)
		}
	}
}

type sourceManager struct {
	mu        sync.RWMutex
	src       sdr.Source
	newSource func(cfg config.Config) (sdr.Source, error)
}

func (m *sourceManager) Restart(cfg config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.src
	_ = old.Stop()
	next, err := m.newSource(cfg)
	if err != nil {
		_ = old.Start()
		m.src = old
		return err
	}
	if err := next.Start(); err != nil {
		_ = next.Stop()
		_ = old.Start()
		m.src = old
		return err
	}
	m.src = next
	return nil
}

func (m *sourceManager) Stats() sdr.SourceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sp, ok := m.src.(sdr.StatsProvider); ok {
		return sp.Stats()
	}
	return sdr.SourceStats{}
}

func (m *sourceManager) Flush() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if fl, ok := m.src.(sdr.Flushable); ok {
		fl.Flush()
	}
}

func newSourceManager(src sdr.Source, newSource func(cfg config.Config) (sdr.Source, error)) *sourceManager {
	return &sourceManager{src: src, newSource: newSource}
}

func (m *sourceManager) Start() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.src.Start()
}

func (m *sourceManager) Stop() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.src.Stop()
}

func (m *sourceManager) ReadIQ(n int) ([]complex64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.src.ReadIQ(n)
}

func (m *sourceManager) ApplyConfig(cfg config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if updatable, ok := m.src.(sdr.ConfigurableSource); ok {
		if err := updatable.UpdateConfig(cfg.SampleRate, cfg.CenterHz, cfg.GainDb, cfg.AGC, cfg.TunerBwKHz); err == nil {
			return nil
		}
	}

	old := m.src
	_ = old.Stop()
	next, err := m.newSource(cfg)
	if err != nil {
		_ = old.Start()
		return err
	}
	if err := next.Start(); err != nil {
		_ = next.Stop()
		_ = old.Start()
		return err
	}
	m.src = next
	return nil
}

type dspUpdate struct {
	cfg       config.Config
	det       *detector.Detector
	window    []float64
	dcBlock   bool
	iqBalance bool
	useGPUFFT bool
}

func pushDSPUpdate(ch chan dspUpdate, update dspUpdate) {
	select {
	case ch <- update:
	default:
		select {
		case <-ch:
		default:
		}
		ch <- update
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

	cfgManager := runtime.New(cfg)
	gpuState := &gpuStatus{Available: gpufft.Available()}

	newSource := func(cfg config.Config) (sdr.Source, error) {
		if mockFlag {
			src := mock.New(cfg.SampleRate)
			if updatable, ok := interface{}(src).(sdr.ConfigurableSource); ok {
				_ = updatable.UpdateConfig(cfg.SampleRate, cfg.CenterHz, cfg.GainDb, cfg.AGC, cfg.TunerBwKHz)
			}
			return src, nil
		}
		src, err := sdrplay.New(cfg.SampleRate, cfg.CenterHz, cfg.GainDb, cfg.TunerBwKHz)
		if err != nil {
			return nil, err
		}
		if updatable, ok := src.(sdr.ConfigurableSource); ok {
			_ = updatable.UpdateConfig(cfg.SampleRate, cfg.CenterHz, cfg.GainDb, cfg.AGC, cfg.TunerBwKHz)
		}
		return src, nil
	}

	src, err := newSource(cfg)
	if err != nil {
		log.Fatalf("sdrplay init failed: %v (try --mock or build with -tags sdrplay)", err)
	}
	srcMgr := newSourceManager(src, newSource)
	if err := srcMgr.Start(); err != nil {
		log.Fatalf("source start: %v", err)
	}
	defer srcMgr.Stop()

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
	dspUpdates := make(chan dspUpdate, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runDSP(ctx, srcMgr, cfg, det, window, h, eventFile, dspUpdates, gpuState)

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
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(cfgManager.Snapshot())
		case http.MethodPost:
			var update runtime.ConfigUpdate
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			prev := cfgManager.Snapshot()
			next, err := cfgManager.ApplyConfig(update)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			sourceChanged := prev.CenterHz != next.CenterHz || prev.SampleRate != next.SampleRate || prev.GainDb != next.GainDb || prev.AGC != next.AGC || prev.TunerBwKHz != next.TunerBwKHz
			if sourceChanged {
				if err := srcMgr.ApplyConfig(next); err != nil {
					cfgManager.Replace(prev)
					http.Error(w, "failed to apply source config", http.StatusInternalServerError)
					return
				}
			}
			detChanged := prev.Detector.ThresholdDb != next.Detector.ThresholdDb ||
				prev.Detector.MinDurationMs != next.Detector.MinDurationMs ||
				prev.Detector.HoldMs != next.Detector.HoldMs ||
				prev.SampleRate != next.SampleRate ||
				prev.FFTSize != next.FFTSize
			windowChanged := prev.FFTSize != next.FFTSize
			var newDet *detector.Detector
			var newWindow []float64
			if detChanged {
				newDet = detector.New(next.Detector.ThresholdDb, next.SampleRate, next.FFTSize,
					time.Duration(next.Detector.MinDurationMs)*time.Millisecond,
					time.Duration(next.Detector.HoldMs)*time.Millisecond)
			}
			if windowChanged {
				newWindow = fftutil.Hann(next.FFTSize)
			}
			pushDSPUpdate(dspUpdates, dspUpdate{
				cfg:       next,
				det:       newDet,
				window:    newWindow,
				dcBlock:   next.DCBlock,
				iqBalance: next.IQBalance,
				useGPUFFT: next.UseGPUFFT,
			})
			_ = json.NewEncoder(w).Encode(next)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/sdr/settings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var update runtime.SettingsUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		prev := cfgManager.Snapshot()
		next, err := cfgManager.ApplySettings(update)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if prev.AGC != next.AGC || prev.TunerBwKHz != next.TunerBwKHz {
			if err := srcMgr.ApplyConfig(next); err != nil {
				cfgManager.Replace(prev)
				http.Error(w, "failed to apply sdr settings", http.StatusInternalServerError)
				return
			}
		}
		if prev.DCBlock != next.DCBlock || prev.IQBalance != next.IQBalance {
			pushDSPUpdate(dspUpdates, dspUpdate{
				cfg:       next,
				dcBlock:   next.DCBlock,
				iqBalance: next.IQBalance,
			})
		}
		_ = json.NewEncoder(w).Encode(next)
	})

	http.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(srcMgr.Stats())
	})

	http.HandleFunc("/api/gpu", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gpuState.snapshot())
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

func runDSP(ctx context.Context, srcMgr *sourceManager, cfg config.Config, det *detector.Detector, window []float64, h *hub, eventFile *os.File, updates <-chan dspUpdate, gpuState *gpuStatus) {
	ticker := time.NewTicker(cfg.FrameInterval())
	defer ticker.Stop()
	logTicker := time.NewTicker(5 * time.Second)
	defer logTicker.Stop()
	enc := json.NewEncoder(eventFile)
	dcBlocker := dsp.NewDCBlocker(0.995)
	dcEnabled := cfg.DCBlock
	iqEnabled := cfg.IQBalance
	plan := fftutil.NewCmplxPlan(cfg.FFTSize)
	useGPU := cfg.UseGPUFFT
	var gpuEngine *gpufft.Engine
	if useGPU && gpuState != nil && gpuState.Available {
		if eng, err := gpufft.New(cfg.FFTSize); err == nil {
			gpuEngine = eng
			gpuState.set(true, nil)
		} else {
			gpuState.set(false, err)
			useGPU = false
		}
	} else if gpuState != nil {
		gpuState.set(false, nil)
	}

	gotSamples := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-logTicker.C:
			st := srcMgr.Stats()
			log.Printf("stats: buf=%d drop=%d reset=%d last=%dms", st.BufferSamples, st.Dropped, st.Resets, st.LastSampleAgoMs)
		case upd := <-updates:
			prevFFT := cfg.FFTSize
			prevUseGPU := useGPU
			cfg = upd.cfg
			if upd.det != nil {
				det = upd.det
			}
			if upd.window != nil {
				window = upd.window
				plan = fftutil.NewCmplxPlan(cfg.FFTSize)
			}
			dcEnabled = upd.dcBlock
			iqEnabled = upd.iqBalance
			if cfg.FFTSize != prevFFT || cfg.UseGPUFFT != prevUseGPU {
				srcMgr.Flush()
				gotSamples = false
				if gpuEngine != nil {
					gpuEngine.Close()
					gpuEngine = nil
				}
				useGPU = cfg.UseGPUFFT
				if useGPU && gpuState != nil && gpuState.Available {
					if eng, err := gpufft.New(cfg.FFTSize); err == nil {
						gpuEngine = eng
						gpuState.set(true, nil)
					} else {
						gpuState.set(false, err)
						useGPU = false
					}
				} else if gpuState != nil {
					gpuState.set(false, nil)
				}
			}
			dcBlocker.Reset()
			ticker.Reset(cfg.FrameInterval())
		case <-ticker.C:
			iq, err := srcMgr.ReadIQ(cfg.FFTSize)
			if err != nil {
				log.Printf("read IQ: %v", err)
				if strings.Contains(err.Error(), "timeout") {
					if err := srcMgr.Restart(cfg); err != nil {
						log.Printf("restart failed: %v", err)
					}
				}
				continue
			}
			if !gotSamples {
				log.Printf("received IQ samples")
				gotSamples = true
			}
			if dcEnabled {
				dcBlocker.Apply(iq)
			}
			if iqEnabled {
				dsp.IQBalance(iq)
			}
			var spectrum []float64
			if useGPU && gpuEngine != nil {
				if len(window) == len(iq) {
					for i := 0; i < len(iq); i++ {
						v := iq[i]
						w := float32(window[i])
						iq[i] = complex(real(v)*w, imag(v)*w)
					}
				}
				out, err := gpuEngine.Exec(iq)
				if err != nil {
					if gpuState != nil {
						gpuState.set(false, err)
					}
					useGPU = false
					spectrum = fftutil.Spectrum(iq, window)
				} else {
					spectrum = fftutil.SpectrumFromFFT(out)
				}
			} else {
				spectrum = fftutil.SpectrumWithPlan(iq, window, plan)
			}
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

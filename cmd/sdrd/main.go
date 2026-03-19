package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"

	"sdr-visual-suite/internal/classifier"
	"sdr-visual-suite/internal/config"
	"sdr-visual-suite/internal/detector"
	"sdr-visual-suite/internal/dsp"
	"sdr-visual-suite/internal/events"
	fftutil "sdr-visual-suite/internal/fft"
	"sdr-visual-suite/internal/fft/gpufft"
	"sdr-visual-suite/internal/mock"
	"sdr-visual-suite/internal/recorder"
	"sdr-visual-suite/internal/runtime"
	"sdr-visual-suite/internal/sdr"
	"sdr-visual-suite/internal/sdrplay"
)

type SpectrumFrame struct {
	Timestamp  int64             `json:"ts"`
	CenterHz   float64           `json:"center_hz"`
	SampleHz   int               `json:"sample_rate"`
	FFTSize    int               `json:"fft_size"`
	Spectrum   []float64         `json:"spectrum_db"`
	Thresholds []float64         `json:"thresholds,omitempty"`
	NoiseFloor float64           `json:"noise_floor,omitempty"`
	Signals    []detector.Signal `json:"signals"`
}

type client struct {
	conn      *websocket.Conn
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
}

type hub struct {
	mu        sync.Mutex
	clients   map[*client]struct{}
	frameCnt  int64
	lastLogTs time.Time
}

type gpuStatus struct {
	mu        sync.RWMutex
	Available bool   `json:"available"`
	Active    bool   `json:"active"`
	Error     string `json:"error"`
}

type signalSnapshot struct {
	mu      sync.RWMutex
	signals []detector.Signal
}

func (s *signalSnapshot) set(sig []detector.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signals = append([]detector.Signal(nil), sig...)
}

func (s *signalSnapshot) get() []detector.Signal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]detector.Signal(nil), s.signals...)
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
	return &hub{clients: map[*client]struct{}{}, lastLogTs: time.Now()}
}

func (h *hub) add(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	log.Printf("ws connected (%d clients)", len(h.clients))
}

func (h *hub) remove(c *client) {
	c.closeOnce.Do(func() { close(c.done) })
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	log.Printf("ws disconnected (%d clients)", len(h.clients))
}

func (h *hub) broadcast(frame SpectrumFrame) {
	b, err := json.Marshal(frame)
	if err != nil {
		log.Printf("marshal frame: %v", err)
		return
	}

	h.mu.Lock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		select {
		case c.send <- b:
		default:
			h.remove(c)
		}
	}
	h.frameCnt++
	if time.Since(h.lastLogTs) > 2*time.Second {
		h.lastLogTs = time.Now()
		log.Printf("broadcast frames=%d clients=%d", h.frameCnt, len(clients))
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
	eventMu := &sync.RWMutex{}

	det := detector.New(cfg.Detector, cfg.SampleRate, cfg.FFTSize)

	window := fftutil.Hann(cfg.FFTSize)
	h := newHub()
	dspUpdates := make(chan dspUpdate, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	decodeMap := buildDecoderMap(cfg)
	recMgr := recorder.New(cfg.SampleRate, cfg.FFTSize, recorder.Policy{
		Enabled:     cfg.Recorder.Enabled,
		MinSNRDb:    cfg.Recorder.MinSNRDb,
		MinDuration: mustParseDuration(cfg.Recorder.MinDuration, 1*time.Second),
		MaxDuration: mustParseDuration(cfg.Recorder.MaxDuration, 300*time.Second),
		PrerollMs:   cfg.Recorder.PrerollMs,
		RecordIQ:    cfg.Recorder.RecordIQ,
		RecordAudio: cfg.Recorder.RecordAudio,
		AutoDemod:   cfg.Recorder.AutoDemod,
		AutoDecode:  cfg.Recorder.AutoDecode,
		MaxDiskMB:   cfg.Recorder.MaxDiskMB,
		OutputDir:   cfg.Recorder.OutputDir,
		ClassFilter: cfg.Recorder.ClassFilter,
		RingSeconds: cfg.Recorder.RingSeconds,
	}, cfg.CenterHz, decodeMap)

	sigSnap := &signalSnapshot{}

	go runDSP(ctx, srcMgr, cfg, det, window, h, eventFile, eventMu, dspUpdates, gpuState, recMgr, sigSnap)

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" || origin == "null" {
			return true
		}
		// allow same-host or any local IP
		return true
	}}
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade failed: %v (origin: %s)", err, r.Header.Get("Origin"))
			return
		}
		c := &client{conn: conn, send: make(chan []byte, 32), done: make(chan struct{})}
		h.add(c)
		defer func() {
			h.remove(c)
			_ = conn.Close()
		}()
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})
		go func() {
			ping := time.NewTicker(30 * time.Second)
			defer ping.Stop()
			for {
				select {
				case msg, ok := <-c.send:
					if !ok {
						return
					}
					_ = conn.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
					if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
						return
					}
				case <-ping.C:
					_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						log.Printf("ws ping error: %v", err)
						return
					}
				}
			}
		}()
		for {
			_, _, err := conn.ReadMessage()
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
			if err := config.Save(cfgPath, next); err != nil {
				log.Printf("config save failed: %v", err)
			}
			detChanged := prev.Detector.ThresholdDb != next.Detector.ThresholdDb ||
				prev.Detector.MinDurationMs != next.Detector.MinDurationMs ||
				prev.Detector.HoldMs != next.Detector.HoldMs ||
				prev.Detector.EmaAlpha != next.Detector.EmaAlpha ||
				prev.Detector.HysteresisDb != next.Detector.HysteresisDb ||
				prev.Detector.MinStableFrames != next.Detector.MinStableFrames ||
				prev.Detector.GapToleranceMs != next.Detector.GapToleranceMs ||
				prev.Detector.CFARMode != next.Detector.CFARMode ||
				prev.Detector.CFARGuardCells != next.Detector.CFARGuardCells ||
				prev.Detector.CFARTrainCells != next.Detector.CFARTrainCells ||
				prev.Detector.CFARRank != next.Detector.CFARRank ||
				prev.Detector.CFARScaleDb != next.Detector.CFARScaleDb ||
				prev.Detector.CFARWrapAround != next.Detector.CFARWrapAround ||
				prev.SampleRate != next.SampleRate ||
				prev.FFTSize != next.FFTSize
			windowChanged := prev.FFTSize != next.FFTSize
			var newDet *detector.Detector
			var newWindow []float64
			if detChanged {
				newDet = detector.New(next.Detector, next.SampleRate, next.FFTSize)
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
		if err := config.Save(cfgPath, next); err != nil {
			log.Printf("config save failed: %v", err)
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
		snap := cfgManager.Snapshot()
		eventMu.RLock()
		evs, err := events.ReadRecent(snap.EventPath, limit, since)
		eventMu.RUnlock()
		if err != nil {
			http.Error(w, "failed to read events", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(evs)
	})

	http.HandleFunc("/api/signals", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if sigSnap == nil {
			_ = json.NewEncoder(w).Encode([]detector.Signal{})
			return
		}
		_ = json.NewEncoder(w).Encode(sigSnap.get())
	})

	http.HandleFunc("/api/decoders", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(decoderKeys(cfgManager.Snapshot()))
	})

	http.HandleFunc("/api/recordings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		snap := cfgManager.Snapshot()
		list, err := recorder.ListRecordings(snap.Recorder.OutputDir)
		if err != nil {
			http.Error(w, "failed to list recordings", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(list)
	})

	http.HandleFunc("/api/recordings/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := strings.TrimPrefix(r.URL.Path, "/api/recordings/")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		snap := cfgManager.Snapshot()
		base := filepath.Clean(filepath.Join(snap.Recorder.OutputDir, id))
		if !strings.HasPrefix(base, filepath.Clean(snap.Recorder.OutputDir)) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		if r.URL.Path == "/api/recordings/"+id+"/audio" {
			http.ServeFile(w, r, filepath.Join(base, "audio.wav"))
			return
		}
		if r.URL.Path == "/api/recordings/"+id+"/iq" {
			http.ServeFile(w, r, filepath.Join(base, "signal.cf32"))
			return
		}
		if r.URL.Path == "/api/recordings/"+id+"/decode" {
			mode := r.URL.Query().Get("mode")
			cmd := buildDecoderMap(cfgManager.Snapshot())[mode]
			if cmd == "" {
				http.Error(w, "decoder not configured", http.StatusBadRequest)
				return
			}
			meta, err := recorder.ReadMeta(filepath.Join(base, "meta.json"))
			if err != nil {
				http.Error(w, "meta read failed", http.StatusInternalServerError)
				return
			}
			audioPath := filepath.Join(base, "audio.wav")
			if _, errStat := os.Stat(audioPath); errStat != nil {
				audioPath = ""
			}
			res, err := recorder.DecodeOnDemand(cmd, filepath.Join(base, "signal.cf32"), meta.SampleRate, audioPath)
			if err != nil {
				http.Error(w, res.Stderr, http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(res)
			return
		}
		// default: meta.json
		http.ServeFile(w, r, filepath.Join(base, "meta.json"))
	})

	http.HandleFunc("/api/demod", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		freq, _ := strconv.ParseFloat(q.Get("freq"), 64)
		bw, _ := strconv.ParseFloat(q.Get("bw"), 64)
		sec, _ := strconv.Atoi(q.Get("sec"))
		if sec < 1 {
			sec = 1
		}
		if sec > 10 {
			sec = 10
		}
		mode := q.Get("mode")
		data, _, err := recMgr.DemodLive(freq, bw, mode, sec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(data)
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

func runDSP(ctx context.Context, srcMgr *sourceManager, cfg config.Config, det *detector.Detector, window []float64, h *hub, eventFile *os.File, eventMu *sync.RWMutex, updates <-chan dspUpdate, gpuState *gpuStatus, rec *recorder.Manager, sigSnap *signalSnapshot) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("FATAL: runDSP goroutine panic: %v\n%s", r, debug.Stack())
		}
	}()
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
	if useGPU && gpuState != nil {
		snap := gpuState.snapshot()
		if snap.Available {
			if eng, err := gpufft.New(cfg.FFTSize); err == nil {
				gpuEngine = eng
				gpuState.set(true, nil)
			} else {
				gpuState.set(false, err)
				useGPU = false
			}
		} else {
			gpuState.set(false, nil)
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
			if rec != nil {
				rec.Update(cfg.SampleRate, cfg.FFTSize, recorder.Policy{
					Enabled:     cfg.Recorder.Enabled,
					MinSNRDb:    cfg.Recorder.MinSNRDb,
					MinDuration: mustParseDuration(cfg.Recorder.MinDuration, 1*time.Second),
					MaxDuration: mustParseDuration(cfg.Recorder.MaxDuration, 300*time.Second),
					PrerollMs:   cfg.Recorder.PrerollMs,
					RecordIQ:    cfg.Recorder.RecordIQ,
					RecordAudio: cfg.Recorder.RecordAudio,
					AutoDemod:   cfg.Recorder.AutoDemod,
					AutoDecode:  cfg.Recorder.AutoDecode,
					MaxDiskMB:   cfg.Recorder.MaxDiskMB,
					OutputDir:   cfg.Recorder.OutputDir,
					ClassFilter: cfg.Recorder.ClassFilter,
					RingSeconds: cfg.Recorder.RingSeconds,
				}, cfg.CenterHz, buildDecoderMap(cfg))
			}
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
				if useGPU && gpuState != nil {
					snap := gpuState.snapshot()
					if snap.Available {
						if eng, err := gpufft.New(cfg.FFTSize); err == nil {
							gpuEngine = eng
							gpuState.set(true, nil)
						} else {
							gpuState.set(false, err)
							useGPU = false
						}
					} else {
						gpuState.set(false, nil)
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
			if rec != nil {
				rec.Ingest(time.Now(), iq)
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
					spectrum = fftutil.SpectrumWithPlan(iq, nil, plan)
				} else {
					spectrum = fftutil.SpectrumFromFFT(out)
				}
			} else {
				spectrum = fftutil.SpectrumWithPlan(iq, window, plan)
			}
			for i := range spectrum {
				if math.IsNaN(spectrum[i]) || math.IsInf(spectrum[i], 0) {
					spectrum[i] = -200
				}
			}
			now := time.Now()
			finished, signals := det.Process(now, spectrum, cfg.CenterHz)
			thresholds := det.LastThresholds()
			noiseFloor := det.LastNoiseFloor()
			// enrich classification with temporal IQ features on per-signal snippet
			if len(iq) > 0 {
				for i := range signals {
					snip := extractSignalIQ(iq, cfg.SampleRate, cfg.CenterHz, signals[i].CenterHz, signals[i].BWHz)
					cls := classifier.Classify(classifier.SignalInput{FirstBin: signals[i].FirstBin, LastBin: signals[i].LastBin, SNRDb: signals[i].SNRDb}, spectrum, cfg.SampleRate, cfg.FFTSize, snip)
					signals[i].Class = cls
				}
				det.UpdateClasses(signals)
			}
			if sigSnap != nil {
				sigSnap.set(signals)
			}
			eventMu.Lock()
			for _, ev := range finished {
				_ = enc.Encode(ev)
			}
			eventMu.Unlock()
			if rec != nil && len(finished) > 0 {
				evCopy := make([]detector.Event, len(finished))
				copy(evCopy, finished)
				go rec.OnEvents(evCopy)
			}
			h.broadcast(SpectrumFrame{
				Timestamp:  now.UnixMilli(),
				CenterHz:   cfg.CenterHz,
				SampleHz:   cfg.SampleRate,
				FFTSize:    cfg.FFTSize,
				Spectrum:   spectrum,
				Thresholds: thresholds,
				NoiseFloor: noiseFloor,
				Signals:    signals,
			})
		}
	}
}

func mustParseDuration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	return fallback
}

func buildDecoderMap(cfg config.Config) map[string]string {
	out := map[string]string{}
	if cfg.Decoder.FT8Cmd != "" {
		out["FT8"] = cfg.Decoder.FT8Cmd
	}
	if cfg.Decoder.WSPRCmd != "" {
		out["WSPR"] = cfg.Decoder.WSPRCmd
	}
	if cfg.Decoder.DMRCmd != "" {
		out["DMR"] = cfg.Decoder.DMRCmd
	}
	if cfg.Decoder.DStarCmd != "" {
		out["D-STAR"] = cfg.Decoder.DStarCmd
	}
	if cfg.Decoder.FSKCmd != "" {
		out["FSK"] = cfg.Decoder.FSKCmd
	}
	if cfg.Decoder.PSKCmd != "" {
		out["PSK"] = cfg.Decoder.PSKCmd
	}
	return out
}

func decoderKeys(cfg config.Config) []string {
	m := buildDecoderMap(cfg)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func extractSignalIQ(iq []complex64, sampleRate int, centerHz float64, sigHz float64, bwHz float64) []complex64 {
	if len(iq) == 0 || sampleRate <= 0 {
		return nil
	}
	offset := sigHz - centerHz
	shifted := dsp.FreqShift(iq, sampleRate, offset)
	cutoff := bwHz / 2
	if cutoff < 200 {
		cutoff = 200
	}
	if cutoff > float64(sampleRate)/2-1 {
		cutoff = float64(sampleRate)/2 - 1
	}
	taps := dsp.LowpassFIR(cutoff, sampleRate, 101)
	filtered := dsp.ApplyFIR(shifted, taps)
	decim := sampleRate / 200000
	if decim < 1 {
		decim = 1
	}
	return dsp.Decimate(filtered, decim)
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

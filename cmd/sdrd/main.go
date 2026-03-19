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
	"sdr-visual-suite/internal/events"
	fftutil "sdr-visual-suite/internal/fft"
	"sdr-visual-suite/internal/fft/gpufft"
	"sdr-visual-suite/internal/mock"
	"sdr-visual-suite/internal/recorder"
	"sdr-visual-suite/internal/runtime"
	"sdr-visual-suite/internal/sdr"
	"sdr-visual-suite/internal/sdrplay"
)

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



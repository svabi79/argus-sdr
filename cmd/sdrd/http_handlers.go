package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"sdr-visual-suite/internal/config"
	"sdr-visual-suite/internal/detector"
	"sdr-visual-suite/internal/events"
	fftutil "sdr-visual-suite/internal/fft"
	"sdr-visual-suite/internal/recorder"
	"sdr-visual-suite/internal/runtime"
)

func registerAPIHandlers(mux *http.ServeMux, cfgPath string, cfgManager *runtime.Manager, srcMgr *sourceManager, dspUpdates chan dspUpdate, gpuState *gpuStatus, recMgr *recorder.Manager, sigSnap *signalSnapshot, eventMu *sync.RWMutex) {
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
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
				prev.Detector.CFARGuardHz != next.Detector.CFARGuardHz ||
				prev.Detector.CFARTrainHz != next.Detector.CFARTrainHz ||
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
			pushDSPUpdate(dspUpdates, dspUpdate{cfg: next, det: newDet, window: newWindow, dcBlock: next.DCBlock, iqBalance: next.IQBalance, useGPUFFT: next.UseGPUFFT})
			_ = json.NewEncoder(w).Encode(next)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/sdr/settings", func(w http.ResponseWriter, r *http.Request) {
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
			pushDSPUpdate(dspUpdates, dspUpdate{cfg: next, dcBlock: next.DCBlock, iqBalance: next.IQBalance})
		}
		if err := config.Save(cfgPath, next); err != nil {
			log.Printf("config save failed: %v", err)
		}
		_ = json.NewEncoder(w).Encode(next)
	})

	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(srcMgr.Stats())
	})
	mux.HandleFunc("/api/gpu", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gpuState.snapshot())
	})
	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/api/signals", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if sigSnap == nil {
			_ = json.NewEncoder(w).Encode([]detector.Signal{})
			return
		}
		_ = json.NewEncoder(w).Encode(sigSnap.get())
	})
	mux.HandleFunc("/api/decoders", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(decoderKeys(cfgManager.Snapshot()))
	})
	mux.HandleFunc("/api/recordings", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/api/recordings/", func(w http.ResponseWriter, r *http.Request) {
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
		http.ServeFile(w, r, filepath.Join(base, "meta.json"))
	})
	mux.HandleFunc("/api/demod", func(w http.ResponseWriter, r *http.Request) {
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
}

func newHTTPServer(addr string, webRoot string, h *hub, cfgPath string, cfgManager *runtime.Manager, srcMgr *sourceManager, dspUpdates chan dspUpdate, gpuState *gpuStatus, recMgr *recorder.Manager, sigSnap *signalSnapshot, eventMu *sync.RWMutex) *http.Server {
	mux := http.NewServeMux()
	registerWSHandlers(mux, h)
	registerAPIHandlers(mux, cfgPath, cfgManager, srcMgr, dspUpdates, gpuState, recMgr, sigSnap, eventMu)
	mux.Handle("/", http.FileServer(http.Dir(webRoot)))
	return &http.Server{Addr: addr, Handler: mux}
}

func shutdownServer(server *http.Server) {
	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()
	_ = server.Shutdown(ctxTimeout)
}

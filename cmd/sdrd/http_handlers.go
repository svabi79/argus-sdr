package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/events"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/pipeline"
	"sdr-wideband-suite/internal/recorder"
	"sdr-wideband-suite/internal/runtime"
	"sdr-wideband-suite/internal/telemetry"
)

func registerAPIHandlers(mux *http.ServeMux, cfgPath string, cfgManager *runtime.Manager, srcMgr *sourceManager, dspUpdates chan dspUpdate, gpuState *gpuStatus, recMgr *recorder.Manager, sigSnap *signalSnapshot, eventMu *sync.RWMutex, phaseSnap *phaseSnapshot, telem *telemetry.Collector) {
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
			if update.Pipeline != nil && update.Pipeline.Profile != nil {
				if prof, ok := pipeline.ResolveProfile(next, *update.Pipeline.Profile); ok {
					pipeline.MergeProfile(&next, prof)
					cfgManager.Replace(next)
				}
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
	mux.HandleFunc("/api/pipeline/policy", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cfg := cfgManager.Snapshot()
		_ = json.NewEncoder(w).Encode(pipeline.PolicyFromConfig(cfg))
	})
	mux.HandleFunc("/api/pipeline/recommendations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cfg := cfgManager.Snapshot()
		policy := pipeline.PolicyFromConfig(cfg)
		budget := pipeline.BudgetModelFromPolicy(policy)
		recommend := map[string]any{
			"profile":                policy.Profile,
			"mode":                   policy.Mode,
			"intent":                 policy.Intent,
			"surveillance_strategy":  policy.SurveillanceStrategy,
			"surveillance_detection": policy.SurveillanceDetection,
			"refinement_strategy":    policy.RefinementStrategy,
			"monitor_center_hz":      policy.MonitorCenterHz,
			"monitor_start_hz":       policy.MonitorStartHz,
			"monitor_end_hz":         policy.MonitorEndHz,
			"monitor_span_hz":        policy.MonitorSpanHz,
			"monitor_windows":        policy.MonitorWindows,
			"signal_priorities":      policy.SignalPriorities,
			"auto_record_classes":    policy.AutoRecordClasses,
			"auto_decode_classes":    policy.AutoDecodeClasses,
			"refinement_jobs":        policy.MaxRefinementJobs,
			"refinement_detail_fft":  policy.RefinementDetailFFTSize,
			"refinement_auto_span":   policy.RefinementAutoSpan,
			"refinement_min_span_hz": policy.RefinementMinSpanHz,
			"refinement_max_span_hz": policy.RefinementMaxSpanHz,
			"budgets":                budget,
		}
		_ = json.NewEncoder(w).Encode(recommend)
	})
	mux.HandleFunc("/api/refinement", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		snap := phaseSnap.Snapshot()
		windowSummary := buildWindowSummary(snap.refinement.Input.Plan, snap.refinement.Input.Windows, snap.surveillance.Candidates, snap.refinement.Input.WorkItems, snap.refinement.Result.Decisions)
		var windowStats *RefinementWindowStats
		var monitorSummary []pipeline.MonitorWindowStats
		if windowSummary != nil {
			windowStats = windowSummary.Refinement
			monitorSummary = windowSummary.MonitorWindows
		}
		if windowStats == nil {
			windowStats = buildWindowStats(snap.refinement.Input.Windows)
		}
		if len(monitorSummary) == 0 && len(snap.refinement.Input.Plan.MonitorWindowStats) > 0 {
			monitorSummary = snap.refinement.Input.Plan.MonitorWindowStats
		}
		arbitration := buildArbitrationSnapshot(snap.refinement, snap.arbitration)
		levelSet := snap.surveillance.LevelSet
		spectraBins := map[string]int{}
		for _, spec := range snap.surveillance.Spectra {
			if len(spec.Spectrum) == 0 {
				continue
			}
			spectraBins[spec.Level.Name] = len(spec.Spectrum)
		}
		levelSummaries := buildSurveillanceLevelSummaries(levelSet, snap.surveillance.Spectra)
		candidateSources := buildCandidateSourceSummary(snap.surveillance.Candidates)
		candidateEvidence := buildCandidateEvidenceSummary(snap.surveillance.Candidates)
		candidateEvidenceStates := buildCandidateEvidenceStateSummary(snap.surveillance.Candidates)
		candidateWindows := buildCandidateWindowSummary(snap.surveillance.Candidates, snap.refinement.Input.Plan.MonitorWindows)
		out := map[string]any{
			"plan":                          snap.refinement.Input.Plan,
			"windows":                       snap.refinement.Input.Windows,
			"window_stats":                  windowStats,
			"window_summary":                windowSummary,
			"request":                       snap.refinement.Input.Request,
			"context":                       snap.refinement.Input.Context,
			"detail_level":                  snap.refinement.Input.Detail,
			"arbitration":                   arbitration,
			"work_items":                    snap.refinement.Input.WorkItems,
			"candidates":                    len(snap.refinement.Input.Candidates),
			"scheduled":                     len(snap.refinement.Input.Scheduled),
			"signals":                       len(snap.refinement.Result.Signals),
			"decisions":                     len(snap.refinement.Result.Decisions),
			"surveillance_level":            snap.surveillance.Level,
			"surveillance_levels":           snap.surveillance.Levels,
			"surveillance_level_set":        levelSet,
			"surveillance_detection_policy": snap.surveillance.DetectionPolicy,
			"surveillance_detection_levels": levelSet.Detection,
			"surveillance_support_levels":   levelSet.Support,
			"surveillance_active_levels": func() []pipeline.AnalysisLevel {
				if len(levelSet.All) > 0 {
					return levelSet.All
				}
				active := make([]pipeline.AnalysisLevel, 0, len(snap.surveillance.Levels)+1)
				if snap.surveillance.Level.Name != "" {
					active = append(active, snap.surveillance.Level)
				}
				active = append(active, snap.surveillance.Levels...)
				if snap.surveillance.DisplayLevel.Name != "" {
					active = append(active, snap.surveillance.DisplayLevel)
				}
				return active
			}(),
			"surveillance_level_summary": levelSummaries,
			"surveillance_spectra_bins":  spectraBins,
			"candidate_sources":          candidateSources,
			"candidate_evidence":         candidateEvidence,
			"candidate_evidence_states":  candidateEvidenceStates,
			"candidate_windows":          candidateWindows,
			"monitor_windows":            snap.refinement.Input.Plan.MonitorWindows,
			"monitor_window_stats":       monitorSummary,
			"display_level":              snap.surveillance.DisplayLevel,
			"refinement_level":           snap.refinement.Input.Level,
			"presentation_level":         snap.presentation,
		}
		_ = json.NewEncoder(w).Encode(out)
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
	mux.HandleFunc("/api/candidates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if sigSnap == nil {
			_ = json.NewEncoder(w).Encode([]pipeline.Candidate{})
			return
		}
		sigs := sigSnap.get()
		_ = json.NewEncoder(w).Encode(pipeline.CandidatesFromSignals(sigs, "tracked-signal-snapshot"))
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
	mux.HandleFunc("/api/streams", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		n := recMgr.ActiveStreams()
		_ = json.NewEncoder(w).Encode(map[string]any{"active_sessions": n})
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
	mux.HandleFunc("/api/debug/telemetry/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if telem == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false, "error": "telemetry unavailable"})
			return
		}
		_ = json.NewEncoder(w).Encode(telem.LiveSnapshot())
	})
	mux.HandleFunc("/api/debug/telemetry/history", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if telem == nil {
			http.Error(w, "telemetry unavailable", http.StatusServiceUnavailable)
			return
		}
		query, err := telemetryQueryFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items, err := telem.QueryMetrics(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "count": len(items)})
	})
	mux.HandleFunc("/api/debug/telemetry/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if telem == nil {
			http.Error(w, "telemetry unavailable", http.StatusServiceUnavailable)
			return
		}
		query, err := telemetryQueryFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		items, err := telem.QueryEvents(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "count": len(items)})
	})
	mux.HandleFunc("/api/debug/telemetry/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if telem == nil {
			http.Error(w, "telemetry unavailable", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"collector": telem.Config(),
				"config":    cfgManager.Snapshot().Debug.Telemetry,
			})
		case http.MethodPost:
			var update struct {
				Enabled           *bool   `json:"enabled"`
				HeavyEnabled      *bool   `json:"heavy_enabled"`
				HeavySampleEvery  *int    `json:"heavy_sample_every"`
				MetricSampleEvery *int    `json:"metric_sample_every"`
				MetricHistoryMax  *int    `json:"metric_history_max"`
				EventHistoryMax   *int    `json:"event_history_max"`
				RetentionSeconds  *int    `json:"retention_seconds"`
				PersistEnabled    *bool   `json:"persist_enabled"`
				PersistDir        *string `json:"persist_dir"`
				RotateMB          *int    `json:"rotate_mb"`
				KeepFiles         *int    `json:"keep_files"`
			}
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			next := cfgManager.Snapshot()
			cur := next.Debug.Telemetry
			if update.Enabled != nil {
				cur.Enabled = *update.Enabled
			}
			if update.HeavyEnabled != nil {
				cur.HeavyEnabled = *update.HeavyEnabled
			}
			if update.HeavySampleEvery != nil {
				cur.HeavySampleEvery = *update.HeavySampleEvery
			}
			if update.MetricSampleEvery != nil {
				cur.MetricSampleEvery = *update.MetricSampleEvery
			}
			if update.MetricHistoryMax != nil {
				cur.MetricHistoryMax = *update.MetricHistoryMax
			}
			if update.EventHistoryMax != nil {
				cur.EventHistoryMax = *update.EventHistoryMax
			}
			if update.RetentionSeconds != nil {
				cur.RetentionSeconds = *update.RetentionSeconds
			}
			if update.PersistEnabled != nil {
				cur.PersistEnabled = *update.PersistEnabled
			}
			if update.PersistDir != nil && *update.PersistDir != "" {
				cur.PersistDir = *update.PersistDir
			}
			if update.RotateMB != nil {
				cur.RotateMB = *update.RotateMB
			}
			if update.KeepFiles != nil {
				cur.KeepFiles = *update.KeepFiles
			}
			next.Debug.Telemetry = cur
			cfgManager.Replace(next)
			if err := config.Save(cfgPath, next); err != nil {
				log.Printf("telemetry config save failed: %v", err)
			}
			err := telem.Configure(telemetry.Config{
				Enabled:           cur.Enabled,
				HeavyEnabled:      cur.HeavyEnabled,
				HeavySampleEvery:  cur.HeavySampleEvery,
				MetricSampleEvery: cur.MetricSampleEvery,
				MetricHistoryMax:  cur.MetricHistoryMax,
				EventHistoryMax:   cur.EventHistoryMax,
				Retention:         time.Duration(cur.RetentionSeconds) * time.Second,
				PersistEnabled:    cur.PersistEnabled,
				PersistDir:        cur.PersistDir,
				RotateMB:          cur.RotateMB,
				KeepFiles:         cur.KeepFiles,
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "collector": telem.Config(), "config": cur})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func newHTTPServer(addr string, webRoot string, h *hub, cfgPath string, cfgManager *runtime.Manager, srcMgr *sourceManager, dspUpdates chan dspUpdate, gpuState *gpuStatus, recMgr *recorder.Manager, sigSnap *signalSnapshot, eventMu *sync.RWMutex, phaseSnap *phaseSnapshot, telem *telemetry.Collector) *http.Server {
	mux := http.NewServeMux()
	registerWSHandlers(mux, h, recMgr)
	registerAPIHandlers(mux, cfgPath, cfgManager, srcMgr, dspUpdates, gpuState, recMgr, sigSnap, eventMu, phaseSnap, telem)
	mux.Handle("/", http.FileServer(http.Dir(webRoot)))
	return &http.Server{Addr: addr, Handler: mux}
}

func telemetryQueryFromRequest(r *http.Request) (telemetry.Query, error) {
	q := r.URL.Query()
	var out telemetry.Query
	var err error
	if out.From, err = telemetry.ParseTimeQuery(q.Get("since")); err != nil {
		return out, errors.New("invalid since")
	}
	if out.To, err = telemetry.ParseTimeQuery(q.Get("until")); err != nil {
		return out, errors.New("invalid until")
	}
	if v := q.Get("limit"); v != "" {
		if parsed, parseErr := strconv.Atoi(v); parseErr == nil {
			out.Limit = parsed
		}
	}
	out.Name = q.Get("name")
	out.NamePrefix = q.Get("prefix")
	out.Level = q.Get("level")
	out.IncludePersisted = true
	if v := q.Get("include_persisted"); v != "" {
		if b, parseErr := strconv.ParseBool(v); parseErr == nil {
			out.IncludePersisted = b
		}
	}
	tags := telemetry.Tags{}
	for key, vals := range q {
		if len(vals) == 0 {
			continue
		}
		if strings.HasPrefix(key, "tag_") {
			tags[strings.TrimPrefix(key, "tag_")] = vals[0]
		}
	}
	for _, key := range []string{"signal_id", "session_id", "stage", "trace_id", "component"} {
		if v := q.Get(key); v != "" {
			tags[key] = v
		}
	}
	if len(tags) > 0 {
		out.Tags = tags
	}
	return out, nil
}

func shutdownServer(server *http.Server) {
	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()
	_ = server.Shutdown(ctxTimeout)
}

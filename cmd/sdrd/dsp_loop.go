package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/dsp"
	"sdr-wideband-suite/internal/logging"
	"sdr-wideband-suite/internal/pipeline"
	"sdr-wideband-suite/internal/recorder"
	"sdr-wideband-suite/internal/telemetry"
)

// SDRD_DEBUG_HZ overrides the configured spectrum_debug_hz at startup (dev knob;
// the config value stays live-adjustable via /api/config). Parsed once.
var (
	debugHzEnvVal int
	debugHzEnvSet bool
)

func init() {
	if raw := strings.TrimSpace(os.Getenv("SDRD_DEBUG_HZ")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			debugHzEnvVal, debugHzEnvSet = v, true
		}
	}
}

// debugEmitEveryN maps a target Debug rate (hz) and the frame rate (fps) to a
// frame stride. hz <= 0 or hz >= fps means every frame (full rate). The heavy
// spectrum Debug payload (#19) is only built+broadcast on those frames; the
// spectrum/waterfall/signals stream every frame regardless.
func debugEmitEveryN(hz, fps int) uint64 {
	if hz <= 0 || fps <= 0 || hz >= fps {
		return 1
	}
	n := fps / hz
	if n < 1 {
		n = 1
	}
	return uint64(n)
}

func runDSP(ctx context.Context, srcMgr *sourceManager, cfg config.Config, det *detector.Detector, window []float64, h *hub, eventFile *os.File, eventMu *sync.RWMutex, updates <-chan dspUpdate, gpuState *gpuStatus, rec *recorder.Manager, sigSnap *signalSnapshot, extractMgr *extractionManager, phaseSnap *phaseSnapshot, coll *telemetry.Collector) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("FATAL: runDSP goroutine panic: %v\n%s", r, debug.Stack())
		}
	}()
	rt := newDSPRuntime(cfg, det, window, gpuState, coll)
	ticker := time.NewTicker(cfg.FrameInterval())
	defer ticker.Stop()
	logTicker := time.NewTicker(5 * time.Second)
	defer logTicker.Stop()
	enc := json.NewEncoder(eventFile)
	dcBlocker := dsp.NewDCBlocker(0.995)
	state := &phaseState{}
	var frameID uint64
	prevDisplayed := map[int64]detector.Signal{}
	lastSourceDrops := uint64(0)
	lastSourceResets := uint64(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-logTicker.C:
			st := srcMgr.Stats()
			log.Printf("stats: buf=%d drop=%d reset=%d last=%dms", st.BufferSamples, st.Dropped, st.Resets, st.LastSampleAgoMs)
			if coll != nil {
				coll.SetGauge("source.buffer_samples", float64(st.BufferSamples), nil)
				coll.SetGauge("source.last_sample_ago_ms", float64(st.LastSampleAgoMs), nil)
				if st.Dropped > lastSourceDrops {
					coll.IncCounter("source.drop.count", float64(st.Dropped-lastSourceDrops), nil)
				}
				if st.Resets > lastSourceResets {
					coll.IncCounter("source.reset.count", float64(st.Resets-lastSourceResets), nil)
					coll.Event("source_reset", "warn", "source reset observed", nil, map[string]any{"resets": st.Resets})
				}
				lastSourceDrops = st.Dropped
				lastSourceResets = st.Resets
			}
		case upd := <-updates:
			rt.applyUpdate(upd, srcMgr, rec, gpuState)
			dcBlocker.Reset()
			ticker.Reset(rt.cfg.FrameInterval())
			if coll != nil {
				coll.IncCounter("dsp.update.apply", 1, nil)
			}
		case <-ticker.C:
			frameStart := time.Now()
			frameID++
			art, err := rt.captureSpectrum(srcMgr, rec, dcBlocker, gpuState)
			if err != nil {
				log.Printf("read IQ: %v", err)
				if strings.Contains(err.Error(), "timeout") {
					if err := srcMgr.Restart(rt.cfg); err != nil {
						log.Printf("restart failed: %v", err)
					}
				}
				continue
			}
			if !rt.gotSamples {
				log.Printf("received IQ samples")
				rt.gotSamples = true
			}
			logging.Debug("trace", "capture_done", "trace", frameID, "allIQ", len(art.allIQ), "detailIQ", len(art.detailIQ))
			if coll != nil {
				// No frame_id tag: a per-frame-unique tag gives every sample its own
				// metric series (cloneTags + new map + Sprintf every frame, unbounded
				// cardinality) and dominated the GC mark cost (#21). The distribution
				// aggregates across frames; metric-history timestamps preserve time order.
				coll.Observe("stage.capture.duration_ms", float64(time.Since(frameStart).Microseconds())/1000.0, nil)
			}
			survStart := time.Now()
			state.surveillance = rt.buildSurveillanceResult(art)
			if coll != nil {
				coll.Observe("stage.surveillance.duration_ms", float64(time.Since(survStart).Microseconds())/1000.0, nil)
			}
			refineStart := time.Now()
			state.refinement = rt.runRefinement(art, state.surveillance, extractMgr, rec)
			if coll != nil {
				coll.Observe("stage.refinement.duration_ms", float64(time.Since(refineStart).Microseconds())/1000.0, nil)
			}
			finished := state.surveillance.Finished
			thresholds := state.surveillance.Thresholds
			noiseFloor := state.surveillance.NoiseFloor
			var displaySignals []detector.Signal
			if len(art.detailIQ) > 0 {
				displaySignals = state.refinement.Result.Signals
				stableSignals := rt.det.StableSignals()
				streamSignals := displaySignals
				if len(stableSignals) > 0 {
					streamSignals = stableSignals
				}
				if rec != nil && len(art.allIQ) > 0 {
					if art.streamDropped {
						rt.streamOverlap = &streamIQOverlap{}
						for k := range rt.streamPhaseState {
							rt.streamPhaseState[k].phase = 0
						}
						resetStreamingOracleRunner()
						rec.ResetStreams()
						logging.Warn("gap", "iq_dropped", "msg", "buffer bloat caused extraction drop; overlap reset")
						if coll != nil {
							coll.IncCounter("capture.stream_reset", 1, nil)
							coll.Event("iq_dropped", "warn", "stream overlap reset after dropped IQ", nil, map[string]any{"frame_id": frameID})
						}
					}
					if rt.cfg.Recorder.DebugLiveAudio {
						log.Printf("LIVEAUDIO DSP: detailIQ=%d displaySignals=%d streamSignals=%d stableSignals=%d allIQ=%d", len(art.detailIQ), len(displaySignals), len(streamSignals), len(stableSignals), len(art.allIQ))
					}
					aqCfg := extractionConfig{firTaps: rt.cfg.Recorder.ExtractionTaps, bwMult: rt.cfg.Recorder.ExtractionBwMult}
					extractStart := time.Now()
					streamSnips, streamRates := extractForStreaming(extractMgr, art.allIQ, rt.cfg.SampleRate, rt.cfg.CenterHz, streamSignals, rt.streamPhaseState, rt.streamOverlap, aqCfg, rt.telemetry)
					if coll != nil {
						coll.Observe("stage.extract_stream.duration_ms", float64(time.Since(extractStart).Microseconds())/1000.0, nil)
						coll.SetGauge("stage.extract_stream.signals", float64(len(streamSignals)), nil)
						if coll.ShouldSampleHeavy() {
							for i := range streamSnips {
								if i >= len(streamSignals) {
									break
								}
								tags := telemetry.TagsFromPairs(
									"signal_id", fmt.Sprintf("%d", streamSignals[i].ID),
									"stage", "extract_stream",
								)
								coll.SetGauge("iq.stage.extract.length", float64(len(streamSnips[i])), tags)
								if len(streamSnips[i]) > 0 {
									observeIQStats(coll, "extract_stream", streamSnips[i], tags)
								}
							}
						}
					}
					nonEmpty := 0
					minLen := 0
					maxLen := 0
					for i := range streamSnips {
						l := len(streamSnips[i])
						if l == 0 {
							continue
						}
						nonEmpty++
						if minLen == 0 || l < minLen {
							minLen = l
						}
						if l > maxLen {
							maxLen = l
						}
					}
					logging.Debug("trace", "extract_stats", "trace", frameID, "signals", len(streamSignals), "nonempty", nonEmpty, "minLen", minLen, "maxLen", maxLen)
					items := make([]recorder.StreamFeedItem, 0, len(streamSignals))
					for j, ds := range streamSignals {
						className := "<nil>"
						if ds.Class != nil {
							className = string(ds.Class.ModType)
						}
						snipLen := 0
						if j < len(streamSnips) {
							snipLen = len(streamSnips[j])
						}
						if rt.cfg.Recorder.DebugLiveAudio {
							log.Printf("LIVEAUDIO DSP: streamSignal idx=%d id=%d center=%.3fMHz bw=%.0f class=%s snip=%d", j, ds.ID, ds.CenterHz/1e6, ds.BWHz, className, snipLen)
						}
						if ds.ID == 0 || ds.Class == nil {
							continue
						}
						if j >= len(streamSnips) || len(streamSnips[j]) == 0 {
							logging.Warn("gap", "snippet_empty", "signal", ds.ID)
							continue
						}
						snipRate := rt.cfg.SampleRate
						if j < len(streamRates) && streamRates[j] > 0 {
							snipRate = streamRates[j]
						}
						items = append(items, recorder.StreamFeedItem{Signal: ds, Snippet: streamSnips[j], SnipRate: snipRate})
					}
					if rt.cfg.Recorder.DebugLiveAudio {
						log.Printf("LIVEAUDIO DSP: feedItems=%d", len(items))
					}
					if len(items) > 0 {
						feedStart := time.Now()
						rec.FeedSnippets(items, frameID)
						if coll != nil {
							coll.Observe("stage.feed_enqueue.duration_ms", float64(time.Since(feedStart).Microseconds())/1000.0, nil)
							coll.SetGauge("stage.feed.items", float64(len(items)), nil)
						}
						logging.Debug("trace", "feed", "trace", frameID, "items", len(items), "signals", len(streamSignals), "allIQ", len(art.allIQ))
					} else {
						logging.Warn("gap", "feed_empty", "signals", len(streamSignals), "trace", frameID)
						if coll != nil {
							coll.IncCounter("stage.feed.empty", 1, nil)
						}
					}
				}
				rt.maintenance(displaySignals, rec)
			} else {
				displaySignals = rt.det.StableSignals()
			}
			if rec != nil && len(displaySignals) > 0 {
				runtimeInfo := rec.RuntimeInfoBySignalID()
				for i := range displaySignals {
					if info, ok := runtimeInfo[displaySignals[i].ID]; ok {
						displaySignals[i].DemodName = info.DemodName
						displaySignals[i].PlaybackMode = info.PlaybackMode
						displaySignals[i].StereoState = info.StereoState
					}
				}
			}
			state.arbitration = rt.arbitration
			state.presentation = state.surveillance.DisplayLevel
			if phaseSnap != nil {
				phaseSnap.Set(*state)
			}

			if sigSnap != nil {
				sigSnap.set(displaySignals)
			}
			if coll != nil {
				coll.SetGauge("signals.display.count", float64(len(displaySignals)), nil)
				current := make(map[int64]detector.Signal, len(displaySignals))
				for _, s := range displaySignals {
					current[s.ID] = s
					if _, ok := prevDisplayed[s.ID]; !ok {
						coll.Event("signal_create", "info", "signal entered display set", telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", s.ID)), map[string]any{
							"center_hz": s.CenterHz,
							"bw_hz":     s.BWHz,
						})
					}
				}
				for id, prev := range prevDisplayed {
					if _, ok := current[id]; !ok {
						coll.Event("signal_remove", "info", "signal left display set", telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", id)), map[string]any{
							"center_hz": prev.CenterHz,
						})
					}
				}
				prevDisplayed = current
			}
			eventMu.Lock()
			for _, ev := range finished {
				_ = enc.Encode(ev)
			}
			eventMu.Unlock()
			if rec != nil && len(finished) > 0 {
				evCopy := make([]detector.Event, len(finished))
				copy(evCopy, finished)
				rec.OnEvents(evCopy)
			}
			var debugInfo *SpectrumDebug
			// Cadence-limit the heavy Debug payload (#19): only build + broadcast it
			// at ~spectrum_debug_hz instead of every frame. SDRD_DEBUG_HZ overrides
			// the live-adjustable config value. Skipping it drops both the build
			// allocations below and the per-frame json.Marshal in the broadcast.
			debugHz := rt.cfg.Debug.SpectrumDebugHz
			if debugHzEnvSet {
				debugHz = debugHzEnvVal
			}
			emitDebug := frameID%debugEmitEveryN(debugHz, rt.cfg.FrameRate) == 0
			plan := state.refinement.Input.Plan
			var windowSummary *WindowSummary
			var windowStats *RefinementWindowStats
			var monitorSummary []pipeline.MonitorWindowStats
			var hasPlan, hasWindows bool
			if emitDebug {
				windowSummary = buildWindowSummary(plan, state.refinement.Input.Windows, state.surveillance.Candidates, state.refinement.Input.WorkItems, state.refinement.Result.Decisions)
				if windowSummary != nil {
					windowStats = windowSummary.Refinement
					monitorSummary = windowSummary.MonitorWindows
				}
				hasPlan = plan.TotalCandidates > 0 || plan.Budget > 0 || plan.DroppedBySNR > 0 || plan.DroppedByBudget > 0
				hasWindows = windowStats != nil && windowStats.Count > 0
			}
			if emitDebug && (len(thresholds) > 0 || len(displaySignals) > 0 || noiseFloor != 0 || hasPlan || hasWindows) {
				scoreDebug := make([]map[string]any, 0, len(displaySignals))
				for _, s := range displaySignals {
					if s.Class == nil || len(s.Class.Scores) == 0 {
						scoreDebug = append(scoreDebug, map[string]any{"center_hz": s.CenterHz, "class": nil})
						continue
					}
					scores := make(map[string]float64, len(s.Class.Scores))
					for k, v := range s.Class.Scores {
						scores[string(k)] = v
					}
					scoreDebug = append(scoreDebug, map[string]any{
						"center_hz":   s.CenterHz,
						"mod_type":    s.Class.ModType,
						"confidence":  s.Class.Confidence,
						"second_best": s.Class.SecondBest,
						"scores":      scores,
					})
				}
				debugInfo = &SpectrumDebug{Thresholds: thresholds, NoiseFloor: noiseFloor, Scores: scoreDebug}
				candidateSources := buildCandidateSourceSummary(state.surveillance.Candidates)
				candidateEvidence := buildCandidateEvidenceSummary(state.surveillance.Candidates)
				candidateEvidenceStates := buildCandidateEvidenceStateSummary(state.surveillance.Candidates)
				candidateWindows := buildCandidateWindowSummary(state.surveillance.Candidates, plan.MonitorWindows)
				if len(candidateSources) > 0 {
					debugInfo.CandidateSources = candidateSources
				}
				if len(candidateEvidence) > 0 {
					debugInfo.CandidateEvidence = candidateEvidence
				}
				if candidateEvidenceStates != nil {
					debugInfo.CandidateEvidenceStates = candidateEvidenceStates
				}
				if len(candidateWindows) > 0 {
					debugInfo.CandidateWindows = candidateWindows
				}
				if len(monitorSummary) > 0 {
					debugInfo.MonitorWindowStats = monitorSummary
				}
				if windowSummary != nil {
					debugInfo.WindowSummary = windowSummary
				}
				if hasPlan {
					debugInfo.RefinementPlan = &plan
				}
				if hasWindows {
					debugInfo.Windows = windowStats
				}
				refinementDebug := &RefinementDebug{}
				if hasPlan {
					refinementDebug.Plan = &plan
					refinementDebug.Request = &state.refinement.Input.Request
					refinementDebug.WorkItems = state.refinement.Input.WorkItems
				}
				if hasWindows {
					refinementDebug.Windows = windowStats
				}
				if len(monitorSummary) > 0 {
					refinementDebug.MonitorWindowStats = monitorSummary
				}
				if windowSummary != nil {
					refinementDebug.WindowSummary = windowSummary
				}
				refinementDebug.Arbitration = buildArbitrationSnapshot(state.refinement, state.arbitration)
				debugInfo.Refinement = refinementDebug
			}
			h.broadcast(SpectrumFrame{Timestamp: art.now.UnixMilli(), CenterHz: rt.cfg.CenterHz, SampleHz: rt.cfg.SampleRate, FFTSize: rt.cfg.FFTSize, Spectrum: art.surveillanceSpectrum, Signals: displaySignals, Debug: debugInfo})
			if coll != nil {
				coll.Observe("dsp.frame.duration_ms", float64(time.Since(frameStart).Microseconds())/1000.0, nil)
			}
		}
	}
}

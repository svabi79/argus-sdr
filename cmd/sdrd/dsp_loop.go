package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/dsp"
	"sdr-wideband-suite/internal/recorder"
)

func runDSP(ctx context.Context, srcMgr *sourceManager, cfg config.Config, det *detector.Detector, window []float64, h *hub, eventFile *os.File, eventMu *sync.RWMutex, updates <-chan dspUpdate, gpuState *gpuStatus, rec *recorder.Manager, sigSnap *signalSnapshot, extractMgr *extractionManager, phaseSnap *phaseSnapshot) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("FATAL: runDSP goroutine panic: %v\n%s", r, debug.Stack())
		}
	}()
	rt := newDSPRuntime(cfg, det, window, gpuState)
	ticker := time.NewTicker(cfg.FrameInterval())
	defer ticker.Stop()
	logTicker := time.NewTicker(5 * time.Second)
	defer logTicker.Stop()
	enc := json.NewEncoder(eventFile)
	dcBlocker := dsp.NewDCBlocker(0.995)
	state := &phaseState{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-logTicker.C:
			st := srcMgr.Stats()
			log.Printf("stats: buf=%d drop=%d reset=%d last=%dms", st.BufferSamples, st.Dropped, st.Resets, st.LastSampleAgoMs)
		case upd := <-updates:
			rt.applyUpdate(upd, srcMgr, rec, gpuState)
			dcBlocker.Reset()
			ticker.Reset(rt.cfg.FrameInterval())
		case <-ticker.C:
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
			state.surveillance = rt.buildSurveillanceResult(art)
			state.refinement = rt.runRefinement(art, state.surveillance, extractMgr, rec)
			finished := state.surveillance.Finished
			thresholds := state.surveillance.Thresholds
			noiseFloor := state.surveillance.NoiseFloor
			var displaySignals []detector.Signal
			if len(art.detailIQ) > 0 {
				displaySignals = state.refinement.Result.Signals
				if rec != nil && len(displaySignals) > 0 && len(art.allIQ) > 0 {
					aqCfg := extractionConfig{firTaps: rt.cfg.Recorder.ExtractionTaps, bwMult: rt.cfg.Recorder.ExtractionBwMult}
					streamSnips, streamRates := extractForStreaming(extractMgr, art.allIQ, rt.cfg.SampleRate, rt.cfg.CenterHz, displaySignals, rt.streamPhaseState, rt.streamOverlap, aqCfg)
					items := make([]recorder.StreamFeedItem, 0, len(displaySignals))
					for j, ds := range displaySignals {
						if ds.ID == 0 || ds.Class == nil {
							continue
						}
						if j >= len(streamSnips) || len(streamSnips[j]) == 0 {
							continue
						}
						snipRate := rt.cfg.SampleRate
						if j < len(streamRates) && streamRates[j] > 0 {
							snipRate = streamRates[j]
						}
						items = append(items, recorder.StreamFeedItem{Signal: ds, Snippet: streamSnips[j], SnipRate: snipRate})
					}
					if len(items) > 0 {
						rec.FeedSnippets(items)
					}
				}
				rt.maintenance(displaySignals, rec)
			} else {
				displaySignals = rt.det.StableSignals()
			}
			state.arbitration = rt.arbitration
			state.presentation = state.surveillance.DisplayLevel
			if phaseSnap != nil {
				phaseSnap.Set(*state)
			}

			if sigSnap != nil {
				sigSnap.set(displaySignals)
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
			plan := state.refinement.Input.Plan
			windowStats := buildWindowStats(state.refinement.Input.Windows)
			hasPlan := plan.TotalCandidates > 0 || plan.Budget > 0 || plan.DroppedBySNR > 0 || plan.DroppedByBudget > 0
			hasWindows := windowStats != nil && windowStats.Count > 0
			if len(thresholds) > 0 || len(displaySignals) > 0 || noiseFloor != 0 || hasPlan || hasWindows {
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
				refinementDebug.Arbitration = buildArbitrationSnapshot(state.refinement, state.arbitration)
				debugInfo.Refinement = refinementDebug
			}
			h.broadcast(SpectrumFrame{Timestamp: art.now.UnixMilli(), CenterHz: rt.cfg.CenterHz, SampleHz: rt.cfg.SampleRate, FFTSize: rt.cfg.FFTSize, Spectrum: art.surveillanceSpectrum, Signals: displaySignals, Debug: debugInfo})
		}
	}
}

package main

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"sdr-visual-suite/internal/classifier"
	"sdr-visual-suite/internal/config"
	"sdr-visual-suite/internal/detector"
	"sdr-visual-suite/internal/dsp"
	fftutil "sdr-visual-suite/internal/fft"
	"sdr-visual-suite/internal/fft/gpufft"
	"sdr-visual-suite/internal/recorder"
)

func runDSP(ctx context.Context, srcMgr *sourceManager, cfg config.Config, det *detector.Detector, window []float64, h *hub, eventFile *os.File, eventMu *sync.RWMutex, updates <-chan dspUpdate, gpuState *gpuStatus, rec *recorder.Manager, sigSnap *signalSnapshot, extractMgr *extractionManager) {
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
			if len(iq) > 0 {
				snips := extractSignalIQBatch(extractMgr, iq, cfg.SampleRate, cfg.CenterHz, signals)
				for i := range signals {
					var snip []complex64
					if i < len(snips) {
						snip = snips[i]
					}
					cls := classifier.Classify(classifier.SignalInput{FirstBin: signals[i].FirstBin, LastBin: signals[i].LastBin, SNRDb: signals[i].SNRDb, CenterHz: signals[i].CenterHz}, spectrum, cfg.SampleRate, cfg.FFTSize, snip, classifier.ClassifierMode(cfg.ClassifierMode))
					signals[i].Class = cls
					if cls != nil && snip != nil && len(snip) > 256 {
						pll := classifier.EstimateExactFrequency(snip, cfg.SampleRate, signals[i].CenterHz, cls.ModType)
						cls.PLL = &pll
						signals[i].PLL = &pll
						if pll.Locked {
							signals[i].CenterHz = pll.ExactHz
						}
					}
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
				rec.OnEvents(evCopy)
			}
			var debugInfo *SpectrumDebug
			if len(thresholds) > 0 || len(signals) > 0 || noiseFloor != 0 {
				scoreDebug := make([]map[string]any, 0, len(signals))
				for _, s := range signals {
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
			}
			h.broadcast(SpectrumFrame{Timestamp: now.UnixMilli(), CenterHz: cfg.CenterHz, SampleHz: cfg.SampleRate, FFTSize: cfg.FFTSize, Spectrum: spectrum, Signals: signals, Debug: debugInfo})
		}
	}
}

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
	"sync/atomic"
	"time"

	"sdr-visual-suite/internal/classifier"
	"sdr-visual-suite/internal/config"
	"sdr-visual-suite/internal/demod"
	"sdr-visual-suite/internal/detector"
	"sdr-visual-suite/internal/dsp"
	fftutil "sdr-visual-suite/internal/fft"
	"sdr-visual-suite/internal/fft/gpufft"
	"sdr-visual-suite/internal/rds"
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

	// Persistent RDS decoders per signal — async ring-buffer based
	type rdsState struct {
		dec        rds.Decoder
		result     rds.Result
		lastDecode time.Time
		busy       int32 // atomic: 1 = goroutine running
		mu         sync.Mutex
	}
	rdsMap := map[int64]*rdsState{}
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
			// Read all available IQ data — not just one FFT block.
			// This ensures the ring buffer captures 100% of IQ for recording/demod.
			available := cfg.FFTSize
			st := srcMgr.Stats()
			if st.BufferSamples > cfg.FFTSize {
				// Round down to multiple of FFTSize for clean processing
				available = (st.BufferSamples / cfg.FFTSize) * cfg.FFTSize
				if available < cfg.FFTSize {
					available = cfg.FFTSize
				}
			}
			allIQ, err := srcMgr.ReadIQ(available)
			if err != nil {
				log.Printf("read IQ: %v", err)
				if strings.Contains(err.Error(), "timeout") {
					if err := srcMgr.Restart(cfg); err != nil {
						log.Printf("restart failed: %v", err)
					}
				}
				continue
			}
			// Ingest ALL IQ data into the ring buffer for recording
			if rec != nil {
				rec.Ingest(time.Now(), allIQ)
			}
			// Use only the last FFT block for spectrum display
			iq := allIQ
			if len(allIQ) > cfg.FFTSize {
				iq = allIQ[len(allIQ)-cfg.FFTSize:]
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
				snips, snipRates := extractSignalIQBatch(extractMgr, iq, cfg.SampleRate, cfg.CenterHz, signals)
				for i := range signals {
					var snip []complex64
					if i < len(snips) {
						snip = snips[i]
					}
					// Determine actual sample rate of the extracted snippet
					snipRate := cfg.SampleRate
					if i < len(snipRates) && snipRates[i] > 0 {
						snipRate = snipRates[i]
					}
					cls := classifier.Classify(classifier.SignalInput{FirstBin: signals[i].FirstBin, LastBin: signals[i].LastBin, SNRDb: signals[i].SNRDb, CenterHz: signals[i].CenterHz, BWHz: signals[i].BWHz}, spectrum, cfg.SampleRate, cfg.FFTSize, snip, classifier.ClassifierMode(cfg.ClassifierMode))
					signals[i].Class = cls
					if cls != nil && snip != nil && len(snip) > 256 {
						pll := classifier.EstimateExactFrequency(snip, snipRate, signals[i].CenterHz, cls.ModType)
						cls.PLL = &pll
						signals[i].PLL = &pll
						// Upgrade WFM → WFM_STEREO if stereo pilot detected
						if cls.ModType == classifier.ClassWFM && pll.Stereo {
							cls.ModType = classifier.ClassWFMStereo
						}
						// RDS decode for WFM — async, uses ring buffer for continuous IQ
						if (cls.ModType == classifier.ClassWFM || cls.ModType == classifier.ClassWFMStereo) && rec != nil {
							key := int64(math.Round(signals[i].CenterHz / 500000))
							st := rdsMap[key]
							if st == nil {
								st = &rdsState{}
								rdsMap[key] = st
							}
							// Launch async decode every 4 seconds, skip if previous still running
							if now.Sub(st.lastDecode) >= 4*time.Second && atomic.LoadInt32(&st.busy) == 0 {
								st.lastDecode = now
								atomic.StoreInt32(&st.busy, 1)
								go func(st *rdsState, sigHz float64) {
									defer atomic.StoreInt32(&st.busy, 0)
									ringIQ, ringSR, ringCenter := rec.SliceRecent(4.0)
									if len(ringIQ) < ringSR || ringSR <= 0 {
										return
									}
									// Shift FM station to center
									offset := sigHz - ringCenter
									shifted := dsp.FreqShift(ringIQ, ringSR, offset)

									// Two-stage decimation to ~250kHz with proper anti-alias
									// Stage 1: 4MHz → 1MHz (decim 4), LP at 400kHz
									decim1 := ringSR / 1000000
									if decim1 < 1 {
										decim1 = 1
									}
									lp1 := dsp.LowpassFIR(float64(ringSR/decim1)/2.0*0.8, ringSR, 51)
									f1 := dsp.ApplyFIR(shifted, lp1)
									d1 := dsp.Decimate(f1, decim1)
									rate1 := ringSR / decim1

									// Stage 2: 1MHz → 250kHz (decim 4), LP at 100kHz
									decim2 := rate1 / 250000
									if decim2 < 1 {
										decim2 = 1
									}
									lp2 := dsp.LowpassFIR(float64(rate1/decim2)/2.0*0.8, rate1, 101)
									f2 := dsp.ApplyFIR(d1, lp2)
									decimated := dsp.Decimate(f2, decim2)
									actualRate := rate1 / decim2

									// RDS baseband extraction on the clean decimated block
									rdsBase := demod.RDSBasebandComplex(decimated, actualRate)
									if len(rdsBase.Samples) == 0 {
										return
									}
									st.mu.Lock()
									result := st.dec.Decode(rdsBase.Samples, rdsBase.SampleRate)
									diag := st.dec.LastDiag
									if result.PS != "" {
										st.result = result
									}
									st.mu.Unlock()
									log.Printf("RDS TRACE: ring decode freq=%.1fMHz decIQ=%d decSR=%d bbLen=%d bbRate=%d PI=%04X PS=%q %s",
										sigHz/1e6, len(decimated), actualRate, len(rdsBase.Samples), rdsBase.SampleRate,
										result.PI, result.PS, diag)
									if result.PS != "" {
										log.Printf("RDS decoded: PI=%04X PS=%q RT=%q freq=%.1fMHz", result.PI, result.PS, result.RT, sigHz/1e6)
									}
								}(st, signals[i].CenterHz)
							}
							// Read last known result (lock-free for display)
							st.mu.Lock()
							ps := st.result.PS
							st.mu.Unlock()
							if ps != "" {
								pll.RDSStation = strings.TrimSpace(ps)
								cls.PLL = &pll
								signals[i].PLL = &pll
							}
						}
					}
				}
				det.UpdateClasses(signals)

				// Cleanup RDS accumulators for signals that no longer exist
				if len(rdsMap) > 0 {
					activeIDs := make(map[int64]bool, len(signals))
					for _, s := range signals {
						activeIDs[int64(math.Round(s.CenterHz / 500000))] = true
					}
					for id := range rdsMap {
						if !activeIDs[id] {
							delete(rdsMap, id)
						}
					}
				}
			}
			// Use smoothed active events for frontend display (stable markers)
			displaySignals := det.StableSignals()
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
			if len(thresholds) > 0 || len(displaySignals) > 0 || noiseFloor != 0 {
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
			}
			h.broadcast(SpectrumFrame{Timestamp: now.UnixMilli(), CenterHz: cfg.CenterHz, SampleHz: cfg.SampleRate, FFTSize: cfg.FFTSize, Spectrum: spectrum, Signals: displaySignals, Debug: debugInfo})
		}
	}
}

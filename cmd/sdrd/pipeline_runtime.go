package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"sdr-wideband-suite/internal/classifier"
	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/demod"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/dsp"
	"sdr-wideband-suite/internal/logging"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/fft/gpufft"
	"sdr-wideband-suite/internal/pipeline"
	"sdr-wideband-suite/internal/rds"
	"sdr-wideband-suite/internal/recorder"
	"sdr-wideband-suite/internal/telemetry"
)

type rdsState struct {
	dec        rds.Decoder
	result     rds.Result
	lastDecode time.Time
	lastSeen   time.Time
	busy       int32
	mu         sync.Mutex
	iqBuf      []complex64 // reused ring-slice buffer (only touched by the busy goroutine)
	stereo     bool        // 19 kHz pilot present in the last long-window decode (OI-24)
}

var forceFixedStreamReadSamples = func() int {
	raw := strings.TrimSpace(os.Getenv("SDR_FORCE_FIXED_STREAM_READ_SAMPLES"))
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}()

type dspRuntime struct {
	cfg              config.Config
	det              *detector.Detector
	derivedDetectors map[string]*derivedDetector
	nextDerivedBase  int64
	window           []float64
	plan             *fftutil.CmplxPlan
	detailWindow     []float64
	detailPlan       *fftutil.CmplxPlan
	detailFFT        int
	survWindows      map[int][]float64
	survPlans        map[int]*fftutil.CmplxPlan
	survFIR          map[int][]float64
	dcEnabled        bool
	iqEnabled        bool
	useGPU           bool
	gpuEngine        *gpufft.Engine
	rdsMap           map[int64]*rdsState
	stereoHold       map[int64]time.Time // sticky stereo-lock per signal ID
	streamPhaseState map[int64]*streamExtractState
	streamOverlap    *streamIQOverlap
	arbiter          *pipeline.Arbiter
	arbitration      pipeline.ArbitrationState
	gotSamples       bool
	telemetry        *telemetry.Collector
	lastAllIQTail    []complex64
}

type spectrumArtifacts struct {
	allIQ                []complex64
	streamDropped        bool
	surveillanceIQ       []complex64
	detailIQ             []complex64
	surveillanceSpectrum []float64
	surveillanceSpectra  []pipeline.SurveillanceLevelSpectrum
	surveillancePlan     surveillancePlan
	detailSpectrum       []float64
	finished             []detector.Event
	detected             []detector.Signal
	thresholds           []float64
	noiseFloor           float64
	now                  time.Time
}

type derivedDetector struct {
	det        *detector.Detector
	sampleRate int
	fftSize    int
	idBase     int64
}

type surveillanceLevelSpec struct {
	Level    pipeline.AnalysisLevel
	Decim    int
	AllowGPU bool
}

type surveillancePlan struct {
	Primary         pipeline.AnalysisLevel
	Levels          []pipeline.AnalysisLevel
	LevelSet        pipeline.SurveillanceLevelSet
	Presentation    pipeline.AnalysisLevel
	Context         pipeline.AnalysisContext
	DetectionPolicy pipeline.SurveillanceDetectionPolicy
	Specs           []surveillanceLevelSpec
}

const derivedIDBlock = int64(1_000_000_000)

func newDSPRuntime(cfg config.Config, det *detector.Detector, window []float64, gpuState *gpuStatus, coll *telemetry.Collector) *dspRuntime {
	detailFFT := cfg.Refinement.DetailFFTSize
	if detailFFT <= 0 {
		detailFFT = cfg.FFTSize
	}
	rt := &dspRuntime{
		cfg:              cfg,
		det:              det,
		derivedDetectors: map[string]*derivedDetector{},
		nextDerivedBase:  -derivedIDBlock,
		window:           window,
		plan:             fftutil.NewCmplxPlan(cfg.FFTSize),
		detailWindow:     fftutil.Hann(detailFFT),
		detailPlan:       fftutil.NewCmplxPlan(detailFFT),
		detailFFT:        detailFFT,
		survWindows:      map[int][]float64{},
		survPlans:        map[int]*fftutil.CmplxPlan{},
		survFIR:          map[int][]float64{},
		dcEnabled:        cfg.DCBlock,
		iqEnabled:        cfg.IQBalance,
		useGPU:           cfg.UseGPUFFT,
		rdsMap:           map[int64]*rdsState{},
		streamPhaseState: map[int64]*streamExtractState{},
		streamOverlap:    &streamIQOverlap{},
		arbiter:          pipeline.NewArbiter(),
		telemetry:        coll,
	}
	if rt.useGPU && gpuState != nil {
		snap := gpuState.snapshot()
		if snap.Available {
			if eng, err := gpufft.New(cfg.FFTSize); err == nil {
				rt.gpuEngine = eng
				gpuState.set(true, nil)
			} else {
				gpuState.set(false, err)
				rt.useGPU = false
			}
		}
	}
	return rt
}

func (rt *dspRuntime) applyUpdate(upd dspUpdate, srcMgr *sourceManager, rec *recorder.Manager, gpuState *gpuStatus) {
	prevFFT := rt.cfg.FFTSize
	prevSampleRate := rt.cfg.SampleRate
	prevUseGPU := rt.useGPU
	prevDetailFFT := rt.detailFFT
	rt.cfg = upd.cfg
	if rec != nil {
		rec.Update(rt.cfg.SampleRate, rt.cfg.FFTSize, recorder.Policy{
			Enabled:          rt.cfg.Recorder.Enabled,
			MinSNRDb:         rt.cfg.Recorder.MinSNRDb,
			MinDuration:      mustParseDuration(rt.cfg.Recorder.MinDuration, 1*time.Second),
			MaxDuration:      mustParseDuration(rt.cfg.Recorder.MaxDuration, 300*time.Second),
			PrerollMs:        rt.cfg.Recorder.PrerollMs,
			RecordIQ:         rt.cfg.Recorder.RecordIQ,
			RecordAudio:      rt.cfg.Recorder.RecordAudio,
			AutoDemod:        rt.cfg.Recorder.AutoDemod,
			AutoDecode:       rt.cfg.Recorder.AutoDecode,
			MaxDiskMB:        rt.cfg.Recorder.MaxDiskMB,
			OutputDir:        rt.cfg.Recorder.OutputDir,
			ClassFilter:      rt.cfg.Recorder.ClassFilter,
			RingSeconds:      rt.cfg.Recorder.RingSeconds,
			DeemphasisUs:     rt.cfg.Recorder.DeemphasisUs,
			ExtractionTaps:   rt.cfg.Recorder.ExtractionTaps,
			ExtractionBwMult: rt.cfg.Recorder.ExtractionBwMult,
		}, rt.cfg.CenterHz, buildDecoderMap(rt.cfg))
	}
	if upd.det != nil {
		rt.det = upd.det
	}
	if upd.window != nil {
		rt.window = upd.window
		rt.plan = fftutil.NewCmplxPlan(rt.cfg.FFTSize)
	}
	detailFFT := rt.cfg.Refinement.DetailFFTSize
	if detailFFT <= 0 {
		detailFFT = rt.cfg.FFTSize
	}
	if detailFFT != prevDetailFFT {
		rt.detailFFT = detailFFT
		rt.detailWindow = fftutil.Hann(detailFFT)
		rt.detailPlan = fftutil.NewCmplxPlan(detailFFT)
	}
	if prevSampleRate != rt.cfg.SampleRate {
		rt.survFIR = map[int][]float64{}
	}
	if prevFFT != rt.cfg.FFTSize {
		rt.survWindows = map[int][]float64{}
		rt.survPlans = map[int]*fftutil.CmplxPlan{}
	}
	if upd.det != nil || prevSampleRate != rt.cfg.SampleRate || prevFFT != rt.cfg.FFTSize {
		rt.derivedDetectors = map[string]*derivedDetector{}
		rt.nextDerivedBase = -derivedIDBlock
	}
	rt.dcEnabled = upd.dcBlock
	rt.iqEnabled = upd.iqBalance
	if rt.cfg.FFTSize != prevFFT || rt.cfg.UseGPUFFT != prevUseGPU {
		srcMgr.Flush()
		rt.gotSamples = false
		if rt.gpuEngine != nil {
			rt.gpuEngine.Close()
			rt.gpuEngine = nil
		}
		rt.useGPU = rt.cfg.UseGPUFFT
		if rt.useGPU && gpuState != nil {
			snap := gpuState.snapshot()
			if snap.Available {
				if eng, err := gpufft.New(rt.cfg.FFTSize); err == nil {
					rt.gpuEngine = eng
					gpuState.set(true, nil)
				} else {
					gpuState.set(false, err)
					rt.useGPU = false
				}
			} else {
				gpuState.set(false, nil)
				rt.useGPU = false
			}
		} else if gpuState != nil {
			gpuState.set(false, nil)
		}
	}
	if rt.telemetry != nil {
		rt.telemetry.Event("dsp_config_update", "info", "dsp runtime configuration updated", nil, map[string]any{
			"fft_size":     rt.cfg.FFTSize,
			"sample_rate":  rt.cfg.SampleRate,
			"use_gpu_fft":  rt.cfg.UseGPUFFT,
			"detail_fft":   rt.detailFFT,
			"surv_strategy": rt.cfg.Surveillance.Strategy,
		})
	}
}

func (rt *dspRuntime) spectrumFromIQ(iq []complex64, gpuState *gpuStatus) []float64 {
	return rt.spectrumFromIQWithPlan(iq, rt.window, rt.plan, gpuState, true)
}

// surveillanceSpectrumFromIQ computes the surveillance spectrum, using Welch
// averaging over the longer allIQ buffer when configured (R2.4) to cut noise
// variance. Falls back to the single-FFT (GPU-capable) path otherwise.
func (rt *dspRuntime) surveillanceSpectrumFromIQ(allIQ, survIQ []complex64, gpuState *gpuStatus) []float64 {
	segs := rt.cfg.Surveillance.WelchSegments
	n := rt.cfg.FFTSize
	if segs > 1 && n > 0 {
		need := n + (segs-1)*n/2 // segs segments at 50% overlap
		if len(allIQ) >= need {
			src := append([]complex64(nil), allIQ[len(allIQ)-need:]...)
			if rt.iqEnabled {
				dsp.IQBalance(src) // allIQ is already DC-blocked in place
			}
			if psd := fftutil.WelchPSD(src, n, 0.5, rt.window); len(psd) == n {
				return psd
			}
		}
	}
	return rt.spectrumFromIQ(survIQ, gpuState)
}

func (rt *dspRuntime) spectrumFromIQWithPlan(iq []complex64, window []float64, plan *fftutil.CmplxPlan, gpuState *gpuStatus, allowGPU bool) []float64 {
	if len(iq) == 0 {
		return nil
	}
	if allowGPU && rt.useGPU && rt.gpuEngine != nil {
		gpuBuf := make([]complex64, len(iq))
		if len(window) == len(iq) {
			for i := 0; i < len(iq); i++ {
				v := iq[i]
				w := float32(window[i])
				gpuBuf[i] = complex(real(v)*w, imag(v)*w)
			}
		} else {
			copy(gpuBuf, iq)
		}
		out, err := rt.gpuEngine.Exec(gpuBuf)
		if err != nil {
			if gpuState != nil {
				gpuState.set(false, err)
			}
			rt.useGPU = false
			return fftutil.SpectrumWithPlan(gpuBuf, nil, plan)
		}
		return fftutil.SpectrumFromFFT(out)
	}
	return fftutil.SpectrumWithPlan(iq, window, plan)
}

func (rt *dspRuntime) windowForFFT(fftSize int) []float64 {
	if fftSize <= 0 {
		return nil
	}
	if fftSize == rt.cfg.FFTSize {
		return rt.window
	}
	if rt.survWindows == nil {
		rt.survWindows = map[int][]float64{}
	}
	if window, ok := rt.survWindows[fftSize]; ok {
		return window
	}
	window := fftutil.Hann(fftSize)
	rt.survWindows[fftSize] = window
	return window
}

func (rt *dspRuntime) planForFFT(fftSize int) *fftutil.CmplxPlan {
	if fftSize <= 0 {
		return nil
	}
	if fftSize == rt.cfg.FFTSize {
		return rt.plan
	}
	if rt.survPlans == nil {
		rt.survPlans = map[int]*fftutil.CmplxPlan{}
	}
	if plan, ok := rt.survPlans[fftSize]; ok {
		return plan
	}
	plan := fftutil.NewCmplxPlan(fftSize)
	rt.survPlans[fftSize] = plan
	return plan
}

func (rt *dspRuntime) spectrumForLevel(iq []complex64, fftSize int, gpuState *gpuStatus, allowGPU bool) []float64 {
	if len(iq) == 0 || fftSize <= 0 {
		return nil
	}
	if len(iq) > fftSize {
		iq = iq[len(iq)-fftSize:]
	}
	window := rt.windowForFFT(fftSize)
	plan := rt.planForFFT(fftSize)
	return rt.spectrumFromIQWithPlan(iq, window, plan, gpuState, allowGPU)
}

func sanitizeSpectrum(spectrum []float64) {
	for i := range spectrum {
		if math.IsNaN(spectrum[i]) || math.IsInf(spectrum[i], 0) {
			spectrum[i] = -200
		}
	}
}

func (rt *dspRuntime) decimationTaps(factor int) []float64 {
	if factor <= 1 {
		return nil
	}
	if rt.survFIR == nil {
		rt.survFIR = map[int][]float64{}
	}
	if taps, ok := rt.survFIR[factor]; ok {
		return taps
	}
	cutoff := float64(rt.cfg.SampleRate/factor) * 0.5 * 0.8
	taps := dsp.LowpassFIR(cutoff, rt.cfg.SampleRate, 101)
	rt.survFIR[factor] = taps
	return taps
}

func (rt *dspRuntime) decimateSurveillanceIQ(iq []complex64, factor int) []complex64 {
	if factor <= 1 {
		return iq
	}
	taps := rt.decimationTaps(factor)
	if len(taps) == 0 {
		return dsp.Decimate(iq, factor)
	}
	filtered := dsp.ApplyFIR(iq, taps)
	return dsp.Decimate(filtered, factor)
}

func meanMagComplex(samples []complex64) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, v := range samples {
		sum += math.Hypot(float64(real(v)), float64(imag(v)))
	}
	return sum / float64(len(samples))
}

func phaseStepAbs(a, b complex64) float64 {
	num := float64(real(a))*float64(imag(b)) - float64(imag(a))*float64(real(b))
	den := float64(real(a))*float64(real(b)) + float64(imag(a))*float64(imag(b))
	return math.Abs(math.Atan2(num, den))
}

func boundaryMetrics(prevTail []complex64, curr []complex64, window int) (float64, float64, float64, int) {
	if len(curr) == 0 {
		return 0, 0, 0, 0
	}
	if window <= 0 {
		window = 16
	}
	headN := window
	if len(curr) < headN {
		headN = len(curr)
	}
	headMean := meanMagComplex(curr[:headN])
	if len(prevTail) == 0 {
		return headMean, 0, 0, headN
	}
	tailN := window
	if len(prevTail) < tailN {
		tailN = len(prevTail)
	}
	tailMean := meanMagComplex(prevTail[len(prevTail)-tailN:])
	deltaMag := math.Abs(headMean - tailMean)
	phaseJump := phaseStepAbs(prevTail[len(prevTail)-1], curr[0])
	score := deltaMag + phaseJump
	return headMean, tailMean, score, headN
}

func tailWindowComplex(src []complex64, n int) []complex64 {
	if n <= 0 || len(src) == 0 {
		return nil
	}
	if len(src) <= n {
		out := make([]complex64, len(src))
		copy(out, src)
		return out
	}
	out := make([]complex64, n)
	copy(out, src[len(src)-n:])
	return out
}

func (rt *dspRuntime) captureSpectrum(srcMgr *sourceManager, rec *recorder.Manager, dcBlocker *dsp.DCBlocker, gpuState *gpuStatus) (*spectrumArtifacts, error) {
	start := time.Now()
	required := rt.cfg.FFTSize
	if rt.detailFFT > required {
		required = rt.detailFFT
	}
	available := required
	st := srcMgr.Stats()
	if rt.telemetry != nil {
		rt.telemetry.SetGauge("source.buffer_samples", float64(st.BufferSamples), nil)
		rt.telemetry.SetGauge("source.last_sample_ago_ms", float64(st.LastSampleAgoMs), nil)
		rt.telemetry.SetGauge("source.dropped", float64(st.Dropped), nil)
		rt.telemetry.SetGauge("source.resets", float64(st.Resets), nil)
	}
	if forceFixedStreamReadSamples > 0 {
		available = forceFixedStreamReadSamples
		if available < required {
			available = required
		}
		available = (available / required) * required
		if available < required {
			available = required
		}
		logging.Warn("boundary", "fixed_stream_read_samples", "configured", forceFixedStreamReadSamples, "effective", available, "required", required)
	} else if st.BufferSamples > required {
		available = (st.BufferSamples / required) * required
		if available < required {
			available = required
		}
	}
	logging.Debug("capture", "read_iq", "required", required, "available", available, "buf", st.BufferSamples, "reset", st.Resets, "drop", st.Dropped)
	readStart := time.Now()
	allIQ, err := srcMgr.ReadIQ(available)
	if err != nil {
		if rt.telemetry != nil {
			rt.telemetry.IncCounter("capture.read.error", 1, nil)
		}
		return nil, err
	}
	if rt.telemetry != nil {
		rt.telemetry.Observe("capture.read.duration_ms", float64(time.Since(readStart).Microseconds())/1000.0, nil)
		rt.telemetry.Observe("capture.read.samples", float64(len(allIQ)), nil)
	}
	if rec != nil {
		ingestStart := time.Now()
		rec.Ingest(time.Now(), allIQ)
		if rt.telemetry != nil {
			rt.telemetry.Observe("capture.ingest.duration_ms", float64(time.Since(ingestStart).Microseconds())/1000.0, nil)
		}
	}
	// Cap allIQ for downstream extraction to prevent buffer bloat.
	// Without this cap, buffer accumulation during processing stalls causes
	// increasingly large reads → longer extraction → more accumulation
	// (positive feedback loop). For audio streaming this creates >150ms
	// feed gaps that produce audible clicks.
	// The ring buffer (Ingest above) gets the full data; only extraction is capped.
	maxStreamSamples := rt.cfg.SampleRate / rt.cfg.FrameRate * 2
	if maxStreamSamples < required {
		maxStreamSamples = required
	}
	maxStreamSamples = (maxStreamSamples / required) * required
	streamDropped := false
	if len(allIQ) > maxStreamSamples {
		allIQ = allIQ[len(allIQ)-maxStreamSamples:]
		streamDropped = true
		if rt.telemetry != nil {
			rt.telemetry.IncCounter("capture.stream_drop.count", 1, nil)
			rt.telemetry.Event("iq_dropped", "warn", "capture IQ dropped before extraction", nil, map[string]any{
				"max_stream_samples": maxStreamSamples,
				"required":           required,
			})
		}
	}
	logging.Debug("capture", "iq_len", "len", len(allIQ), "surv_fft", rt.cfg.FFTSize, "detail_fft", rt.detailFFT)
	survIQ := allIQ
	if len(allIQ) > rt.cfg.FFTSize {
		survIQ = allIQ[len(allIQ)-rt.cfg.FFTSize:]
	}
	detailIQ := survIQ
	if rt.detailFFT > 0 && len(allIQ) >= rt.detailFFT {
		detailIQ = allIQ[len(allIQ)-rt.detailFFT:]
	}
	if rt.dcEnabled {
		dcBlocker.Apply(allIQ)
		if rt.telemetry != nil {
			rt.telemetry.IncCounter("dsp.dc_block.apply", 1, nil)
		}
	}
	if rt.iqEnabled {
		// IQBalance must NOT modify allIQ in-place: allIQ goes to the extraction
		// pipeline and any in-place modification creates a phase/amplitude
		// discontinuity at the survIQ boundary (len-FFTSize) that the polyphase
		// extractor then sees as paired click artifacts in the FM discriminator.
		detailIsSurv := sameIQBuffer(detailIQ, survIQ)
		survIQ = append([]complex64(nil), survIQ...)
		dsp.IQBalance(survIQ)
		if detailIsSurv {
			detailIQ = survIQ
		} else {
			detailIQ = append([]complex64(nil), detailIQ...)
			dsp.IQBalance(detailIQ)
		}
	}
	if rt.telemetry != nil {
		rt.telemetry.SetGauge("iq.stage.all.length", float64(len(allIQ)), nil)
		rt.telemetry.SetGauge("iq.stage.surveillance.length", float64(len(survIQ)), nil)
		rt.telemetry.SetGauge("iq.stage.detail.length", float64(len(detailIQ)), nil)
		rt.telemetry.Observe("capture.total.duration_ms", float64(time.Since(start).Microseconds())/1000.0, nil)

		headMean, tailMean, boundaryScore, boundaryWindow := boundaryMetrics(rt.lastAllIQTail, allIQ, 32)
		rt.telemetry.SetGauge("iq.boundary.all.head_mean_mag", headMean, nil)
		rt.telemetry.SetGauge("iq.boundary.all.prev_tail_mean_mag", tailMean, nil)
		rt.telemetry.Observe("iq.boundary.all.discontinuity_score", boundaryScore, nil)
		if len(rt.lastAllIQTail) > 0 && len(allIQ) > 0 {
			deltaMag := math.Abs(math.Hypot(float64(real(allIQ[0])), float64(imag(allIQ[0]))) - math.Hypot(float64(real(rt.lastAllIQTail[len(rt.lastAllIQTail)-1])), float64(imag(rt.lastAllIQTail[len(rt.lastAllIQTail)-1]))))
			phaseJump := phaseStepAbs(rt.lastAllIQTail[len(rt.lastAllIQTail)-1], allIQ[0])
			rt.telemetry.Observe("iq.boundary.all.delta_mag", deltaMag, nil)
			rt.telemetry.Observe("iq.boundary.all.delta_phase", phaseJump, nil)
			if rt.telemetry.ShouldSampleHeavy() {
				rt.telemetry.Event("alliq_boundary", "info", "allIQ boundary snapshot", nil, map[string]any{
					"window":                boundaryWindow,
					"head_mean_mag":         headMean,
					"prev_tail_mean_mag":    tailMean,
					"delta_mag":             deltaMag,
					"delta_phase":           phaseJump,
					"discontinuity_score":   boundaryScore,
					"alliq_len":             len(allIQ),
					"stream_dropped":        streamDropped,
				})
			}
		}
		if rt.telemetry.ShouldSampleHeavy() {
			observeIQStats(rt.telemetry, "capture_all", allIQ, nil)
			observeIQStats(rt.telemetry, "capture_surveillance", survIQ, nil)
			observeIQStats(rt.telemetry, "capture_detail", detailIQ, nil)
		}
	}
	rt.lastAllIQTail = tailWindowComplex(allIQ, 32)
	survSpectrum := rt.surveillanceSpectrumFromIQ(allIQ, survIQ, gpuState)
	sanitizeSpectrum(survSpectrum)
	detailSpectrum := survSpectrum
	if !sameIQBuffer(detailIQ, survIQ) {
		detailSpectrum = rt.spectrumFromIQWithPlan(detailIQ, rt.detailWindow, rt.detailPlan, gpuState, false)
		sanitizeSpectrum(detailSpectrum)
	}
	policy := pipeline.PolicyFromConfig(rt.cfg)
	plan := rt.buildSurveillancePlan(policy)
	surveillanceSpectra := make([]pipeline.SurveillanceLevelSpectrum, 0, len(plan.Specs))
	for _, spec := range plan.Specs {
		if spec.Level.FFTSize <= 0 {
			continue
		}
		var spectrum []float64
		if spec.Decim <= 1 {
			if spec.Level.FFTSize == len(survSpectrum) {
				spectrum = survSpectrum
			} else {
				spectrum = rt.spectrumForLevel(survIQ, spec.Level.FFTSize, gpuState, spec.AllowGPU)
				sanitizeSpectrum(spectrum)
			}
		} else {
			required := spec.Level.FFTSize * spec.Decim
			if required > len(survIQ) {
				continue
			}
			src := survIQ
			if len(src) > required {
				src = src[len(src)-required:]
			}
			decimated := rt.decimateSurveillanceIQ(src, spec.Decim)
			spectrum = rt.spectrumForLevel(decimated, spec.Level.FFTSize, gpuState, false)
			sanitizeSpectrum(spectrum)
		}
		if len(spectrum) == 0 {
			continue
		}
		surveillanceSpectra = append(surveillanceSpectra, pipeline.SurveillanceLevelSpectrum{Level: spec.Level, Spectrum: spectrum})
	}
	now := time.Now()
	finished, detected := rt.det.Process(now, survSpectrum, rt.cfg.CenterHz)
	if rt.telemetry != nil {
		rt.telemetry.SetGauge("signals.detected.count", float64(len(detected)), nil)
		rt.telemetry.SetGauge("signals.finished.count", float64(len(finished)), nil)
	}
	return &spectrumArtifacts{
		allIQ:                allIQ,
		streamDropped:        streamDropped,
		surveillanceIQ:       survIQ,
		detailIQ:             detailIQ,
		surveillanceSpectrum: survSpectrum,
		surveillanceSpectra:  surveillanceSpectra,
		surveillancePlan:     plan,
		detailSpectrum:       detailSpectrum,
		finished:             finished,
		detected:             detected,
		thresholds:           rt.det.LastThresholds(),
		noiseFloor:           rt.det.LastNoiseFloor(),
		now:                  now,
	}, nil
}

func (rt *dspRuntime) buildSurveillanceResult(art *spectrumArtifacts) pipeline.SurveillanceResult {
	if art == nil {
		return pipeline.SurveillanceResult{}
	}
	policy := pipeline.PolicyFromConfig(rt.cfg)
	plan := art.surveillancePlan
	if plan.Primary.Name == "" {
		plan = rt.buildSurveillancePlan(policy)
	}
	primaryCandidates := pipeline.CandidatesFromSignalsWithLevel(art.detected, "surveillance-detector", plan.Primary)
	derivedCandidates := rt.detectDerivedCandidates(art, plan)
	candidates := pipeline.FuseCandidates(primaryCandidates, derivedCandidates)
	pipeline.ApplyMonitorWindowMatchesToCandidates(policy, candidates)
	scheduled := pipeline.ScheduleCandidates(candidates, policy)
	return pipeline.SurveillanceResult{
		Level:           plan.Primary,
		Levels:          plan.Levels,
		LevelSet:        plan.LevelSet,
		DetectionPolicy: plan.DetectionPolicy,
		DisplayLevel:    plan.Presentation,
		Context:         plan.Context,
		Spectra:         art.surveillanceSpectra,
		Candidates:      candidates,
		Scheduled:       scheduled,
		Finished:        art.finished,
		Signals:         art.detected,
		NoiseFloor:      art.noiseFloor,
		Thresholds:      art.thresholds,
	}
}

func (rt *dspRuntime) detectDerivedCandidates(art *spectrumArtifacts, plan surveillancePlan) []pipeline.Candidate {
	if art == nil || len(plan.LevelSet.Derived) == 0 {
		return nil
	}
	spectra := map[string][]float64{}
	for _, spec := range art.surveillanceSpectra {
		if spec.Level.Name == "" || len(spec.Spectrum) == 0 {
			continue
		}
		spectra[spec.Level.Name] = spec.Spectrum
	}
	if len(spectra) == 0 {
		return nil
	}
	out := make([]pipeline.Candidate, 0, len(plan.LevelSet.Derived))
	for _, level := range plan.LevelSet.Derived {
		if level.Name == "" {
			continue
		}
		if !pipeline.IsDetectionLevel(level) {
			continue
		}
		spectrum := spectra[level.Name]
		if len(spectrum) == 0 {
			continue
		}
		entry := rt.derivedDetectorForLevel(level)
		if entry == nil || entry.det == nil {
			continue
		}
		_, signals := entry.det.Process(art.now, spectrum, level.CenterHz)
		if len(signals) == 0 {
			continue
		}
		cands := pipeline.CandidatesFromSignalsWithLevel(signals, "surveillance-derived", level)
		for i := range cands {
			if cands[i].ID == 0 {
				continue
			}
			cands[i].ID = entry.idBase - cands[i].ID
		}
		out = append(out, cands...)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (rt *dspRuntime) derivedDetectorForLevel(level pipeline.AnalysisLevel) *derivedDetector {
	if level.SampleRate <= 0 || level.FFTSize <= 0 {
		return nil
	}
	if rt.derivedDetectors == nil {
		rt.derivedDetectors = map[string]*derivedDetector{}
	}
	key := level.Name
	if key == "" {
		key = fmt.Sprintf("%d:%d", level.SampleRate, level.FFTSize)
	}
	entry := rt.derivedDetectors[key]
	if entry != nil && entry.sampleRate == level.SampleRate && entry.fftSize == level.FFTSize {
		return entry
	}
	if rt.nextDerivedBase == 0 {
		rt.nextDerivedBase = -derivedIDBlock
	}
	entry = &derivedDetector{
		det:        detector.New(rt.cfg.Detector, level.SampleRate, level.FFTSize),
		sampleRate: level.SampleRate,
		fftSize:    level.FFTSize,
		idBase:     rt.nextDerivedBase,
	}
	rt.nextDerivedBase -= derivedIDBlock
	rt.derivedDetectors[key] = entry
	return entry
}

func (rt *dspRuntime) buildRefinementInput(surv pipeline.SurveillanceResult, now time.Time) pipeline.RefinementInput {
	policy := pipeline.PolicyFromConfig(rt.cfg)
	baseBudget := pipeline.BudgetModelFromPolicy(policy)
	pressure := pipeline.BuildBudgetPressureSummary(baseBudget, rt.arbitration.Refinement, rt.arbitration.Queue)
	budget := pipeline.ApplyBudgetRebalance(policy, baseBudget, pressure)
	plan := pipeline.BuildRefinementPlanWithBudget(surv.Candidates, policy, budget)
	admission := rt.arbiter.AdmitRefinementWithBudget(plan, policy, budget, now)
	plan = admission.Plan
	workItems := make([]pipeline.RefinementWorkItem, 0, len(admission.WorkItems))
	if len(admission.WorkItems) > 0 {
		workItems = append(workItems, admission.WorkItems...)
	}
	scheduled := append([]pipeline.ScheduledCandidate(nil), admission.Admitted...)
	workIndex := map[int64]int{}
	for i := range workItems {
		if workItems[i].Candidate.ID == 0 {
			continue
		}
		workIndex[workItems[i].Candidate.ID] = i
	}
	windows := make([]pipeline.RefinementWindow, 0, len(scheduled))
	for _, sc := range scheduled {
		window := pipeline.RefinementWindowForCandidate(policy, sc.Candidate)
		windows = append(windows, window)
		if idx, ok := workIndex[sc.Candidate.ID]; ok {
			workItems[idx].Window = window
		}
	}
	detailFFT := rt.cfg.Refinement.DetailFFTSize
	if detailFFT <= 0 {
		detailFFT = rt.cfg.FFTSize
	}
	levelSpan := spanForPolicy(policy, float64(rt.cfg.SampleRate))
	if _, maxSpan, ok := windowSpanBounds(windows); ok {
		levelSpan = maxSpan
	}
	level := analysisLevel("refinement", "refinement", "refinement", rt.cfg.SampleRate, detailFFT, rt.cfg.CenterHz, levelSpan, "refinement-window", 1, rt.cfg.SampleRate)
	detailLevel := analysisLevel("detail", "detail", "refinement", rt.cfg.SampleRate, detailFFT, rt.cfg.CenterHz, levelSpan, "detail-spectrum", 1, rt.cfg.SampleRate)
	if len(workItems) > 0 {
		for i := range workItems {
			item := &workItems[i]
			if item.Window.SpanHz <= 0 {
				continue
			}
			item.Execution = &pipeline.RefinementExecution{
				Stage:      "refine",
				SampleRate: rt.cfg.SampleRate,
				FFTSize:    detailFFT,
				CenterHz:   item.Window.CenterHz,
				SpanHz:     item.Window.SpanHz,
				Source:     detailLevel.Source,
			}
		}
	}
	input := pipeline.RefinementInput{
		Level:      level,
		Detail:     detailLevel,
		Context:    surv.Context,
		Request:    pipeline.RefinementRequest{Strategy: plan.Strategy, Reason: "surveillance-plan", SpanHintHz: levelSpan},
		Budgets:    budget,
		Admission:  admission.Admission,
		Candidates: append([]pipeline.Candidate(nil), surv.Candidates...),
		Scheduled:  scheduled,
		WorkItems:  workItems,
		Plan:       plan,
		Windows:    windows,
		SampleRate: rt.cfg.SampleRate,
		FFTSize:    detailFFT,
		CenterHz:   rt.cfg.CenterHz,
		Source:     "surveillance-detector",
	}
	input.Context.Refinement = level
	input.Context.Detail = detailLevel
	if !policy.RefinementEnabled {
		for i := range input.WorkItems {
			item := &input.WorkItems[i]
			if item.Status == pipeline.RefinementStatusDropped {
				continue
			}
			item.Status = pipeline.RefinementStatusDropped
			item.Reason = pipeline.RefinementReasonDisabled
		}
		input.Scheduled = nil
		input.Request.Reason = pipeline.ReasonAdmissionDisabled
		input.Admission.Reason = pipeline.ReasonAdmissionDisabled
		input.Admission.Admitted = 0
		input.Admission.Skipped = 0
		input.Admission.Displaced = 0
		input.Plan.Selected = nil
		input.Plan.DroppedByBudget = 0
	}
	rt.setArbitration(policy, input.Budgets, input.Admission, rt.arbitration.Queue)
	return input
}

func (rt *dspRuntime) runRefinement(art *spectrumArtifacts, surv pipeline.SurveillanceResult, extractMgr *extractionManager, rec *recorder.Manager) pipeline.RefinementStep {
	input := rt.buildRefinementInput(surv, art.now)
	markWorkItemsStatus(input.WorkItems, pipeline.RefinementStatusAdmitted, pipeline.RefinementStatusRunning, pipeline.RefinementReasonRunning)
	result := rt.refineSignals(art, input, extractMgr, rec)
	markWorkItemsCompleted(input.WorkItems, result.Candidates)
	return pipeline.RefinementStep{Input: input, Result: result}
}

func (rt *dspRuntime) refineSignals(art *spectrumArtifacts, input pipeline.RefinementInput, extractMgr *extractionManager, rec *recorder.Manager) pipeline.RefinementResult {
	if art == nil || len(art.detailIQ) == 0 || len(input.Scheduled) == 0 {
		return pipeline.RefinementResult{}
	}
	policy := pipeline.PolicyFromConfig(rt.cfg)
	selectedCandidates := make([]pipeline.Candidate, 0, len(input.Scheduled))
	selectedSignals := make([]detector.Signal, 0, len(input.Scheduled))
	for _, sc := range input.Scheduled {
		selectedCandidates = append(selectedCandidates, sc.Candidate)
		selectedSignals = append(selectedSignals, detector.Signal{
			ID:       sc.Candidate.ID,
			FirstBin: sc.Candidate.FirstBin,
			LastBin:  sc.Candidate.LastBin,
			CenterHz: sc.Candidate.CenterHz,
			BWHz:     sc.Candidate.BandwidthHz,
			PeakDb:   sc.Candidate.PeakDb,
			SNRDb:    sc.Candidate.SNRDb,
			NoiseDb:  sc.Candidate.NoiseDb,
		})
	}
	sampleRate := input.SampleRate
	fftSize := input.FFTSize
	centerHz := input.CenterHz
	if sampleRate <= 0 {
		sampleRate = rt.cfg.SampleRate
	}
	if fftSize <= 0 {
		fftSize = rt.cfg.FFTSize
	}
	if centerHz == 0 {
		centerHz = rt.cfg.CenterHz
	}
	snips, snipRates := extractSignalIQBatch(extractMgr, art.detailIQ, sampleRate, centerHz, selectedSignals)
	occBwFraction := rt.cfg.Refinement.OccupiedBwFraction
	switch {
	case occBwFraction == 0:
		occBwFraction = 0.99 // default on
	case occBwFraction < 0:
		occBwFraction = 0 // explicitly disabled
	}
	refined := pipeline.RefineCandidates(selectedCandidates, input.Windows, art.detailSpectrum, sampleRate, fftSize, snips, snipRates, classifier.ClassifierMode(rt.cfg.ClassifierMode), art.surveillanceSpectrum, occBwFraction)
	signals := make([]detector.Signal, 0, len(refined))
	decisions := make([]pipeline.SignalDecision, 0, len(refined))
	for i, ref := range refined {
		sig := ref.Signal
		signals = append(signals, sig)
		cls := sig.Class
		snipRate := ref.SnippetRate
		decision := pipeline.DecideSignalAction(policy, ref.Candidate, cls)
		decisions = append(decisions, decision)
		if cls != nil {
			if cls.ModType == classifier.ClassWFM {
				cls.ModType = classifier.ClassWFMStereo
				signals[i].PlaybackMode = string(classifier.ClassWFMStereo)
				signals[i].DemodName = string(classifier.ClassWFMStereo)
				signals[i].StereoState = "searching"
			}
			pll := classifier.PLLResult{}
			if i < len(snips) && snips[i] != nil && len(snips[i]) > 256 {
				// Short-window PLL: kept only for the exact-frequency (carrier offset)
				// estimate. Its .Stereo is NOT trusted — the ~1 ms snippet cannot
				// separate the 19 kHz pilot from noise (OI-24). Stereo lock is driven
				// by the long-window detection in updateRDS below.
				pll = classifier.EstimateExactFrequency(snips[i], snipRate, signals[i].CenterHz, cls.ModType)
				cls.PLL = &pll
				signals[i].PLL = &pll
			}
			if cls.ModType == classifier.ClassWFMStereo {
				signals[i].PlaybackMode = string(classifier.ClassWFMStereo)
				signals[i].DemodName = string(classifier.ClassWFMStereo)
				if signals[i].StereoState == "" {
					signals[i].StereoState = "searching"
				}
			}
			if cls.ModType == classifier.ClassWFMStereo && rec != nil {
				// updateRDS runs the stereo pilot + RDS decode on a multi-second ring
				// slice and sets signals[i].PLL.Stereo / .RDSStation.
				rt.updateRDS(art.now, rec, extractMgr, &signals[i], cls)
				// Sticky lock: the long-window decode only refreshes every few
				// seconds (RDS cadence), so hold "locked" between refreshes to keep
				// the indicator from flickering.
				id := signals[i].ID
				if rt.stereoHold == nil {
					rt.stereoHold = map[int64]time.Time{}
				}
				if signals[i].PLL != nil && (signals[i].PLL.Stereo || signals[i].PLL.RDSStation != "") {
					rt.stereoHold[id] = art.now.Add(6 * time.Second)
				}
				if until, ok := rt.stereoHold[id]; ok && art.now.Before(until) {
					signals[i].StereoState = "locked"
				}
			}
		}
	}
	for id, until := range rt.stereoHold {
		if art.now.Sub(until) > 30*time.Second {
			delete(rt.stereoHold, id)
		}
	}
	budget := input.Budgets
	queueStats := rt.arbiter.ApplyDecisions(decisions, budget, art.now, policy)
	rt.setArbitration(policy, budget, input.Admission, queueStats)
	summary := summarizeDecisions(decisions)
	if rec != nil {
		if summary.RecordEnabled > 0 {
			rt.cfg.Recorder.Enabled = true
		}
		if summary.DecodeEnabled > 0 {
			rt.cfg.Recorder.AutoDecode = true
		}
	}
	rt.det.UpdateClasses(signals)
	return pipeline.RefinementResult{Level: input.Level, Signals: signals, Decisions: decisions, Candidates: selectedCandidates}
}

func (rt *dspRuntime) updateRDS(now time.Time, rec *recorder.Manager, extractMgr *extractionManager, sig *detector.Signal, cls *classifier.Classification) {
	if sig == nil || cls == nil {
		return
	}
	// Key the RDS state by the stable tracked signal ID, not the (jittering)
	// center frequency. A frequency-quantized key flips whenever the detected
	// center wobbles, spawning a fresh rdsState every frame — which defeats the
	// reused IQ buffer (re-allocating ~100+ MB) and re-fires the decode every
	// frame instead of every few seconds. The tracker ID is stable per station.
	key := sig.ID
	if key == 0 {
		return
	}
	st := rt.rdsMap[key]
	if st == nil {
		st = &rdsState{}
		rt.rdsMap[key] = st
	}
	st.lastSeen = now
	// Prune states for signals that have disappeared so their large reused IQ
	// buffers (~100+ MB each) are released; ID keys are otherwise unbounded.
	for id, other := range rt.rdsMap {
		if now.Sub(other.lastSeen) > 30*time.Second && atomic.LoadInt32(&other.busy) == 0 {
			delete(rt.rdsMap, id)
		}
	}
	if now.Sub(st.lastDecode) >= 4*time.Second && atomic.LoadInt32(&st.busy) == 0 {
		st.lastDecode = now
		atomic.StoreInt32(&st.busy, 1)
		go func(st *rdsState, sigHz float64) {
			defer atomic.StoreInt32(&st.busy, 0)
			// Reuse the per-station buffer (only this goroutine touches st.iqBuf;
			// the busy flag guarantees one at a time) to avoid a ~100+ MB
			// allocation per decode — the dominant allocation source.
			ringIQ, ringSR, ringCenter := rec.SliceRecentInto(4.0, st.iqBuf)
			st.iqBuf = ringIQ
			if len(ringIQ) < ringSR || ringSR <= 0 {
				return
			}
			offset := sigHz - ringCenter
			var decimated []complex64
			var actualRate int
			// GPU-first (OI-26 / AGENTS §7): shift+filter+decimate on the GPU.
			// The GPU polyphase extractor needs an integer decimation factor, so
			// derive an out-rate that divides the sample rate evenly (~250 kHz).
			rdsDecim := ringSR / 250000
			if rdsDecim < 1 {
				rdsDecim = 1
			}
			rdsOutRate := ringSR / rdsDecim
			if out, rate, ok := extractMgr.extractRDS(ringIQ, ringSR, offset, 200000, rdsOutRate); ok {
				decimated = out
				actualRate = rate
			} else {
				// CPU fallback (no GPU / mock).
				shifted := dsp.FreqShift(ringIQ, ringSR, offset)
				decim1 := ringSR / 1000000
				if decim1 < 1 {
					decim1 = 1
				}
				lp1 := dsp.LowpassFIR(float64(ringSR/decim1)/2.0*0.8, ringSR, 51)
				f1 := dsp.ApplyFIR(shifted, lp1)
				d1 := dsp.Decimate(f1, decim1)
				rate1 := ringSR / decim1
				decim2 := rate1 / 250000
				if decim2 < 1 {
					decim2 = 1
				}
				lp2 := dsp.LowpassFIR(float64(rate1/decim2)/2.0*0.8, rate1, 101)
				f2 := dsp.ApplyFIR(d1, lp2)
				decimated = dsp.Decimate(f2, decim2)
				actualRate = rate1 / decim2
			}
			// Long-window stereo pilot lock (OI-24): the per-frame snippet PLL only
			// sees ~1 ms and cannot separate the 19 kHz pilot from noise. Reuse this
			// same multi-second baseband (carrier at DC after the shift) — the pilot
			// is tens of dB over the floor here.
			stereo := classifier.StereoPilotPresent(decimated, actualRate)
			st.mu.Lock()
			st.stereo = stereo
			st.mu.Unlock()
			rdsBase := demod.RDSBasebandComplex(decimated, actualRate)
			if len(rdsBase.Samples) == 0 {
				return
			}
			st.mu.Lock()
			result := st.dec.Decode(rdsBase.Samples, rdsBase.SampleRate)
			if result.PS != "" {
				st.result = result
			}
			st.mu.Unlock()
		}(st, sig.CenterHz)
	}
	st.mu.Lock()
	ps := st.result.PS
	stereo := st.stereo
	st.mu.Unlock()
	if sig.PLL == nil {
		sig.PLL = &classifier.PLLResult{ExactHz: sig.CenterHz, Method: "pilot"}
	}
	sig.PLL.Stereo = stereo
	if ps != "" {
		sig.PLL.RDSStation = strings.TrimSpace(ps)
	}
	cls.PLL = sig.PLL
}

func (rt *dspRuntime) maintenance(displaySignals []detector.Signal, rec *recorder.Manager) {
	if len(rt.rdsMap) > 0 {
		// rt.rdsMap is keyed by the stable tracker ID (see updateRDS, OI-25). Prune
		// by that same ID — a previous frequency-quantized key (keyHz/25000) never
		// matched the ID keys, so every rdsState was deleted every frame: the async
		// pilot/RDS decode wrote to an orphaned state, the next frame recreated a
		// fresh one (stereo=false, lastDecode=0 → relaunch), so stereo never stuck
		// and the RDS decoder state never accumulated.
		activeIDs := make(map[int64]bool, len(displaySignals))
		for _, s := range displaySignals {
			if s.ID != 0 {
				activeIDs[s.ID] = true
			}
		}
		for id := range rt.rdsMap {
			if !activeIDs[id] {
				delete(rt.rdsMap, id)
			}
		}
	}
	if len(rt.streamPhaseState) > 0 {
		sigIDs := make(map[int64]bool, len(displaySignals))
		for _, s := range displaySignals {
			sigIDs[s.ID] = true
		}
		for id := range rt.streamPhaseState {
			if !sigIDs[id] {
				delete(rt.streamPhaseState, id)
			}
		}
	}
	if rec != nil && len(displaySignals) > 0 {
		aqCfg := extractionConfig{firTaps: rt.cfg.Recorder.ExtractionTaps, bwMult: rt.cfg.Recorder.ExtractionBwMult}
		_ = aqCfg
	}
}

func spanForPolicy(policy pipeline.Policy, fallback float64) float64 {
	if policy.MonitorSpanHz > 0 {
		return policy.MonitorSpanHz
	}
	if len(policy.MonitorWindows) > 0 {
		maxSpan := 0.0
		for _, w := range policy.MonitorWindows {
			if w.SpanHz > maxSpan {
				maxSpan = w.SpanHz
			}
		}
		if maxSpan > 0 {
			return maxSpan
		}
	}
	if policy.MonitorStartHz != 0 && policy.MonitorEndHz != 0 && policy.MonitorEndHz > policy.MonitorStartHz {
		return policy.MonitorEndHz - policy.MonitorStartHz
	}
	return fallback
}

func windowSpanBounds(windows []pipeline.RefinementWindow) (float64, float64, bool) {
	minSpan := 0.0
	maxSpan := 0.0
	ok := false
	for _, w := range windows {
		if w.SpanHz <= 0 {
			continue
		}
		if !ok || w.SpanHz < minSpan {
			minSpan = w.SpanHz
		}
		if !ok || w.SpanHz > maxSpan {
			maxSpan = w.SpanHz
		}
		ok = true
	}
	return minSpan, maxSpan, ok
}

func analysisLevel(name, role, truth string, sampleRate int, fftSize int, centerHz float64, spanHz float64, source string, decimation int, baseRate int) pipeline.AnalysisLevel {
	level := pipeline.AnalysisLevel{
		Name:       name,
		Role:       role,
		Truth:      truth,
		SampleRate: sampleRate,
		FFTSize:    fftSize,
		CenterHz:   centerHz,
		SpanHz:     spanHz,
		Source:     source,
	}
	if level.SampleRate > 0 && level.FFTSize > 0 {
		level.BinHz = float64(level.SampleRate) / float64(level.FFTSize)
	}
	if decimation > 0 {
		level.Decimation = decimation
	} else if baseRate > 0 && level.SampleRate > 0 && baseRate%level.SampleRate == 0 {
		level.Decimation = baseRate / level.SampleRate
	}
	return level
}

func (rt *dspRuntime) buildSurveillancePlan(policy pipeline.Policy) surveillancePlan {
	baseRate := rt.cfg.SampleRate
	baseFFT := rt.cfg.Surveillance.AnalysisFFTSize
	if baseFFT <= 0 {
		baseFFT = rt.cfg.FFTSize
	}
	span := spanForPolicy(policy, float64(baseRate))
	detectionPolicy := pipeline.SurveillanceDetectionPolicyFromPolicy(policy)
	primary := analysisLevel("surveillance", pipeline.RoleSurveillancePrimary, "surveillance", baseRate, baseFFT, rt.cfg.CenterHz, span, "baseband", 1, baseRate)
	levels := []pipeline.AnalysisLevel{primary}
	specs := []surveillanceLevelSpec{{Level: primary, Decim: 1, AllowGPU: true}}
	context := pipeline.AnalysisContext{Surveillance: primary}
	derivedLevels := make([]pipeline.AnalysisLevel, 0, 2)
	supportLevels := make([]pipeline.AnalysisLevel, 0, 2)

	strategy := strings.ToLower(strings.TrimSpace(policy.SurveillanceStrategy))
	switch strategy {
	case "multi-res", "multi-resolution", "multi", "multi_res":
		decim := 2
		derivedRate := baseRate / decim
		derivedFFT := baseFFT / decim
		if derivedRate >= 200000 && derivedFFT >= 256 {
			derivedSpan := spanForPolicy(policy, float64(derivedRate))
			role := pipeline.RoleSurveillanceSupport
			if detectionPolicy.DerivedDetectionEnabled {
				role = pipeline.RoleSurveillanceDerived
			}
			derived := analysisLevel("surveillance-lowres", role, "surveillance", derivedRate, derivedFFT, rt.cfg.CenterHz, derivedSpan, "decimated", decim, baseRate)
			if detectionPolicy.DerivedDetectionEnabled {
				levels = append(levels, derived)
				derivedLevels = append(derivedLevels, derived)
			} else {
				supportLevels = append(supportLevels, derived)
			}
			specs = append(specs, surveillanceLevelSpec{Level: derived, Decim: decim, AllowGPU: false})
			context.Derived = append(context.Derived, derived)
		}
	}

	presentation := analysisLevel("presentation", pipeline.RolePresentation, "presentation", baseRate, rt.cfg.Surveillance.DisplayBins, rt.cfg.CenterHz, span, "display", 1, baseRate)
	context.Presentation = presentation
	if len(derivedLevels) == 0 && detectionPolicy.DerivedDetectionEnabled {
		detectionPolicy.DerivedDetectionEnabled = false
		detectionPolicy.DerivedDetectionReason = "levels"
	}
	switch {
	case len(derivedLevels) > 0:
		detectionPolicy.DerivedDetectionMode = "detection"
	case len(supportLevels) > 0:
		detectionPolicy.DerivedDetectionMode = "support"
	default:
		detectionPolicy.DerivedDetectionMode = "disabled"
	}
	levelSet := pipeline.SurveillanceLevelSet{
		Primary:      primary,
		Derived:      append([]pipeline.AnalysisLevel(nil), derivedLevels...),
		Support:      append([]pipeline.AnalysisLevel(nil), supportLevels...),
		Presentation: presentation,
	}
	detectionLevels := make([]pipeline.AnalysisLevel, 0, 1+len(derivedLevels))
	detectionLevels = append(detectionLevels, primary)
	detectionLevels = append(detectionLevels, derivedLevels...)
	levelSet.Detection = detectionLevels
	allLevels := make([]pipeline.AnalysisLevel, 0, 1+len(derivedLevels)+len(supportLevels)+1)
	allLevels = append(allLevels, primary)
	allLevels = append(allLevels, derivedLevels...)
	allLevels = append(allLevels, supportLevels...)
	if presentation.Name != "" {
		allLevels = append(allLevels, presentation)
	}
	levelSet.All = allLevels

	return surveillancePlan{
		Primary:         primary,
		Levels:          levels,
		LevelSet:        levelSet,
		Presentation:    presentation,
		Context:         context,
		DetectionPolicy: detectionPolicy,
		Specs:           specs,
	}
}

func sameIQBuffer(a []complex64, b []complex64) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	return &a[0] == &b[0]
}

func markWorkItemsStatus(items []pipeline.RefinementWorkItem, from string, to string, reason string) {
	for i := range items {
		if items[i].Status != from {
			continue
		}
		items[i].Status = to
		if reason != "" {
			items[i].Reason = reason
		}
	}
}

func markWorkItemsCompleted(items []pipeline.RefinementWorkItem, candidates []pipeline.Candidate) {
	if len(items) == 0 || len(candidates) == 0 {
		return
	}
	done := map[int64]struct{}{}
	for _, cand := range candidates {
		if cand.ID != 0 {
			done[cand.ID] = struct{}{}
		}
	}
	for i := range items {
		if _, ok := done[items[i].Candidate.ID]; !ok {
			continue
		}
		items[i].Status = pipeline.RefinementStatusCompleted
		items[i].Reason = pipeline.RefinementReasonCompleted
	}
}

func (rt *dspRuntime) setArbitration(policy pipeline.Policy, budget pipeline.BudgetModel, admission pipeline.RefinementAdmission, queue pipeline.DecisionQueueStats) {
	rt.arbitration = pipeline.BuildArbitrationState(policy, budget, admission, queue)
}

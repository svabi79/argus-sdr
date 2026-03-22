package main

import (
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"sdr-wideband-suite/internal/classifier"
	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/demod"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/dsp"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/fft/gpufft"
	"sdr-wideband-suite/internal/pipeline"
	"sdr-wideband-suite/internal/rds"
	"sdr-wideband-suite/internal/recorder"
)

type rdsState struct {
	dec        rds.Decoder
	result     rds.Result
	lastDecode time.Time
	busy       int32
	mu         sync.Mutex
}

type dspRuntime struct {
	cfg              config.Config
	det              *detector.Detector
	window           []float64
	plan             *fftutil.CmplxPlan
	dcEnabled        bool
	iqEnabled        bool
	useGPU           bool
	gpuEngine        *gpufft.Engine
	rdsMap           map[int64]*rdsState
	streamPhaseState map[int64]*streamExtractState
	streamOverlap    *streamIQOverlap
	decisionQueues   *decisionQueues
	queueStats       decisionQueueStats
	gotSamples       bool
}

type spectrumArtifacts struct {
	allIQ                []complex64
	surveillanceIQ       []complex64
	detailIQ             []complex64
	surveillanceSpectrum []float64
	detailSpectrum       []float64
	finished             []detector.Event
	detected             []detector.Signal
	thresholds           []float64
	noiseFloor           float64
	now                  time.Time
}

func newDSPRuntime(cfg config.Config, det *detector.Detector, window []float64, gpuState *gpuStatus) *dspRuntime {
	rt := &dspRuntime{
		cfg:              cfg,
		det:              det,
		window:           window,
		plan:             fftutil.NewCmplxPlan(cfg.FFTSize),
		dcEnabled:        cfg.DCBlock,
		iqEnabled:        cfg.IQBalance,
		useGPU:           cfg.UseGPUFFT,
		rdsMap:           map[int64]*rdsState{},
		streamPhaseState: map[int64]*streamExtractState{},
		streamOverlap:    &streamIQOverlap{},
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
	prevUseGPU := rt.useGPU
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
}

func (rt *dspRuntime) spectrumFromIQ(iq []complex64, gpuState *gpuStatus) []float64 {
	if len(iq) == 0 {
		return nil
	}
	if rt.useGPU && rt.gpuEngine != nil {
		gpuBuf := make([]complex64, len(iq))
		if len(rt.window) == len(iq) {
			for i := 0; i < len(iq); i++ {
				v := iq[i]
				w := float32(rt.window[i])
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
			return fftutil.SpectrumWithPlan(gpuBuf, nil, rt.plan)
		}
		return fftutil.SpectrumFromFFT(out)
	}
	return fftutil.SpectrumWithPlan(iq, rt.window, rt.plan)
}

func (rt *dspRuntime) captureSpectrum(srcMgr *sourceManager, rec *recorder.Manager, dcBlocker *dsp.DCBlocker, gpuState *gpuStatus) (*spectrumArtifacts, error) {
	available := rt.cfg.FFTSize
	st := srcMgr.Stats()
	if st.BufferSamples > rt.cfg.FFTSize {
		available = (st.BufferSamples / rt.cfg.FFTSize) * rt.cfg.FFTSize
		if available < rt.cfg.FFTSize {
			available = rt.cfg.FFTSize
		}
	}
	allIQ, err := srcMgr.ReadIQ(available)
	if err != nil {
		return nil, err
	}
	if rec != nil {
		rec.Ingest(time.Now(), allIQ)
	}
	survIQ := allIQ
	if len(allIQ) > rt.cfg.FFTSize {
		survIQ = allIQ[len(allIQ)-rt.cfg.FFTSize:]
	}
	if rt.dcEnabled {
		dcBlocker.Apply(survIQ)
	}
	if rt.iqEnabled {
		dsp.IQBalance(survIQ)
	}
	survSpectrum := rt.spectrumFromIQ(survIQ, gpuState)
	for i := range survSpectrum {
		if math.IsNaN(survSpectrum[i]) || math.IsInf(survSpectrum[i], 0) {
			survSpectrum[i] = -200
		}
	}
	detailIQ := survIQ
	detailSpectrum := survSpectrum
	if !sameIQBuffer(detailIQ, survIQ) {
		detailSpectrum = rt.spectrumFromIQ(detailIQ, gpuState)
		for i := range detailSpectrum {
			if math.IsNaN(detailSpectrum[i]) || math.IsInf(detailSpectrum[i], 0) {
				detailSpectrum[i] = -200
			}
		}
	}
	now := time.Now()
	finished, detected := rt.det.Process(now, survSpectrum, rt.cfg.CenterHz)
	return &spectrumArtifacts{
		allIQ:                allIQ,
		surveillanceIQ:       survIQ,
		detailIQ:             detailIQ,
		surveillanceSpectrum: survSpectrum,
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
	candidates := pipeline.CandidatesFromSignals(art.detected, "surveillance-detector")
	scheduled := pipeline.ScheduleCandidates(candidates, policy)
	level := pipeline.AnalysisLevel{
		Name:       "surveillance",
		Role:       "surveillance",
		Truth:      "surveillance",
		SampleRate: rt.cfg.SampleRate,
		FFTSize:    rt.cfg.Surveillance.AnalysisFFTSize,
		CenterHz:   rt.cfg.CenterHz,
		SpanHz:     spanForPolicy(policy, float64(rt.cfg.SampleRate)),
		Source:     "baseband",
	}
	lowRate := rt.cfg.SampleRate / 2
	lowFFT := rt.cfg.Surveillance.AnalysisFFTSize / 2
	if lowRate < 200000 {
		lowRate = rt.cfg.SampleRate
	}
	if lowFFT < 256 {
		lowFFT = rt.cfg.Surveillance.AnalysisFFTSize
	}
	lowLevel := pipeline.AnalysisLevel{
		Name:       "surveillance-lowres",
		Role:       "surveillance-lowres",
		Truth:      "surveillance",
		SampleRate: lowRate,
		FFTSize:    lowFFT,
		CenterHz:   rt.cfg.CenterHz,
		SpanHz:     spanForPolicy(policy, float64(lowRate)),
		Source:     "downsampled",
	}
	displayLevel := pipeline.AnalysisLevel{
		Name:       "presentation",
		Role:       "presentation",
		Truth:      "presentation",
		SampleRate: rt.cfg.SampleRate,
		FFTSize:    rt.cfg.Surveillance.DisplayBins,
		CenterHz:   rt.cfg.CenterHz,
		SpanHz:     spanForPolicy(policy, float64(rt.cfg.SampleRate)),
		Source:     "display",
	}
	levels, context := surveillanceLevels(policy, level, lowLevel, displayLevel)
	return pipeline.SurveillanceResult{
		Level:        level,
		Levels:       levels,
		DisplayLevel: displayLevel,
		Context:      context,
		Candidates:   candidates,
		Scheduled:    scheduled,
		Finished:     art.finished,
		Signals:      art.detected,
		NoiseFloor:   art.noiseFloor,
		Thresholds:   art.thresholds,
	}
}

func (rt *dspRuntime) buildRefinementInput(surv pipeline.SurveillanceResult) pipeline.RefinementInput {
	policy := pipeline.PolicyFromConfig(rt.cfg)
	plan := pipeline.BuildRefinementPlan(surv.Candidates, policy)
	scheduled := append([]pipeline.ScheduledCandidate(nil), surv.Scheduled...)
	if len(scheduled) == 0 && len(plan.Selected) > 0 {
		scheduled = append([]pipeline.ScheduledCandidate(nil), plan.Selected...)
	}
	workItems := make([]pipeline.RefinementWorkItem, 0, len(plan.WorkItems))
	if len(plan.WorkItems) > 0 {
		workItems = append(workItems, plan.WorkItems...)
	}
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
	levelSpan := spanForPolicy(policy, float64(rt.cfg.SampleRate))
	if _, maxSpan, ok := windowSpanBounds(windows); ok {
		levelSpan = maxSpan
	}
	level := pipeline.AnalysisLevel{
		Name:       "refinement",
		Role:       "refinement",
		Truth:      "refinement",
		SampleRate: rt.cfg.SampleRate,
		FFTSize:    rt.cfg.FFTSize,
		CenterHz:   rt.cfg.CenterHz,
		SpanHz:     levelSpan,
		Source:     "refinement-window",
	}
	detailLevel := pipeline.AnalysisLevel{
		Name:       "detail",
		Role:       "detail",
		Truth:      "refinement",
		SampleRate: rt.cfg.SampleRate,
		FFTSize:    rt.cfg.FFTSize,
		CenterHz:   rt.cfg.CenterHz,
		SpanHz:     levelSpan,
		Source:     "detail-spectrum",
	}
	input := pipeline.RefinementInput{
		Level:      level,
		Detail:     detailLevel,
		Context:    surv.Context,
		Request:    pipeline.RefinementRequest{Strategy: plan.Strategy, Reason: "surveillance-plan", SpanHintHz: levelSpan},
		Budgets:    pipeline.BudgetModelFromPolicy(policy),
		Candidates: append([]pipeline.Candidate(nil), surv.Candidates...),
		Scheduled:  scheduled,
		WorkItems:  workItems,
		Plan:       plan,
		Windows:    windows,
		SampleRate: rt.cfg.SampleRate,
		FFTSize:    rt.cfg.FFTSize,
		CenterHz:   rt.cfg.CenterHz,
		Source:     "surveillance-detector",
	}
	input.Context.Refinement = level
	input.Context.Detail = detailLevel
	if !policy.RefinementEnabled {
		input.Scheduled = nil
		input.WorkItems = nil
		input.Request.Reason = pipeline.RefinementReasonDisabled
	}
	return input
}

func (rt *dspRuntime) runRefinement(art *spectrumArtifacts, surv pipeline.SurveillanceResult, extractMgr *extractionManager, rec *recorder.Manager) pipeline.RefinementStep {
	input := rt.buildRefinementInput(surv)
	result := rt.refineSignals(art, input, extractMgr, rec)
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
	refined := pipeline.RefineCandidates(selectedCandidates, input.Windows, art.detailSpectrum, sampleRate, fftSize, snips, snipRates, classifier.ClassifierMode(rt.cfg.ClassifierMode))
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
			pll := classifier.PLLResult{}
			if i < len(snips) && snips[i] != nil && len(snips[i]) > 256 {
				pll = classifier.EstimateExactFrequency(snips[i], snipRate, signals[i].CenterHz, cls.ModType)
				cls.PLL = &pll
				signals[i].PLL = &pll
				if cls.ModType == classifier.ClassWFM && pll.Stereo {
					cls.ModType = classifier.ClassWFMStereo
				}
			}
			if (cls.ModType == classifier.ClassWFM || cls.ModType == classifier.ClassWFMStereo) && rec != nil {
				rt.updateRDS(art.now, rec, &signals[i], cls)
			}
		}
	}
	budget := pipeline.BudgetModelFromPolicy(policy)
	queueStats := rt.decisionQueues.Apply(decisions, budget, art.now, policy)
	rt.queueStats = queueStats
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

func (rt *dspRuntime) updateRDS(now time.Time, rec *recorder.Manager, sig *detector.Signal, cls *classifier.Classification) {
	if sig == nil || cls == nil {
		return
	}
	keyHz := sig.CenterHz
	if sig.PLL != nil && sig.PLL.ExactHz != 0 {
		keyHz = sig.PLL.ExactHz
	}
	key := int64(math.Round(keyHz / 25000.0))
	st := rt.rdsMap[key]
	if st == nil {
		st = &rdsState{}
		rt.rdsMap[key] = st
	}
	if now.Sub(st.lastDecode) >= 4*time.Second && atomic.LoadInt32(&st.busy) == 0 {
		st.lastDecode = now
		atomic.StoreInt32(&st.busy, 1)
		go func(st *rdsState, sigHz float64) {
			defer atomic.StoreInt32(&st.busy, 0)
			ringIQ, ringSR, ringCenter := rec.SliceRecent(4.0)
			if len(ringIQ) < ringSR || ringSR <= 0 {
				return
			}
			offset := sigHz - ringCenter
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
			decimated := dsp.Decimate(f2, decim2)
			actualRate := rate1 / decim2
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
	st.mu.Unlock()
	if ps != "" && sig.PLL != nil {
		sig.PLL.RDSStation = strings.TrimSpace(ps)
		cls.PLL = sig.PLL
	}
}

func (rt *dspRuntime) maintenance(displaySignals []detector.Signal, rec *recorder.Manager) {
	if len(rt.rdsMap) > 0 {
		activeIDs := make(map[int64]bool, len(displaySignals))
		for _, s := range displaySignals {
			keyHz := s.CenterHz
			if s.PLL != nil && s.PLL.ExactHz != 0 {
				keyHz = s.PLL.ExactHz
			}
			activeIDs[int64(math.Round(keyHz/25000.0))] = true
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

func surveillanceLevels(policy pipeline.Policy, primary pipeline.AnalysisLevel, secondary pipeline.AnalysisLevel, presentation pipeline.AnalysisLevel) ([]pipeline.AnalysisLevel, pipeline.AnalysisContext) {
	levels := []pipeline.AnalysisLevel{primary}
	context := pipeline.AnalysisContext{
		Surveillance: primary,
		Presentation: presentation,
	}
	strategy := strings.ToLower(strings.TrimSpace(policy.SurveillanceStrategy))
	switch strategy {
	case "multi-res", "multi-resolution", "multi", "multi_res":
		if secondary.SampleRate != primary.SampleRate || secondary.FFTSize != primary.FFTSize {
			levels = append(levels, secondary)
			context.Derived = append(context.Derived, secondary)
		}
	}
	return levels, context
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

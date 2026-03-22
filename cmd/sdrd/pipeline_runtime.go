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
	streamPhaseState map[int64]*streamExtractState
	streamOverlap    *streamIQOverlap
	arbiter          *pipeline.Arbiter
	arbitration      pipeline.ArbitrationState
	gotSamples       bool
}

type spectrumArtifacts struct {
	allIQ                []complex64
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

type surveillanceLevelSpec struct {
	Level    pipeline.AnalysisLevel
	Decim    int
	AllowGPU bool
}

type surveillancePlan struct {
	Primary      pipeline.AnalysisLevel
	Levels       []pipeline.AnalysisLevel
	LevelSet     pipeline.SurveillanceLevelSet
	Presentation pipeline.AnalysisLevel
	Context      pipeline.AnalysisContext
	Specs        []surveillanceLevelSpec
}

func newDSPRuntime(cfg config.Config, det *detector.Detector, window []float64, gpuState *gpuStatus) *dspRuntime {
	detailFFT := cfg.Refinement.DetailFFTSize
	if detailFFT <= 0 {
		detailFFT = cfg.FFTSize
	}
	rt := &dspRuntime{
		cfg:              cfg,
		det:              det,
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
	return rt.spectrumFromIQWithPlan(iq, rt.window, rt.plan, gpuState, true)
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

func (rt *dspRuntime) captureSpectrum(srcMgr *sourceManager, rec *recorder.Manager, dcBlocker *dsp.DCBlocker, gpuState *gpuStatus) (*spectrumArtifacts, error) {
	required := rt.cfg.FFTSize
	if rt.detailFFT > required {
		required = rt.detailFFT
	}
	available := required
	st := srcMgr.Stats()
	if st.BufferSamples > required {
		available = (st.BufferSamples / required) * required
		if available < required {
			available = required
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
	detailIQ := survIQ
	if rt.detailFFT > 0 && len(allIQ) >= rt.detailFFT {
		detailIQ = allIQ[len(allIQ)-rt.detailFFT:]
	}
	if rt.dcEnabled {
		dcBlocker.Apply(allIQ)
	}
	if rt.iqEnabled {
		dsp.IQBalance(survIQ)
		if !sameIQBuffer(detailIQ, survIQ) {
			detailIQ = append([]complex64(nil), detailIQ...)
			dsp.IQBalance(detailIQ)
		}
	}
	survSpectrum := rt.spectrumFromIQ(survIQ, gpuState)
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
	return &spectrumArtifacts{
		allIQ:                allIQ,
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
	candidates := pipeline.CandidatesFromSignalsWithLevel(art.detected, "surveillance-detector", plan.Primary)
	scheduled := pipeline.ScheduleCandidates(candidates, policy)
	return pipeline.SurveillanceResult{
		Level:        plan.Primary,
		Levels:       plan.Levels,
		LevelSet:     plan.LevelSet,
		DisplayLevel: plan.Presentation,
		Context:      plan.Context,
		Spectra:      art.surveillanceSpectra,
		Candidates:   candidates,
		Scheduled:    scheduled,
		Finished:     art.finished,
		Signals:      art.detected,
		NoiseFloor:   art.noiseFloor,
		Thresholds:   art.thresholds,
	}
}

func (rt *dspRuntime) buildRefinementInput(surv pipeline.SurveillanceResult, now time.Time) pipeline.RefinementInput {
	policy := pipeline.PolicyFromConfig(rt.cfg)
	plan := pipeline.BuildRefinementPlan(surv.Candidates, policy)
	admission := rt.arbiter.AdmitRefinement(plan, policy, now)
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
		Budgets:    pipeline.BudgetModelFromPolicy(policy),
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
	primary := analysisLevel("surveillance", "surveillance", "surveillance", baseRate, baseFFT, rt.cfg.CenterHz, span, "baseband", 1, baseRate)
	levels := []pipeline.AnalysisLevel{primary}
	specs := []surveillanceLevelSpec{{Level: primary, Decim: 1, AllowGPU: true}}
	context := pipeline.AnalysisContext{Surveillance: primary}
	derivedLevels := make([]pipeline.AnalysisLevel, 0, 2)

	strategy := strings.ToLower(strings.TrimSpace(policy.SurveillanceStrategy))
	switch strategy {
	case "multi-res", "multi-resolution", "multi", "multi_res":
		decim := 2
		derivedRate := baseRate / decim
		derivedFFT := baseFFT / decim
		if derivedRate >= 200000 && derivedFFT >= 256 {
			derivedSpan := spanForPolicy(policy, float64(derivedRate))
			derived := analysisLevel("surveillance-lowres", "surveillance-lowres", "surveillance", derivedRate, derivedFFT, rt.cfg.CenterHz, derivedSpan, "decimated", decim, baseRate)
			levels = append(levels, derived)
			specs = append(specs, surveillanceLevelSpec{Level: derived, Decim: decim, AllowGPU: false})
			context.Derived = append(context.Derived, derived)
			derivedLevels = append(derivedLevels, derived)
		}
	}

	presentation := analysisLevel("presentation", "presentation", "presentation", baseRate, rt.cfg.Surveillance.DisplayBins, rt.cfg.CenterHz, span, "display", 1, baseRate)
	context.Presentation = presentation
	levelSet := pipeline.SurveillanceLevelSet{
		Primary:      primary,
		Derived:      append([]pipeline.AnalysisLevel(nil), derivedLevels...),
		Presentation: presentation,
	}
	allLevels := make([]pipeline.AnalysisLevel, 0, 1+len(derivedLevels)+1)
	allLevels = append(allLevels, primary)
	allLevels = append(allLevels, derivedLevels...)
	if presentation.Name != "" {
		allLevels = append(allLevels, presentation)
	}
	levelSet.All = allLevels

	return surveillancePlan{
		Primary:      primary,
		Levels:       levels,
		LevelSet:     levelSet,
		Presentation: presentation,
		Context:      context,
		Specs:        specs,
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

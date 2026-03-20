package detector

import (
	"math"
	"sort"
	"time"

	"sdr-visual-suite/internal/cfar"
	"sdr-visual-suite/internal/classifier"
	"sdr-visual-suite/internal/config"
)

type Event struct {
	ID        int64                      `json:"id"`
	Start     time.Time                  `json:"start"`
	End       time.Time                  `json:"end"`
	CenterHz  float64                    `json:"center_hz"`
	Bandwidth float64                    `json:"bandwidth_hz"`
	PeakDb    float64                    `json:"peak_db"`
	SNRDb     float64                    `json:"snr_db"`
	FirstBin  int                        `json:"first_bin"`
	LastBin   int                        `json:"last_bin"`
	Class     *classifier.Classification `json:"class,omitempty"`
}

type Detector struct {
	ThresholdDb     float64
	MinDuration     time.Duration
	Hold            time.Duration
	EmaAlpha        float64
	HysteresisDb    float64
	MinStableFrames int
	GapTolerance    time.Duration
	CFARScaleDb     float64
	EdgeMarginDb    float64
	MaxSignalBwHz   float64
	MergeGapHz      float64
	classHistorySize int
	classSwitchRatio float64
	binWidth        float64
	nbins           int
	sampleRate      int

	ema            []float64
	active         map[int64]*activeEvent
	nextID         int64
	cfarEngine     cfar.CFAR
	lastThresholds []float64
	lastNoiseFloor float64
	lastProcessTime time.Time
}

type activeEvent struct {
	id         int64
	start      time.Time
	lastSeen   time.Time
	centerHz   float64
	bwHz       float64
	peakDb     float64
	snrDb      float64
	firstBin   int
	lastBin    int
	class        *classifier.Classification
	stableHits   int
	missedFrames int // Consecutive frames without a matching raw signal
	classHistory []classifier.SignalClass
	classIdx     int
}

type Signal struct {
	ID       int64                       `json:"id"`
	FirstBin int                        `json:"first_bin"`
	LastBin  int                        `json:"last_bin"`
	CenterHz float64                    `json:"center_hz"`
	BWHz     float64                    `json:"bw_hz"`
	PeakDb   float64                    `json:"peak_db"`
	SNRDb    float64                    `json:"snr_db"`
	NoiseDb  float64                    `json:"noise_db,omitempty"`
	Class    *classifier.Classification `json:"class,omitempty"`
	PLL      *classifier.PLLResult      `json:"pll,omitempty"`
}

func New(detCfg config.DetectorConfig, sampleRate int, fftSize int) *Detector {
	minDur := time.Duration(detCfg.MinDurationMs) * time.Millisecond
	hold := time.Duration(detCfg.HoldMs) * time.Millisecond
	gapTolerance := time.Duration(detCfg.GapToleranceMs) * time.Millisecond
	emaAlpha := detCfg.EmaAlpha
	hysteresis := detCfg.HysteresisDb
	minStable := detCfg.MinStableFrames
	cfarMode := detCfg.CFARMode
	binWidth := float64(sampleRate) / float64(fftSize)
	cfarGuard := int(math.Ceil(detCfg.CFARGuardHz / binWidth))
	if cfarGuard < 0 {
		cfarGuard = 0
	}
	cfarTrain := int(math.Ceil(detCfg.CFARTrainHz / binWidth))
	if cfarTrain < 1 {
		cfarTrain = 1
	}
	cfarRank := detCfg.CFARRank
	cfarScaleDb := detCfg.CFARScaleDb
	cfarWrap := detCfg.CFARWrapAround
	thresholdDb := detCfg.ThresholdDb
	edgeMarginDb := detCfg.EdgeMarginDb
	maxSignalBwHz := detCfg.MaxSignalBwHz
	mergeGapHz := detCfg.MergeGapHz
	classHistorySize := detCfg.ClassHistorySize
	classSwitchRatio := detCfg.ClassSwitchRatio

	if minDur <= 0 {
		minDur = 250 * time.Millisecond
	}
	if hold <= 0 {
		hold = 500 * time.Millisecond
	}
	if emaAlpha <= 0 || emaAlpha > 1 {
		emaAlpha = 0.2
	}
	if hysteresis <= 0 {
		hysteresis = 3
	}
	if minStable <= 0 {
		minStable = 3
	}
	if gapTolerance <= 0 {
		gapTolerance = hold
	}
	if cfarScaleDb <= 0 {
		cfarScaleDb = 6
	}
	if edgeMarginDb <= 0 {
		edgeMarginDb = 3.0
	}
	if maxSignalBwHz <= 0 {
		maxSignalBwHz = 150000
	}
	if mergeGapHz <= 0 {
		mergeGapHz = 5000
	}
	if classHistorySize <= 0 {
		classHistorySize = 10
	}
	if classSwitchRatio <= 0 || classSwitchRatio > 1 {
		classSwitchRatio = 0.6
	}
	if cfarRank <= 0 || cfarRank > 2*cfarTrain {
		cfarRank = int(math.Round(0.75 * float64(2*cfarTrain)))
		if cfarRank <= 0 {
			cfarRank = 1
		}
	}
	var cfarEngine cfar.CFAR
	if cfarMode != "" && cfarMode != "OFF" {
		cfarEngine = cfar.New(cfar.Config{
			Mode:       cfar.Mode(cfarMode),
			GuardCells: cfarGuard,
			TrainCells: cfarTrain,
			Rank:       cfarRank,
			ScaleDb:    cfarScaleDb,
			WrapAround: cfarWrap,
		})
	}
	return &Detector{
		ThresholdDb:     thresholdDb,
		MinDuration:     minDur,
		Hold:            hold,
		EmaAlpha:        emaAlpha,
		HysteresisDb:    hysteresis,
		MinStableFrames: minStable,
		GapTolerance:    gapTolerance,
		CFARScaleDb:     cfarScaleDb,
		EdgeMarginDb:    edgeMarginDb,
		MaxSignalBwHz:   maxSignalBwHz,
		MergeGapHz:      mergeGapHz,
		classHistorySize: classHistorySize,
		classSwitchRatio: classSwitchRatio,
		binWidth:        float64(sampleRate) / float64(fftSize),
		nbins:           fftSize,
		sampleRate:      sampleRate,
		ema:             make([]float64, fftSize),
		active:          map[int64]*activeEvent{},
		nextID:          1,
		cfarEngine:      cfarEngine,
	}
}

func (d *Detector) Process(now time.Time, spectrum []float64, centerHz float64) ([]Event, []Signal) {
	// Compute frame-rate adaptive alpha for consistent smoothing regardless of fps
	dt := now.Sub(d.lastProcessTime).Seconds()
	if d.lastProcessTime.IsZero() || dt <= 0 || dt > 1.0 {
		dt = 1.0 / 15.0
	}
	d.lastProcessTime = now
	dtRef := 1.0 / 15.0
	ratio := dt / dtRef
	adaptiveAlpha := 1.0 - math.Pow(1.0-d.EmaAlpha, ratio)
	if adaptiveAlpha < 0.01 {
		adaptiveAlpha = 0.01
	}
	if adaptiveAlpha > 0.99 {
		adaptiveAlpha = 0.99
	}

	signals := d.detectSignals(spectrum, centerHz, adaptiveAlpha)
	finished := d.matchSignals(now, signals, adaptiveAlpha)
	return finished, signals
}

func (d *Detector) LastThresholds() []float64 {
	if len(d.lastThresholds) == 0 {
		return nil
	}
	return append([]float64(nil), d.lastThresholds...)
}

func (d *Detector) LastNoiseFloor() float64 {
	return d.lastNoiseFloor
}

func (ev *activeEvent) updateClass(newCls *classifier.Classification, historySize int, switchRatio float64) {
	if newCls == nil {
		return
	}
	if historySize <= 0 {
		historySize = 10
	}
	if switchRatio <= 0 || switchRatio > 1 {
		switchRatio = 0.6
	}
	if len(ev.classHistory) != historySize {
		ev.classHistory = make([]classifier.SignalClass, historySize)
		ev.classIdx = 0
	}
	ev.classHistory[ev.classIdx%len(ev.classHistory)] = newCls.ModType
	ev.classIdx++
	if ev.class == nil {
		clone := *newCls
		ev.class = &clone
		return
	}
	counts := map[classifier.SignalClass]int{}
	filled := ev.classIdx
	if filled > len(ev.classHistory) {
		filled = len(ev.classHistory)
	}
	for i := 0; i < filled; i++ {
		c := ev.classHistory[i]
		if c != "" {
			counts[c]++
		}
	}
	var majority classifier.SignalClass
	majorityCount := 0
	for c, n := range counts {
		if n > majorityCount {
			majority = c
			majorityCount = n
		}
	}
	threshold := int(math.Ceil(float64(filled) * switchRatio))
	if threshold < 1 {
		threshold = 1
	}
	if majorityCount >= threshold && majority != ev.class.ModType {
		clone := *newCls
		clone.ModType = majority
		ev.class = &clone
	} else if majority == ev.class.ModType && newCls.Confidence > ev.class.Confidence {
		ev.class.Confidence = newCls.Confidence
		ev.class.Features = newCls.Features
		ev.class.SecondBest = newCls.SecondBest
		ev.class.Scores = newCls.Scores
	}
	// Always update PLL — RDS station name accumulates over time
	if newCls.PLL != nil {
		ev.class.PLL = newCls.PLL
	}
}

func (d *Detector) UpdateClasses(signals []Signal) {
	for _, s := range signals {
		for _, ev := range d.active {
			if overlapHz(s.CenterHz, s.BWHz, ev.centerHz, ev.bwHz) && math.Abs(s.CenterHz-ev.centerHz) < (s.BWHz+ev.bwHz)/2.0 {
				if s.Class != nil {
					ev.updateClass(s.Class, d.classHistorySize, d.classSwitchRatio)
				}
			}
		}
	}
}

// StableSignals returns the smoothed active events as a Signal list for frontend display.
// Only events that have been seen for at least MinStableFrames are included.
// Output is sorted by CenterHz for consistent ordering across frames.
func (d *Detector) StableSignals() []Signal {
	var out []Signal
	for _, ev := range d.active {
		if ev.stableHits < d.MinStableFrames {
			continue
		}
		sig := Signal{
			ID:       ev.id,
			FirstBin: ev.firstBin,
			LastBin:  ev.lastBin,
			CenterHz: ev.centerHz,
			BWHz:     ev.bwHz,
			PeakDb:   ev.peakDb,
			SNRDb:    ev.snrDb,
			Class:    ev.class,
		}
		if ev.class != nil && ev.class.PLL != nil {
			sig.PLL = ev.class.PLL
		}
		out = append(out, sig)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CenterHz < out[j].CenterHz })
	return out
}

func (d *Detector) detectSignals(spectrum []float64, centerHz float64, adaptiveAlpha float64) []Signal {
	n := len(spectrum)
	if n == 0 {
		return nil
	}
	smooth := d.smoothSpectrum(spectrum, adaptiveAlpha)
	var thresholds []float64
	if d.cfarEngine != nil {
		thresholds = d.cfarEngine.Thresholds(smooth)
	}
	d.lastThresholds = append(d.lastThresholds[:0], thresholds...)
	noiseGlobal := median(smooth)
	d.lastNoiseFloor = noiseGlobal
	var signals []Signal
	in := false
	start := 0
	peak := -1e9
	peakBin := 0
	for i := 0; i < n; i++ {
		v := smooth[i]
		thresholdOn := d.ThresholdDb
		if thresholds != nil && !math.IsNaN(thresholds[i]) {
			thresholdOn = thresholds[i]
		}
		thresholdOff := thresholdOn - d.HysteresisDb
		if v >= thresholdOn {
			if !in {
				in = true
				start = i
				peak = v
				peakBin = i
			} else if v > peak {
				peak = v
				peakBin = i
			}
		} else if in && v < thresholdOff {
			noise := noiseGlobal
			if thresholds != nil && peakBin >= 0 && peakBin < len(thresholds) && !math.IsNaN(thresholds[peakBin]) {
				noise = thresholds[peakBin] - d.CFARScaleDb
			}
			signals = append(signals, d.makeSignal(start, i-1, peak, peakBin, noise, centerHz, smooth))
			in = false
		}
	}
	if in {
		noise := noiseGlobal
		if thresholds != nil && peakBin >= 0 && peakBin < len(thresholds) && !math.IsNaN(thresholds[peakBin]) {
			noise = thresholds[peakBin] - d.CFARScaleDb
		}
		signals = append(signals, d.makeSignal(start, n-1, peak, peakBin, noise, centerHz, smooth))
	}
	signals = d.expandSignalEdges(signals, smooth, noiseGlobal, centerHz)
	for i := range signals {
		// Use power-weighted centroid for accurate center frequency
		signals[i].CenterHz = d.powerWeightedCenter(smooth, signals[i].FirstBin, signals[i].LastBin, centerHz)
		signals[i].BWHz = float64(signals[i].LastBin-signals[i].FirstBin+1) * d.binWidth
	}
	return signals
}

func (d *Detector) expandSignalEdges(signals []Signal, smooth []float64, noiseFloor float64, centerHz float64) []Signal {
	n := len(smooth)
	if n == 0 || len(signals) == 0 {
		return signals
	}
	margin := d.EdgeMarginDb
	if margin <= 0 {
		margin = 3.0
	}
	maxExpansionBins := int(d.MaxSignalBwHz / d.binWidth)
	if maxExpansionBins < 10 {
		maxExpansionBins = 10
	}
	for i := range signals {
		seed := signals[i]
		peakDb := seed.PeakDb
		localNoise := noiseFloor
		leftProbe := seed.FirstBin - 50
		rightProbe := seed.LastBin + 50
		if leftProbe >= 0 && rightProbe < n {
			leftNoise := minInRange(smooth, maxInt(0, leftProbe), maxInt(0, seed.FirstBin-5))
			rightNoise := minInRange(smooth, minInt(n-1, seed.LastBin+5), minInt(n-1, rightProbe))
			localNoise = math.Min(leftNoise, rightNoise)
		}
		edgeThreshold := localNoise + margin
		newFirst := seed.FirstBin
		prevVal := smooth[newFirst]
		for j := 0; j < maxExpansionBins; j++ {
			next := newFirst - 1
			if next < 0 {
				break
			}
			val := smooth[next]
			if val <= edgeThreshold {
				break
			}
			if val > prevVal+1.0 && val < peakDb-6.0 {
				break
			}
			prevVal = val
			newFirst = next
		}
		newLast := seed.LastBin
		prevVal = smooth[newLast]
		for j := 0; j < maxExpansionBins; j++ {
			next := newLast + 1
			if next >= n {
				break
			}
			val := smooth[next]
			if val <= edgeThreshold {
				break
			}
			if val > prevVal+1.0 && val < peakDb-6.0 {
				break
			}
			prevVal = val
			newLast = next
		}
		signals[i].FirstBin = newFirst
		signals[i].LastBin = newLast
		// CenterHz will be recalculated with power-weighted centroid after expansion
		signals[i].BWHz = float64(newLast-newFirst+1) * d.binWidth
	}
	signals = d.mergeOverlapping(signals, centerHz)
	return signals
}

func (d *Detector) mergeOverlapping(signals []Signal, centerHz float64) []Signal {
	if len(signals) <= 1 {
		return signals
	}
	gapBins := 0
	if d.MergeGapHz > 0 && d.binWidth > 0 {
		gapBins = int(math.Ceil(d.MergeGapHz / d.binWidth))
	}
	sort.Slice(signals, func(i, j int) bool {
		return signals[i].FirstBin < signals[j].FirstBin
	})
	merged := []Signal{signals[0]}
	for i := 1; i < len(signals); i++ {
		last := &merged[len(merged)-1]
		cur := signals[i]
		gap := cur.FirstBin - last.LastBin - 1
		if gap <= gapBins {
			if cur.LastBin > last.LastBin {
				last.LastBin = cur.LastBin
			}
			if cur.PeakDb > last.PeakDb {
				last.PeakDb = cur.PeakDb
			}
			if cur.SNRDb > last.SNRDb {
				last.SNRDb = cur.SNRDb
			}
			centerBin := float64(last.FirstBin+last.LastBin) / 2.0
			last.BWHz = float64(last.LastBin-last.FirstBin+1) * d.binWidth
			last.CenterHz = d.centerFreqForBin(centerBin, centerHz)
			if cur.NoiseDb < last.NoiseDb || last.NoiseDb == 0 {
				last.NoiseDb = cur.NoiseDb
			}
		} else {
			merged = append(merged, cur)
		}
	}
	return merged
}

func (d *Detector) centerFreqForBin(bin float64, centerHz float64) float64 {
	return centerHz + (bin-float64(d.nbins)/2.0)*d.binWidth
}

// powerWeightedCenter computes the power-weighted centroid frequency within [first, last].
// This is more accurate than the midpoint because it accounts for asymmetric signal shapes.
func (d *Detector) powerWeightedCenter(spectrum []float64, first, last int, centerHz float64) float64 {
	if first > last || first < 0 || last >= len(spectrum) {
		centerBin := float64(first+last) / 2.0
		return d.centerFreqForBin(centerBin, centerHz)
	}
	// Convert dB to linear, compute weighted average bin
	var sumPower, sumWeighted float64
	for i := first; i <= last; i++ {
		p := math.Pow(10, spectrum[i]/10.0)
		sumPower += p
		sumWeighted += p * float64(i)
	}
	if sumPower <= 0 {
		centerBin := float64(first+last) / 2.0
		return d.centerFreqForBin(centerBin, centerHz)
	}
	centroidBin := sumWeighted / sumPower
	return d.centerFreqForBin(centroidBin, centerHz)
}

func minInRange(s []float64, from, to int) float64 {
	if len(s) == 0 {
		return 0
	}
	if from < 0 {
		from = 0
	}
	if to >= len(s) {
		to = len(s) - 1
	}
	if from > to {
		return s[minInt(maxInt(from, 0), len(s)-1)]
	}
	m := s[from]
	for i := from + 1; i <= to; i++ {
		if s[i] < m {
			m = s[i]
		}
	}
	return m
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func overlapHz(center1, bw1, center2, bw2 float64) bool {
	left1 := center1 - bw1/2
	right1 := center1 + bw1/2
	left2 := center2 - bw2/2
	right2 := center2 + bw2/2
	return left1 <= right2 && left2 <= right1
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	cp := append([]float64(nil), vals...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2
	}
	return cp[mid]
}

func (d *Detector) makeSignal(first, last int, peak float64, peakBin int, noise float64, centerHz float64, spectrum []float64) Signal {
	// Use peak bin for center frequency — more accurate than midpoint of first/last
	// because edge expansion can be asymmetric
	centerFreq := centerHz + (float64(peakBin)-float64(d.nbins)/2.0)*d.binWidth
	bw := float64(last-first+1) * d.binWidth
	snr := peak - noise
	return Signal{
		FirstBin: first,
		LastBin:  last,
		CenterHz: centerFreq,
		BWHz:     bw,
		PeakDb:   peak,
		SNRDb:    snr,
		NoiseDb:  noise,
	}
}

func (d *Detector) smoothSpectrum(spectrum []float64, alpha float64) []float64 {
	if d.ema == nil || len(d.ema) != len(spectrum) {
		d.ema = make([]float64, len(spectrum))
		copy(d.ema, spectrum)
		return d.ema
	}
	for i := range spectrum {
		v := spectrum[i]
		d.ema[i] = alpha*v + (1-alpha)*d.ema[i]
	}
	return d.ema
}

func (d *Detector) matchSignals(now time.Time, signals []Signal, adaptiveAlpha float64) []Event {
	used := make(map[int64]bool, len(d.active))
	signalUsed := make([]bool, len(signals))
	smoothAlpha := adaptiveAlpha

	// Sort active events by maturity (stableHits descending).
	// Mature events match FIRST, preventing ghost/new events from stealing their signals.
	// Without this, Go map iteration is random and a 1-frame-old ghost can steal
	// a signal from a 1000-frame-old stable event.
	type eventEntry struct {
		id int64
		ev *activeEvent
	}
	sortedEvents := make([]eventEntry, 0, len(d.active))
	for id, ev := range d.active {
		sortedEvents = append(sortedEvents, eventEntry{id, ev})
	}
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].ev.stableHits > sortedEvents[j].ev.stableHits
	})

	// Event-first matching: for each active event (mature first), find the closest unmatched raw signal.
	for _, entry := range sortedEvents {
		id, ev := entry.id, entry.ev
		bestIdx := -1
		bestDist := math.MaxFloat64
		for i, s := range signals {
			if signalUsed[i] {
				continue
			}
			// Use wider of raw and event BW for matching tolerance
			matchBW := math.Max(s.BWHz, ev.bwHz)
			if matchBW < 20000 {
				matchBW = 20000 // Minimum 20 kHz matching window
			}
			dist := math.Abs(s.CenterHz - ev.centerHz)
			if dist < matchBW && dist < bestDist {
				bestIdx = i
				bestDist = dist
			}
		}
		if bestIdx < 0 {
			continue
		}
		signalUsed[bestIdx] = true
		used[id] = true
		s := signals[bestIdx]
		ev.lastSeen = now
		ev.stableHits++
		ev.missedFrames = 0 // Reset miss counter on successful match
		ev.centerHz = smoothAlpha*s.CenterHz + (1-smoothAlpha)*ev.centerHz
		if ev.bwHz <= 0 {
			ev.bwHz = s.BWHz
		} else {
			ev.bwHz = smoothAlpha*s.BWHz + (1-smoothAlpha)*ev.bwHz
		}
		ev.peakDb = smoothAlpha*s.PeakDb + (1-smoothAlpha)*ev.peakDb
		ev.snrDb = smoothAlpha*s.SNRDb + (1-smoothAlpha)*ev.snrDb
		ev.firstBin = int(math.Round(smoothAlpha*float64(s.FirstBin) + (1-smoothAlpha)*float64(ev.firstBin)))
		ev.lastBin = int(math.Round(smoothAlpha*float64(s.LastBin) + (1-smoothAlpha)*float64(ev.lastBin)))
		if s.Class != nil {
			ev.updateClass(s.Class, d.classHistorySize, d.classSwitchRatio)
		}
	}

	// Create new events for unmatched raw signals
	for i, s := range signals {
		if signalUsed[i] {
			continue
		}
		id := d.nextID
		d.nextID++
		d.active[id] = &activeEvent{
			id:         id,
			start:      now,
			lastSeen:   now,
			centerHz:   s.CenterHz,
			bwHz:       s.BWHz,
			peakDb:     s.PeakDb,
			snrDb:      s.SNRDb,
			firstBin:   s.FirstBin,
			lastBin:    s.LastBin,
			class:      s.Class,
			stableHits: 1,
		}
	}

	var finished []Event
	for id, ev := range d.active {
		if used[id] {
			continue
		}
		// Event was NOT matched this frame — increment miss counter
		ev.missedFrames++

		// Proportional gap tolerance: mature events are much harder to kill.
		// A new event (stableHits=3) dies after GapTolerance (e.g. 500ms).
		// A mature event (stableHits=300, i.e. ~10 seconds) gets 10x GapTolerance.
		// This prevents FM broadcast events from dying during brief CFAR dips.
		maturityFactor := 1.0 + math.Log1p(float64(ev.stableHits)/10.0)
		if maturityFactor > 20.0 {
			maturityFactor = 20.0 // Cap at 20x base tolerance
		}
		effectiveTolerance := time.Duration(float64(d.GapTolerance) * maturityFactor)

		if now.Sub(ev.lastSeen) < effectiveTolerance {
			continue
		}
		duration := ev.lastSeen.Sub(ev.start)
		if duration < d.MinDuration || ev.stableHits < d.MinStableFrames {
			delete(d.active, id)
			continue
		}
		finished = append(finished, Event{
			ID:        ev.id,
			Start:     ev.start,
			End:       ev.lastSeen,
			CenterHz:  ev.centerHz,
			Bandwidth: ev.bwHz,
			PeakDb:    ev.peakDb,
			SNRDb:     ev.snrDb,
			FirstBin:  ev.firstBin,
			LastBin:   ev.lastBin,
			Class:     ev.class,
		})
		delete(d.active, id)
	}
	return finished
}

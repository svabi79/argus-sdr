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
	binWidth        float64
	nbins           int
	sampleRate      int

	ema            []float64
	active         map[int64]*activeEvent
	nextID         int64
	cfarEngine     cfar.CFAR
	lastThresholds []float64
	lastNoiseFloor float64
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
	class      *classifier.Classification
	stableHits int
}

type Signal struct {
	FirstBin int                        `json:"first_bin"`
	LastBin  int                        `json:"last_bin"`
	CenterHz float64                    `json:"center_hz"`
	BWHz     float64                    `json:"bw_hz"`
	PeakDb   float64                    `json:"peak_db"`
	SNRDb    float64                    `json:"snr_db"`
	NoiseDb  float64                    `json:"noise_db,omitempty"`
	Class    *classifier.Classification `json:"class,omitempty"`
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
	signals := d.detectSignals(spectrum, centerHz)
	finished := d.matchSignals(now, signals)
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

func (d *Detector) UpdateClasses(signals []Signal) {
	for _, s := range signals {
		for _, ev := range d.active {
			if overlapHz(s.CenterHz, s.BWHz, ev.centerHz, ev.bwHz) && math.Abs(s.CenterHz-ev.centerHz) < (s.BWHz+ev.bwHz)/2.0 {
				if s.Class != nil {
					if ev.class == nil || s.Class.Confidence >= ev.class.Confidence {
						ev.class = s.Class
					}
				}
			}
		}
	}
}

func (d *Detector) detectSignals(spectrum []float64, centerHz float64) []Signal {
	n := len(spectrum)
	if n == 0 {
		return nil
	}
	smooth := d.smoothSpectrum(spectrum)
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
		centerBin := float64(signals[i].FirstBin+signals[i].LastBin) / 2.0
		signals[i].CenterHz = d.centerFreqForBin(centerBin, centerHz)
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
		centerBin := float64(newFirst+newLast) / 2.0
		signals[i].CenterHz = d.centerFreqForBin(centerBin, centerHz)
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
	centerBin := float64(first+last) / 2.0
	centerFreq := centerHz + (centerBin-float64(d.nbins)/2.0)*d.binWidth
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

func (d *Detector) smoothSpectrum(spectrum []float64) []float64 {
	if d.ema == nil || len(d.ema) != len(spectrum) {
		d.ema = make([]float64, len(spectrum))
		copy(d.ema, spectrum)
		return d.ema
	}
	alpha := d.EmaAlpha
	for i := range spectrum {
		v := spectrum[i]
		d.ema[i] = alpha*v + (1-alpha)*d.ema[i]
	}
	return d.ema
}

func (d *Detector) matchSignals(now time.Time, signals []Signal) []Event {
	used := make(map[int64]bool, len(d.active))
	for _, s := range signals {
		var best *activeEvent
		var candidates []struct {
			ev   *activeEvent
			dist float64
		}
		for _, ev := range d.active {
			if overlapHz(s.CenterHz, s.BWHz, ev.centerHz, ev.bwHz) && math.Abs(s.CenterHz-ev.centerHz) < (s.BWHz+ev.bwHz)/2.0 {
				candidates = append(candidates, struct {
					ev   *activeEvent
					dist float64
				}{ev: ev, dist: math.Abs(s.CenterHz - ev.centerHz)})
			}
		}
		if len(candidates) > 0 {
			sort.Slice(candidates, func(i, j int) bool { return candidates[i].dist < candidates[j].dist })
			best = candidates[0].ev
		}
		if best == nil {
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
			continue
		}
		used[best.id] = true
		best.lastSeen = now
		best.stableHits++
		best.centerHz = (best.centerHz + s.CenterHz) / 2.0
		if s.BWHz > best.bwHz {
			best.bwHz = s.BWHz
		}
		if s.PeakDb > best.peakDb {
			best.peakDb = s.PeakDb
		}
		if s.SNRDb > best.snrDb {
			best.snrDb = s.SNRDb
		}
		if s.FirstBin < best.firstBin {
			best.firstBin = s.FirstBin
		}
		if s.LastBin > best.lastBin {
			best.lastBin = s.LastBin
		}
		if s.Class != nil {
			if best.class == nil || s.Class.Confidence >= best.class.Confidence {
				best.class = s.Class
			}
		}
	}

	var finished []Event
	for id, ev := range d.active {
		if used[id] {
			continue
		}
		if now.Sub(ev.lastSeen) < d.GapTolerance {
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

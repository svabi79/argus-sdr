package detector

import (
	"math"
	"sort"
	"time"

	"sdr-visual-suite/internal/classifier"
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
	ThresholdDb float64
	MinDuration time.Duration
	Hold        time.Duration

	binWidth   float64
	nbins      int
	sampleRate int

	active map[int64]*activeEvent
	nextID int64
}

type activeEvent struct {
	id       int64
	start    time.Time
	lastSeen time.Time
	centerHz float64
	bwHz     float64
	peakDb   float64
	snrDb    float64
	firstBin int
	lastBin  int
	class    *classifier.Classification
}

type Signal struct {
	FirstBin int                        `json:"first_bin"`
	LastBin  int                        `json:"last_bin"`
	CenterHz float64                    `json:"center_hz"`
	BWHz     float64                    `json:"bw_hz"`
	PeakDb   float64                    `json:"peak_db"`
	SNRDb    float64                    `json:"snr_db"`
	Class    *classifier.Classification `json:"class,omitempty"`
}

func New(thresholdDb float64, sampleRate int, fftSize int, minDur, hold time.Duration) *Detector {
	if minDur <= 0 {
		minDur = 250 * time.Millisecond
	}
	if hold <= 0 {
		hold = 500 * time.Millisecond
	}
	return &Detector{
		ThresholdDb: thresholdDb,
		MinDuration: minDur,
		Hold:        hold,
		binWidth:    float64(sampleRate) / float64(fftSize),
		nbins:       fftSize,
		sampleRate:  sampleRate,
		active:      map[int64]*activeEvent{},
		nextID:      1,
	}
}

func (d *Detector) Process(now time.Time, spectrum []float64, centerHz float64) ([]Event, []Signal) {
	signals := d.detectSignals(spectrum, centerHz)
	finished := d.matchSignals(now, signals)
	return finished, signals
}

// UpdateClasses refreshes active event classes from current signals.
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
	threshold := d.ThresholdDb
	noise := median(spectrum)
	var signals []Signal
	in := false
	start := 0
	peak := -1e9
	peakBin := 0
	for i := 0; i < n; i++ {
		v := spectrum[i]
		if v >= threshold {
			if !in {
				in = true
				start = i
				peak = v
				peakBin = i
			} else if v > peak {
				peak = v
				peakBin = i
			}
		} else if in {
			signals = append(signals, d.makeSignal(start, i-1, peak, peakBin, noise, centerHz, spectrum))
			in = false
		}
	}
	if in {
		signals = append(signals, d.makeSignal(start, n-1, peak, peakBin, noise, centerHz, spectrum))
	}
	return signals
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
	}
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
				id:       id,
				start:    now,
				lastSeen: now,
				centerHz: s.CenterHz,
				bwHz:     s.BWHz,
				peakDb:   s.PeakDb,
				snrDb:    s.SNRDb,
				firstBin: s.FirstBin,
				lastBin:  s.LastBin,
				class:    s.Class,
			}
			continue
		}
		used[best.id] = true
		best.lastSeen = now
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
		if now.Sub(ev.lastSeen) < d.Hold {
			continue
		}
		duration := ev.lastSeen.Sub(ev.start)
		if duration < d.MinDuration {
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

func overlapHz(c1, b1, c2, b2 float64) bool {
	l1 := c1 - b1/2.0
	r1 := c1 + b1/2.0
	l2 := c2 - b2/2.0
	r2 := c2 + b2/2.0
	return l1 <= r2 && l2 <= r1
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	cpy := append([]float64(nil), vals...)
	sort.Float64s(cpy)
	mid := len(cpy) / 2
	if len(cpy)%2 == 0 {
		return (cpy[mid-1] + cpy[mid]) / 2.0
	}
	return cpy[mid]
}

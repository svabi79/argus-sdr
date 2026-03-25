package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/demod/gpudemod"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/dsp"
	"sdr-wideband-suite/internal/logging"
	"sdr-wideband-suite/internal/telemetry"
)

func mustParseDuration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	return fallback
}

func buildDecoderMap(cfg config.Config) map[string]string {
	out := map[string]string{}
	if cfg.Decoder.FT8Cmd != "" {
		out["FT8"] = cfg.Decoder.FT8Cmd
	}
	if cfg.Decoder.WSPRCmd != "" {
		out["WSPR"] = cfg.Decoder.WSPRCmd
	}
	if cfg.Decoder.DMRCmd != "" {
		out["DMR"] = cfg.Decoder.DMRCmd
	}
	if cfg.Decoder.DStarCmd != "" {
		out["D-STAR"] = cfg.Decoder.DStarCmd
	}
	if cfg.Decoder.FSKCmd != "" {
		out["FSK"] = cfg.Decoder.FSKCmd
	}
	if cfg.Decoder.PSKCmd != "" {
		out["PSK"] = cfg.Decoder.PSKCmd
	}
	return out
}

func decoderKeys(cfg config.Config) []string {
	m := buildDecoderMap(cfg)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (m *extractionManager) reset() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runner != nil {
		m.runner.Close()
		m.runner = nil
	}
}

func (m *extractionManager) get(sampleCount int, sampleRate int) *gpudemod.BatchRunner {
	if m == nil || sampleCount <= 0 || sampleRate <= 0 || !gpudemod.Available() {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runner != nil && sampleCount > m.maxSamples {
		m.runner.Close()
		m.runner = nil
	}
	if m.runner == nil {
		// Allocate generously: enough for full allIQ (sampleRate/10 ≈ 100ms)
		// so the runner never needs re-allocation when used for both
		// classification (FFT-block ~65k) and streaming (allIQ ~273k+).
		allocSize := sampleCount
		generous := sampleRate/10 + 1024 // ~400k at 4MHz — covers any scenario
		if generous > allocSize {
			allocSize = generous
		}
		if r, err := gpudemod.NewBatchRunner(allocSize, sampleRate); err == nil {
			m.runner = r
			m.maxSamples = allocSize
		} else {
			log.Printf("gpudemod: batch runner init failed: %v", err)
		}
		return m.runner
	}
	return m.runner
}

func extractSignalIQ(iq []complex64, sampleRate int, centerHz float64, sigHz float64, bwHz float64) []complex64 {
	if len(iq) == 0 || sampleRate <= 0 {
		return nil
	}
	results, _ := extractSignalIQBatch(nil, iq, sampleRate, centerHz, []detector.Signal{{CenterHz: sigHz, BWHz: bwHz}})
	if len(results) == 0 {
		return nil
	}
	return results[0]
}

func extractSignalIQBatch(extractMgr *extractionManager, iq []complex64, sampleRate int, centerHz float64, signals []detector.Signal) ([][]complex64, []int) {
	out := make([][]complex64, len(signals))
	rates := make([]int, len(signals))
	if len(iq) == 0 || sampleRate <= 0 || len(signals) == 0 {
		return out, rates
	}
	decimTarget := 200000
	if decimTarget <= 0 {
		decimTarget = sampleRate
	}

	runner := extractMgr.get(len(iq), sampleRate)
	if runner != nil {
		jobs := make([]gpudemod.ExtractJob, len(signals))
		for i, sig := range signals {
			bw := sig.BWHz
			sigMHz := sig.CenterHz / 1e6
			isWFM := (sigMHz >= 87.5 && sigMHz <= 108.0) || (sig.Class != nil && (sig.Class.ModType == "WFM" || sig.Class.ModType == "WFM_STEREO"))
			jobOutRate := decimTarget
			if isWFM {
				jobOutRate = wfmStreamOutRate
			}
			// Minimum extraction BW: ensure enough bandwidth for demod features
			// FM broadcast (87.5-108 MHz) needs >=250kHz for stereo pilot + RDS at 57kHz
			// Also widen for any signal classified as WFM (in case of re-extraction)
			if isWFM {
				if bw < wfmStreamMinBW {
					bw = wfmStreamMinBW
				}
			} else if bw < 20000 {
				bw = 20000
			}
			jobs[i] = gpudemod.ExtractJob{OffsetHz: sig.CenterHz - centerHz, BW: bw, OutRate: jobOutRate}
		}
		if gpuOuts, gpuRates, err := runner.ShiftFilterDecimateBatch(iq, jobs); err == nil && len(gpuOuts) == len(signals) {
			// batch extraction OK (silent)
			for i := range gpuOuts {
				out[i] = gpuOuts[i]
				if i < len(gpuRates) {
					rates[i] = gpuRates[i]
				}
			}
			return out, rates
		} else if err != nil {
			log.Printf("gpudemod: batch extraction failed for %d signals: %v", len(signals), err)
		}
	}

	// CPU extraction fallback (silent — see batch extraction failed above if applicable)
	for i, sig := range signals {
		offset := sig.CenterHz - centerHz
		shifted := dsp.FreqShift(iq, sampleRate, offset)
		bw := sig.BWHz
		// FM broadcast (87.5-108 MHz) needs >=250kHz for stereo + RDS
		sigMHz := sig.CenterHz / 1e6
		isWFM := (sigMHz >= 87.5 && sigMHz <= 108.0) || (sig.Class != nil && (sig.Class.ModType == "WFM" || sig.Class.ModType == "WFM_STEREO"))
		if isWFM {
			if bw < wfmStreamMinBW {
				bw = wfmStreamMinBW
			}
		} else if bw < 20000 {
			bw = 20000
		}
		cutoff := bw / 2
		if cutoff < 200 {
			cutoff = 200
		}
		if cutoff > float64(sampleRate)/2-1 {
			cutoff = float64(sampleRate)/2 - 1
		}
		taps := dsp.LowpassFIR(cutoff, sampleRate, 101)
		filtered := dsp.ApplyFIR(shifted, taps)
		decim := sampleRate / decimTarget
		if decim < 1 {
			decim = 1
		}
		out[i] = dsp.Decimate(filtered, decim)
		rates[i] = sampleRate / decim
	}
	return out, rates
}

func parseSince(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	if ms, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if ms > 1e12 {
			return time.UnixMilli(ms), nil
		}
		return time.Unix(ms, 0), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, raw)
}

// streamExtractState holds per-signal persistent state for phase-continuous
// GPU extraction. Stored in the DSP loop, keyed by signal ID.
type streamExtractState struct {
	phase float64 // FreqShift phase accumulator
}

// streamIQOverlap holds the tail of the previous allIQ for FIR halo prepend.
type streamIQOverlap struct {
	tail []complex64
}

// extractionConfig holds audio quality settings for signal extraction.
type extractionConfig struct {
	firTaps   int     // AQ-3: FIR tap count (default 101)
	bwMult    float64 // AQ-5: BW multiplier (default 1.2)
}

const streamOverlapLen = 512 // must be >= FIR tap count with margin
const (
	wfmStreamOutRate = 500000
	wfmStreamMinBW   = 250000
)

var forceCPUStreamExtract = func() bool {
	raw := strings.TrimSpace(os.Getenv("SDR_FORCE_CPU_STREAM_EXTRACT"))
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return v
}()

// extractForStreaming performs GPU-accelerated extraction with:
//   - Per-signal phase-continuous FreqShift (via PhaseStart in ExtractJob)
//   - IQ overlap prepended to allIQ so FIR kernel has real data in halo
//
// Returns extracted snippets with overlap trimmed, and updates phase state.
func extractForStreaming(
	extractMgr *extractionManager,
	allIQ []complex64,
	sampleRate int,
	centerHz float64,
	signals []detector.Signal,
	phaseState map[int64]*streamExtractState,
	overlap *streamIQOverlap,
	aqCfg extractionConfig,
	coll *telemetry.Collector,
) ([][]complex64, []int) {
	out := make([][]complex64, len(signals))
	rates := make([]int, len(signals))
	if len(allIQ) == 0 || sampleRate <= 0 || len(signals) == 0 {
		return out, rates
	}

	// AQ-3: Use configured overlap length (must cover FIR taps)
	overlapNeeded := streamOverlapLen
	if aqCfg.firTaps > 0 && aqCfg.firTaps+64 > overlapNeeded {
		overlapNeeded = aqCfg.firTaps + 64
	}

	// Prepend overlap from previous frame so FIR kernel has real halo data
	var gpuIQ []complex64
	overlapLen := len(overlap.tail)
	logging.Debug("extract", "overlap", "len", overlapLen, "needed", overlapNeeded, "allIQ", len(allIQ))
	if overlapLen > 0 {
		gpuIQ = make([]complex64, overlapLen+len(allIQ))
		copy(gpuIQ, overlap.tail)
		copy(gpuIQ[overlapLen:], allIQ)
	} else {
		gpuIQ = allIQ
		overlapLen = 0
	}

	// Save tail for next frame (sized to cover configured FIR taps)
	if len(allIQ) > overlapNeeded {
		overlap.tail = append(overlap.tail[:0], allIQ[len(allIQ)-overlapNeeded:]...)
	} else {
		overlap.tail = append(overlap.tail[:0], allIQ...)
	}

	decimTarget := 200000

	// AQ-5: BW multiplier for extraction (wider = better S/N for weak signals)
	bwMult := aqCfg.bwMult
	if bwMult <= 0 {
		bwMult = 1.0
	}

	if coll != nil {
		coll.SetGauge("iq.extract.input.length", float64(len(allIQ)), nil)
		coll.SetGauge("iq.extract.input.overlap_length", float64(overlapLen), nil)
		headMean, tailMean, boundaryScore, _ := boundaryMetrics(overlap.tail, allIQ, 32)
		coll.SetGauge("iq.extract.input.head_mean_mag", headMean, nil)
		coll.SetGauge("iq.extract.input.prev_tail_mean_mag", tailMean, nil)
		coll.Observe("iq.extract.input.discontinuity_score", boundaryScore, nil)
	}

	rawBoundary := make(map[int64]boundaryProbeState, len(signals))
	trimmedBoundary := make(map[int64]boundaryProbeState, len(signals))

	// Build jobs with per-signal phase
	jobs := make([]gpudemod.ExtractJob, len(signals))
	for i, sig := range signals {
		bw := sig.BWHz * bwMult // AQ-5: widen extraction BW
		sigMHz := sig.CenterHz / 1e6
		isWFM := (sigMHz >= 87.5 && sigMHz <= 108.0) ||
			(sig.Class != nil && (sig.Class.ModType == "WFM" || sig.Class.ModType == "WFM_STEREO"))
		jobOutRate := decimTarget
		if isWFM {
			jobOutRate = wfmStreamOutRate
			if bw < wfmStreamMinBW {
				bw = wfmStreamMinBW
			}
		} else if bw < 20000 {
			bw = 20000
		}

		ps := phaseState[sig.ID]
		if ps == nil {
			ps = &streamExtractState{}
			phaseState[sig.ID] = ps
		}

		// PhaseStart is where the NEW data begins. But gpuIQ has overlap
		// prepended, so the GPU kernel starts processing at the overlap.
		// We need to rewind the phase by overlapLen samples so that the
		// overlap region gets the correct phase, and the new data region
		// starts at ps.phase exactly.
		phaseInc := -2.0 * math.Pi * (sig.CenterHz - centerHz) / float64(sampleRate)
		gpuPhaseStart := ps.phase - phaseInc*float64(overlapLen)

		jobs[i] = gpudemod.ExtractJob{
			OffsetHz:   sig.CenterHz - centerHz,
			BW:         bw,
			OutRate:    jobOutRate,
			PhaseStart: gpuPhaseStart,
		}
	}

	// Try GPU BatchRunner with phase unless CPU-only debug is forced.
	var runner *gpudemod.BatchRunner
	if forceCPUStreamExtract {
		logging.Warn("boundary", "force_cpu_stream_extract", "allIQ_len", len(allIQ), "gpuIQ_len", len(gpuIQ), "signals", len(signals))
	} else {
		runner = extractMgr.get(len(gpuIQ), sampleRate)
	}
	if runner != nil {
		results, err := runner.ShiftFilterDecimateBatchWithPhase(gpuIQ, jobs)
		if err == nil && len(results) == len(signals) {
			for i, res := range results {
				outRate := res.Rate
				if outRate <= 0 {
					outRate = decimTarget
				}
				sigMHz := signals[i].CenterHz / 1e6
				isWFM := (sigMHz >= 87.5 && sigMHz <= 108.0) || (signals[i].Class != nil && (signals[i].Class.ModType == "WFM" || signals[i].Class.ModType == "WFM_STEREO"))
				if isWFM {
					outRate = wfmStreamOutRate
				}
				decim := sampleRate / outRate
				if decim < 1 {
					decim = 1
				}
				trimSamples := (overlapLen + decim - 1) / decim
				if i == 0 {
					logging.Debug("extract", "gpu_result", "rate", res.Rate, "outRate", outRate, "decim", decim, "trim", trimSamples)
				}
				// Update phase state — advance only by NEW data length, not overlap
				phaseInc := -2.0 * math.Pi * jobs[i].OffsetHz / float64(sampleRate)
				phaseState[signals[i].ID].phase += phaseInc * float64(len(allIQ))
				// Normalize to [-π, π) to prevent float64 drift over long runs
				phaseState[signals[i].ID].phase = math.Remainder(phaseState[signals[i].ID].phase, 2*math.Pi)

				// Trim overlap from output
				iq := res.IQ
				rawLen := len(iq)
				if trimSamples > 0 && trimSamples < len(iq) {
					iq = iq[trimSamples:]
				}
				if i == 0 {
					logging.Debug("boundary", "extract_trim", "path", "gpu", "raw_len", rawLen, "trim", trimSamples, "out_len", len(iq), "overlap_len", overlapLen, "allIQ_len", len(allIQ), "gpuIQ_len", len(gpuIQ), "outRate", outRate, "signal", signals[i].ID)
					logExtractorHeadComparison(signals[i].ID, "gpu", overlapLen, res.IQ, trimSamples, iq)
				}
				if coll != nil {
					tags := telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", signals[i].ID), "path", "gpu")
					stats := computeIQHeadStats(iq, 64)
					coll.SetGauge("iq.extract.output.length", float64(len(iq)), tags)
					coll.Observe("iq.extract.output.head_mean_mag", stats.meanMag, tags)
					coll.Observe("iq.extract.output.head_min_mag", stats.minMag, tags)
					coll.Observe("iq.extract.output.head_max_step", stats.maxStep, tags)
					coll.Observe("iq.extract.output.head_p95_step", stats.p95Step, tags)
					coll.Observe("iq.extract.output.head_tail_ratio", stats.headTail, tags)
					coll.SetGauge("iq.extract.output.head_low_magnitude_count", float64(stats.lowMag), tags)
					coll.SetGauge("iq.extract.raw.length", float64(rawLen), tags)
					coll.SetGauge("iq.extract.trim.trim_samples", float64(trimSamples), tags)
					if rawLen > 0 {
						coll.SetGauge("iq.extract.raw.head_mag", math.Hypot(float64(real(res.IQ[0])), float64(imag(res.IQ[0]))), tags)
						coll.SetGauge("iq.extract.raw.tail_mag", math.Hypot(float64(real(res.IQ[rawLen-1])), float64(imag(res.IQ[rawLen-1]))), tags)
						rawHead := probeHead(res.IQ, 16, 1e-6)
						coll.SetGauge("iq.extract.raw.head_zero_count", float64(rawHead.zeroCount), tags)
						coll.SetGauge("iq.extract.raw.first_nonzero_index", float64(rawHead.firstNonZeroIndex), tags)
						coll.SetGauge("iq.extract.raw.head_max_step", rawHead.maxStep, tags)
						coll.Event("extract_raw_head_probe", "info", "raw extractor head probe", tags, map[string]any{
							"mags": rawHead.mags,
							"zero_count": rawHead.zeroCount,
							"first_nonzero_index": rawHead.firstNonZeroIndex,
							"head_max_step": rawHead.maxStep,
							"trim_samples": trimSamples,
						})
					}
					if len(iq) > 0 {
						coll.SetGauge("iq.extract.trimmed.head_mag", math.Hypot(float64(real(iq[0])), float64(imag(iq[0]))), tags)
						coll.SetGauge("iq.extract.trimmed.tail_mag", math.Hypot(float64(real(iq[len(iq)-1])), float64(imag(iq[len(iq)-1]))), tags)
						trimmedHead := probeHead(iq, 16, 1e-6)
						coll.SetGauge("iq.extract.trimmed.head_zero_count", float64(trimmedHead.zeroCount), tags)
						coll.SetGauge("iq.extract.trimmed.first_nonzero_index", float64(trimmedHead.firstNonZeroIndex), tags)
						coll.SetGauge("iq.extract.trimmed.head_max_step", trimmedHead.maxStep, tags)
						coll.Event("extract_trimmed_head_probe", "info", "trimmed extractor head probe", tags, map[string]any{
							"mags": trimmedHead.mags,
							"zero_count": trimmedHead.zeroCount,
							"first_nonzero_index": trimmedHead.firstNonZeroIndex,
							"head_max_step": trimmedHead.maxStep,
							"trim_samples": trimSamples,
						})
					}
					if rb := rawBoundary[signals[i].ID]; rb.set && rawLen > 0 {
						prevMag := math.Hypot(float64(real(rb.last)), float64(imag(rb.last)))
						currMag := math.Hypot(float64(real(res.IQ[0])), float64(imag(res.IQ[0])))
						coll.SetGauge("iq.extract.raw.boundary.prev_tail_mag", prevMag, tags)
						coll.SetGauge("iq.extract.raw.boundary.curr_head_mag", currMag, tags)
						coll.Event("extract_raw_boundary", "info", "raw extractor boundary", tags, map[string]any{
							"delta_mag": math.Abs(currMag - prevMag),
							"trim_samples": trimSamples,
							"raw_len": rawLen,
						})
					}
					if tb := trimmedBoundary[signals[i].ID]; tb.set && len(iq) > 0 {
						prevMag := math.Hypot(float64(real(tb.last)), float64(imag(tb.last)))
						currMag := math.Hypot(float64(real(iq[0])), float64(imag(iq[0])))
						coll.SetGauge("iq.extract.trimmed.boundary.prev_tail_mag", prevMag, tags)
						coll.SetGauge("iq.extract.trimmed.boundary.curr_head_mag", currMag, tags)
						coll.Event("extract_trimmed_boundary", "info", "trimmed extractor boundary", tags, map[string]any{
							"delta_mag": math.Abs(currMag - prevMag),
							"trim_samples": trimSamples,
							"out_len": len(iq),
						})
					}
				}
				if rawLen > 0 {
					rawBoundary[signals[i].ID] = boundaryProbeState{last: res.IQ[rawLen-1], set: true}
				}
				if len(iq) > 0 {
					trimmedBoundary[signals[i].ID] = boundaryProbeState{last: iq[len(iq)-1], set: true}
				}
				out[i] = iq
				rates[i] = res.Rate
			}
			return out, rates
		} else if err != nil {
			log.Printf("gpudemod: stream batch extraction failed: %v", err)
		}
	}

	// CPU fallback (with phase tracking)
	for i, sig := range signals {
		offset := sig.CenterHz - centerHz
		bw := jobs[i].BW
		ps := phaseState[sig.ID]

		// Phase-continuous FreqShift — rewind by overlap so new data starts at ps.phase
		shifted := make([]complex64, len(gpuIQ))
		inc := -2.0 * math.Pi * offset / float64(sampleRate)
		phase := ps.phase - inc*float64(overlapLen)
		for k, v := range gpuIQ {
			phase += inc
			re := math.Cos(phase)
			im := math.Sin(phase)
			shifted[k] = complex(
				float32(float64(real(v))*re-float64(imag(v))*im),
				float32(float64(real(v))*im+float64(imag(v))*re),
			)
		}
		// Advance phase by NEW data length only
		ps.phase += inc * float64(len(allIQ))
		ps.phase = math.Remainder(ps.phase, 2*math.Pi)

		cutoff := bw / 2
		if cutoff < 200 {
			cutoff = 200
		}
		if cutoff > float64(sampleRate)/2-1 {
			cutoff = float64(sampleRate)/2 - 1
		}
		firTaps := 101
		if aqCfg.firTaps > 0 {
			firTaps = aqCfg.firTaps
		}
		taps := dsp.LowpassFIR(cutoff, sampleRate, firTaps)
		filtered := dsp.ApplyFIR(shifted, taps)
		sigMHz := sig.CenterHz / 1e6
		isWFM := (sigMHz >= 87.5 && sigMHz <= 108.0) || (sig.Class != nil && (sig.Class.ModType == "WFM" || sig.Class.ModType == "WFM_STEREO"))
		outRate := decimTarget
		if isWFM {
			outRate = wfmStreamOutRate
		}
		decim := sampleRate / outRate
		if decim < 1 {
			decim = 1
		}
		decimated := dsp.Decimate(filtered, decim)
		rates[i] = sampleRate / decim

		// Trim overlap — use ceil to ensure ALL overlap samples are removed.
		// Floor trim (overlapLen/decim) leaves a remainder for non-divisible
		// factors (e.g. 512/20=25 trims only 500 of 512 samples → 12 leak).
		trimSamples := (overlapLen + decim - 1) / decim
		if i == 0 {
			logging.Debug("extract", "cpu_result", "outRate", outRate, "decim", decim, "trim", trimSamples)
		}
		rawIQ := decimated
		rawLen := len(rawIQ)
		if trimSamples > 0 && trimSamples < len(decimated) {
			decimated = decimated[trimSamples:]
		}
		if i == 0 {
			logging.Debug("boundary", "extract_trim", "path", "cpu", "raw_len", rawLen, "trim", trimSamples, "out_len", len(decimated), "overlap_len", overlapLen, "allIQ_len", len(allIQ), "gpuIQ_len", len(gpuIQ), "outRate", outRate, "signal", signals[i].ID)
			logExtractorHeadComparison(signals[i].ID, "cpu", overlapLen, decimated, trimSamples, decimated)
		}
		if coll != nil {
			tags := telemetry.TagsFromPairs("signal_id", fmt.Sprintf("%d", signals[i].ID), "path", "cpu")
			stats := computeIQHeadStats(decimated, 64)
			coll.SetGauge("iq.extract.output.length", float64(len(decimated)), tags)
			coll.Observe("iq.extract.output.head_mean_mag", stats.meanMag, tags)
			coll.Observe("iq.extract.output.head_min_mag", stats.minMag, tags)
			coll.Observe("iq.extract.output.head_max_step", stats.maxStep, tags)
			coll.Observe("iq.extract.output.head_p95_step", stats.p95Step, tags)
			coll.Observe("iq.extract.output.head_tail_ratio", stats.headTail, tags)
			coll.SetGauge("iq.extract.output.head_low_magnitude_count", float64(stats.lowMag), tags)
			coll.SetGauge("iq.extract.raw.length", float64(rawLen), tags)
			coll.SetGauge("iq.extract.trim.trim_samples", float64(trimSamples), tags)
			if rb := rawBoundary[signals[i].ID]; rb.set && rawLen > 0 {
				observeBoundarySample(coll, "iq.extract.raw.boundary", tags, rb.last, rawIQ[0])
			}
			if tb := trimmedBoundary[signals[i].ID]; tb.set && len(decimated) > 0 {
				observeBoundarySample(coll, "iq.extract.trimmed.boundary", tags, tb.last, decimated[0])
			}
		}
		if rawLen > 0 {
			rawBoundary[signals[i].ID] = boundaryProbeState{last: rawIQ[rawLen-1], set: true}
		}
		if len(decimated) > 0 {
			trimmedBoundary[signals[i].ID] = boundaryProbeState{last: decimated[len(decimated)-1], set: true}
		}
		out[i] = decimated
	}
	return out, rates
}

type iqHeadStats struct {
	length      int
	minMag      float64
	maxMag      float64
	meanMag     float64
	lowMag      int
	maxStep     float64
	maxStepIdx  int
	p95Step     float64
	headTail    float64
	headMinIdx  int
	stepSamples []float64
}

type boundaryProbeState struct {
	last complex64
	set  bool
}

type headProbe struct {
	zeroCount         int
	firstNonZeroIndex int
	maxStep           float64
	mags              []float64
}

func probeHead(samples []complex64, n int, zeroThreshold float64) headProbe {
	if n <= 0 || len(samples) == 0 {
		return headProbe{firstNonZeroIndex: -1}
	}
	if len(samples) < n {
		n = len(samples)
	}
	if zeroThreshold <= 0 {
		zeroThreshold = 1e-6
	}
	out := headProbe{firstNonZeroIndex: -1, mags: make([]float64, 0, n)}
	for i := 0; i < n; i++ {
		v := samples[i]
		mag := math.Hypot(float64(real(v)), float64(imag(v)))
		out.mags = append(out.mags, mag)
		if mag <= zeroThreshold {
			out.zeroCount++
		} else if out.firstNonZeroIndex < 0 {
			out.firstNonZeroIndex = i
		}
		if i > 0 {
			p := samples[i-1]
			num := float64(real(p))*float64(imag(v)) - float64(imag(p))*float64(real(v))
			den := float64(real(p))*float64(real(v)) + float64(imag(p))*float64(imag(v))
			step := math.Abs(math.Atan2(num, den))
			if step > out.maxStep {
				out.maxStep = step
			}
		}
	}
	return out
}

func observeBoundarySample(coll *telemetry.Collector, metricPrefix string, tags map[string]string, prev complex64, curr complex64) {
	prevMag := math.Hypot(float64(real(prev)), float64(imag(prev)))
	currMag := math.Hypot(float64(real(curr)), float64(imag(curr)))
	deltaMag := math.Abs(currMag - prevMag)
	num := float64(real(prev))*float64(imag(curr)) - float64(imag(prev))*float64(real(curr))
	den := float64(real(prev))*float64(real(curr)) + float64(imag(prev))*float64(imag(curr))
	deltaPhase := math.Abs(math.Atan2(num, den))
	d2 := float64(real(curr-prev))*float64(real(curr-prev)) + float64(imag(curr-prev))*float64(imag(curr-prev))
	coll.Observe(metricPrefix+".delta_mag", deltaMag, tags)
	coll.Observe(metricPrefix+".delta_phase", deltaPhase, tags)
	coll.Observe(metricPrefix+".d2", d2, tags)
	coll.Observe(metricPrefix+".discontinuity_score", deltaMag+deltaPhase, tags)
}

func computeIQHeadStats(iq []complex64, headLen int) iqHeadStats {
	stats := iqHeadStats{minMag: math.MaxFloat64, headMinIdx: -1, maxStepIdx: -1}
	if len(iq) == 0 {
		stats.minMag = 0
		return stats
	}
	n := len(iq)
	if headLen > 0 && headLen < n {
		n = headLen
	}
	stats.length = n
	stats.stepSamples = make([]float64, 0, max(0, n-1))
	sumMag := 0.0
	headSum := 0.0
	tailSum := 0.0
	tailCount := 0
	for i := 0; i < n; i++ {
		v := iq[i]
		mag := math.Hypot(float64(real(v)), float64(imag(v)))
		if mag < stats.minMag {
			stats.minMag = mag
			stats.headMinIdx = i
		}
		if mag > stats.maxMag {
			stats.maxMag = mag
		}
		sumMag += mag
		if mag < 0.05 {
			stats.lowMag++
		}
		if i < min(16, n) {
			headSum += mag
		}
		if i >= max(0, n-16) {
			tailSum += mag
			tailCount++
		}
		if i > 0 {
			p := iq[i-1]
			num := float64(real(p))*float64(imag(v)) - float64(imag(p))*float64(real(v))
			den := float64(real(p))*float64(real(v)) + float64(imag(p))*float64(imag(v))
			step := math.Abs(math.Atan2(num, den))
			if step > stats.maxStep {
				stats.maxStep = step
				stats.maxStepIdx = i - 1
			}
			stats.stepSamples = append(stats.stepSamples, step)
		}
	}
	stats.meanMag = sumMag / float64(n)
	if len(stats.stepSamples) > 0 {
		sorted := append([]float64(nil), stats.stepSamples...)
		sort.Float64s(sorted)
		idx := int(float64(len(sorted)-1) * 0.95)
		stats.p95Step = sorted[idx]
	} else {
		stats.p95Step = stats.maxStep
	}
	if headSum > 0 && tailCount > 0 {
		headMean := headSum / float64(min(16, n))
		tailMean := tailSum / float64(tailCount)
		if tailMean > 0 {
			stats.headTail = headMean / tailMean
		}
	}
	return stats
}

func observeIQStats(coll *telemetry.Collector, stage string, iq []complex64, tags telemetry.Tags) {
	if coll == nil || len(iq) == 0 {
		return
	}
	stats := computeIQHeadStats(iq, len(iq))
	stageTags := telemetry.TagsWith(tags, "stage", stage)
	coll.Observe("iq.magnitude.min", stats.minMag, stageTags)
	coll.Observe("iq.magnitude.max", stats.maxMag, stageTags)
	coll.Observe("iq.magnitude.mean", stats.meanMag, stageTags)
	coll.Observe("iq.phase_step.max", stats.maxStep, stageTags)
	coll.Observe("iq.phase_step.p95", stats.p95Step, stageTags)
	coll.Observe("iq.low_magnitude.count", float64(stats.lowMag), stageTags)
	coll.SetGauge("iq.length", float64(stats.length), stageTags)
}

func logExtractorHeadComparison(signalID int64, path string, overlapLen int, raw []complex64, trimSamples int, out []complex64) {
	rawStats := computeIQHeadStats(raw, 96)
	trimmedStats := computeIQHeadStats(out, 96)
	logging.Debug("boundary", "extract_head_compare",
		"signal", signalID,
		"path", path,
		"raw_len", len(raw),
		"trim", trimSamples,
		"out_len", len(out),
		"overlap_len", overlapLen,
		"raw_min_mag", rawStats.minMag,
		"raw_min_idx", rawStats.headMinIdx,
		"raw_max_step", rawStats.maxStep,
		"raw_max_step_idx", rawStats.maxStepIdx,
		"raw_head_tail", rawStats.headTail,
		"trimmed_min_mag", trimmedStats.minMag,
		"trimmed_min_idx", trimmedStats.headMinIdx,
		"trimmed_max_step", trimmedStats.maxStep,
		"trimmed_max_step_idx", trimmedStats.maxStepIdx,
		"trimmed_head_tail", trimmedStats.headTail,
	)
	for _, off := range []int{2, 4, 8, 16} {
		if len(out) <= off+8 {
			continue
		}
		offStats := computeIQHeadStats(out[off:], 96)
		logging.Debug("boundary", "extract_head_offset_compare",
			"signal", signalID,
			"path", path,
			"offset", off,
			"base_min_mag", trimmedStats.minMag,
			"base_min_idx", trimmedStats.headMinIdx,
			"base_max_step", trimmedStats.maxStep,
			"base_max_step_idx", trimmedStats.maxStepIdx,
			"offset_min_mag", offStats.minMag,
			"offset_min_idx", offStats.headMinIdx,
			"offset_max_step", offStats.maxStep,
			"offset_max_step_idx", offStats.maxStepIdx,
			"offset_head_tail", offStats.headTail,
		)
	}
}

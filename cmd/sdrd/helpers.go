package main

import (
	"log"
	"math"
	"sort"
	"strconv"
	"time"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/demod/gpudemod"
	"sdr-wideband-suite/internal/detector"
	"sdr-wideband-suite/internal/dsp"
	"sdr-wideband-suite/internal/logging"
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

	// Try GPU BatchRunner with phase
	runner := extractMgr.get(len(gpuIQ), sampleRate)
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
				trimSamples := overlapLen / decim
				if i == 0 {
					logging.Debug("extract", "gpu_result", "rate", res.Rate, "outRate", outRate, "decim", decim, "trim", trimSamples)
				}
				// Update phase state — advance only by NEW data length, not overlap
				phaseInc := -2.0 * math.Pi * jobs[i].OffsetHz / float64(sampleRate)
				phaseState[signals[i].ID].phase += phaseInc * float64(len(allIQ))

				// Trim overlap from output
				iq := res.IQ
				if trimSamples > 0 && trimSamples < len(iq) {
					iq = iq[trimSamples:]
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

		// Trim overlap
		trimSamples := overlapLen / decim
		if i == 0 {
			logging.Debug("extract", "cpu_result", "outRate", outRate, "decim", decim, "trim", trimSamples)
		}
		if trimSamples > 0 && trimSamples < len(decimated) {
			decimated = decimated[trimSamples:]
		}
		out[i] = decimated
	}
	return out, rates
}

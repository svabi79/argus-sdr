package main

import (
	"log"
	"sort"
	"strconv"
	"time"

	"sdr-visual-suite/internal/config"
	"sdr-visual-suite/internal/demod/gpudemod"
	"sdr-visual-suite/internal/detector"
	"sdr-visual-suite/internal/dsp"
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
	if m.runner == nil {
		if r, err := gpudemod.NewBatchRunner(sampleCount, sampleRate); err == nil {
			m.runner = r
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
			// Minimum extraction BW: ensure enough bandwidth for demod features
			// FM broadcast (87.5-108 MHz) needs >=150kHz for stereo pilot + RDS at 57kHz
			// Also widen for any signal classified as WFM (in case of re-extraction)
			sigMHz := sig.CenterHz / 1e6
			isWFM := (sigMHz >= 87.5 && sigMHz <= 108.0) || (sig.Class != nil && (sig.Class.ModType == "WFM" || sig.Class.ModType == "WFM_STEREO"))
			if isWFM {
				if bw < 150000 {
					bw = 150000
				}
			} else if bw < 20000 {
				bw = 20000
			}
			jobs[i] = gpudemod.ExtractJob{OffsetHz: sig.CenterHz - centerHz, BW: bw, OutRate: decimTarget}
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
		// FM broadcast (87.5-108 MHz) needs >=150kHz for stereo + RDS
		sigMHz := sig.CenterHz / 1e6
		isWFM := (sigMHz >= 87.5 && sigMHz <= 108.0) || (sig.Class != nil && (sig.Class.ModType == "WFM" || sig.Class.ModType == "WFM_STEREO"))
		if isWFM {
			if bw < 150000 {
				bw = 150000
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

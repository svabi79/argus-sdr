package main

import (
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

func extractSignalIQ(iq []complex64, sampleRate int, centerHz float64, sigHz float64, bwHz float64) []complex64 {
	if len(iq) == 0 || sampleRate <= 0 {
		return nil
	}
	results := extractSignalIQBatch(iq, sampleRate, centerHz, []detector.Signal{{CenterHz: sigHz, BWHz: bwHz}})
	if len(results) == 0 {
		return nil
	}
	return results[0]
}

func extractSignalIQBatch(iq []complex64, sampleRate int, centerHz float64, signals []detector.Signal) [][]complex64 {
	out := make([][]complex64, len(signals))
	if len(iq) == 0 || sampleRate <= 0 || len(signals) == 0 {
		return out
	}
	decimTarget := 200000
	if decimTarget <= 0 {
		decimTarget = sampleRate
	}

	var runner *gpudemod.BatchRunner
	if gpudemod.Available() {
		if gpuRunner, err := gpudemod.NewBatchRunner(len(iq), sampleRate); err == nil {
			runner = gpuRunner
			defer runner.Close()
		}
	}

	if runner != nil {
		jobs := make([]gpudemod.ExtractJob, len(signals))
		for i, sig := range signals {
			jobs[i] = gpudemod.ExtractJob{OffsetHz: sig.CenterHz - centerHz, BW: sig.BWHz, OutRate: decimTarget}
		}
		if gpuOuts, _, err := runner.ShiftFilterDecimateBatch(iq, jobs); err == nil && len(gpuOuts) == len(signals) {
			for i := range gpuOuts {
				out[i] = gpuOuts[i]
			}
			return out
		}
	}

	for i, sig := range signals {
		offset := sig.CenterHz - centerHz
		shifted := dsp.FreqShift(iq, sampleRate, offset)
		cutoff := sig.BWHz / 2
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
	}
	return out
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

package main

import "sdr-wideband-suite/internal/pipeline"

func buildWindowStats(windows []pipeline.RefinementWindow) *RefinementWindowStats {
	if len(windows) == 0 {
		return nil
	}
	stats := &RefinementWindowStats{Sources: map[string]int{}}
	minSpan := 0.0
	maxSpan := 0.0
	sum := 0.0
	for i, w := range windows {
		span := w.SpanHz
		if span <= 0 {
			continue
		}
		if i == 0 || span < minSpan {
			minSpan = span
		}
		if i == 0 || span > maxSpan {
			maxSpan = span
		}
		sum += span
		stats.Count++
		if w.Source != "" {
			stats.Sources[w.Source]++
		}
	}
	if stats.Count == 0 {
		return nil
	}
	stats.MinSpan = minSpan
	stats.MaxSpan = maxSpan
	stats.AvgSpan = sum / float64(stats.Count)
	return stats
}

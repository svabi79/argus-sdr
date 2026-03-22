package main

import (
	"sort"

	"sdr-wideband-suite/internal/pipeline"
)

type WindowSummary struct {
	Refinement     *RefinementWindowStats        `json:"refinement,omitempty"`
	MonitorWindows []pipeline.MonitorWindowStats `json:"monitor_windows,omitempty"`
}

func buildWindowSummary(plan pipeline.RefinementPlan, refinementWindows []pipeline.RefinementWindow, candidates []pipeline.Candidate) *WindowSummary {
	refinementStats := buildWindowStats(refinementWindows)
	monitorSummary := buildMonitorWindowSummary(plan.MonitorWindows, plan.MonitorWindowStats, candidates)
	if refinementStats == nil && len(monitorSummary) == 0 {
		return nil
	}
	return &WindowSummary{
		Refinement:     refinementStats,
		MonitorWindows: monitorSummary,
	}
}

func buildMonitorWindowSummary(windows []pipeline.MonitorWindow, stats []pipeline.MonitorWindowStats, candidates []pipeline.Candidate) []pipeline.MonitorWindowStats {
	var summary []pipeline.MonitorWindowStats
	switch {
	case len(stats) > 0:
		summary = append([]pipeline.MonitorWindowStats(nil), stats...)
	case len(windows) > 0:
		summary = make([]pipeline.MonitorWindowStats, 0, len(windows))
		for _, win := range windows {
			summary = append(summary, pipeline.MonitorWindowStats{
				Index:        win.Index,
				Label:        win.Label,
				Zone:         win.Zone,
				Source:       win.Source,
				StartHz:      win.StartHz,
				EndHz:        win.EndHz,
				CenterHz:     win.CenterHz,
				SpanHz:       win.SpanHz,
				Priority:     win.Priority,
				PriorityBias: win.PriorityBias,
				RecordBias:   win.RecordBias,
				DecodeBias:   win.DecodeBias,
				AutoRecord:   win.AutoRecord,
				AutoDecode:   win.AutoDecode,
			})
		}
	default:
		return nil
	}

	if len(candidates) > 0 && len(summary) > 0 {
		windowsForMatch := windows
		if len(windowsForMatch) == 0 {
			windowsForMatch = monitorWindowsFromStats(summary)
		}
		if len(windowsForMatch) > 0 {
			counts := map[int]int{}
			total := 0
			for _, cand := range candidates {
				matches := cand.MonitorMatches
				if len(matches) == 0 {
					matches = pipeline.MonitorWindowMatchesForCandidate(windowsForMatch, cand)
				}
				for _, match := range matches {
					counts[match.Index]++
					total++
				}
			}
			if total > 0 {
				for i := range summary {
					if summary[i].Candidates == 0 {
						summary[i].Candidates = counts[summary[i].Index]
					}
				}
			}
		}
	}

	sort.Slice(summary, func(i, j int) bool {
		return summary[i].Index < summary[j].Index
	})
	return summary
}

func monitorWindowsFromStats(stats []pipeline.MonitorWindowStats) []pipeline.MonitorWindow {
	if len(stats) == 0 {
		return nil
	}
	windows := make([]pipeline.MonitorWindow, 0, len(stats))
	for _, stat := range stats {
		windows = append(windows, pipeline.MonitorWindow{
			Index:        stat.Index,
			Label:        stat.Label,
			Zone:         stat.Zone,
			Source:       stat.Source,
			StartHz:      stat.StartHz,
			EndHz:        stat.EndHz,
			CenterHz:     stat.CenterHz,
			SpanHz:       stat.SpanHz,
			Priority:     stat.Priority,
			PriorityBias: stat.PriorityBias,
			RecordBias:   stat.RecordBias,
			DecodeBias:   stat.DecodeBias,
			AutoRecord:   stat.AutoRecord,
			AutoDecode:   stat.AutoDecode,
		})
	}
	return windows
}

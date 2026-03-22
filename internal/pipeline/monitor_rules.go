package pipeline

import "sdr-wideband-suite/internal/config"

func NormalizeMonitorWindows(goals config.PipelineGoalConfig, centerHz float64) []MonitorWindow {
	if len(goals.MonitorWindows) > 0 {
		windows := make([]MonitorWindow, 0, len(goals.MonitorWindows))
		for _, raw := range goals.MonitorWindows {
			if win, ok := normalizeGoalWindow(raw, centerHz); ok {
				windows = append(windows, win)
			}
		}
		if len(windows) > 0 {
			return windows
		}
	}
	if goals.MonitorStartHz > 0 && goals.MonitorEndHz > goals.MonitorStartHz {
		start := goals.MonitorStartHz
		end := goals.MonitorEndHz
		span := end - start
		return []MonitorWindow{{
			Label:    "primary",
			StartHz:  start,
			EndHz:    end,
			CenterHz: (start + end) / 2,
			SpanHz:   span,
			Source:   "goals:bounds",
		}}
	}
	if goals.MonitorSpanHz > 0 && centerHz != 0 {
		half := goals.MonitorSpanHz / 2
		start := centerHz - half
		end := centerHz + half
		return []MonitorWindow{{
			Label:    "primary",
			StartHz:  start,
			EndHz:    end,
			CenterHz: centerHz,
			SpanHz:   goals.MonitorSpanHz,
			Source:   "goals:span",
		}}
	}
	return nil
}

func MonitorWindowBounds(windows []MonitorWindow) (float64, float64, bool) {
	minStart := 0.0
	maxEnd := 0.0
	ok := false
	for _, w := range windows {
		if w.StartHz <= 0 || w.EndHz <= 0 || w.EndHz <= w.StartHz {
			continue
		}
		if !ok || w.StartHz < minStart {
			minStart = w.StartHz
		}
		if !ok || w.EndHz > maxEnd {
			maxEnd = w.EndHz
		}
		ok = true
	}
	return minStart, maxEnd, ok
}

func normalizeGoalWindow(raw config.MonitorWindow, fallbackCenter float64) (MonitorWindow, bool) {
	if raw.StartHz > 0 && raw.EndHz > raw.StartHz {
		span := raw.EndHz - raw.StartHz
		return MonitorWindow{
			Label:    raw.Label,
			StartHz:  raw.StartHz,
			EndHz:    raw.EndHz,
			CenterHz: (raw.StartHz + raw.EndHz) / 2,
			SpanHz:   span,
			Source:   "goals:window:start_end",
		}, true
	}
	center := raw.CenterHz
	if center == 0 {
		center = fallbackCenter
	}
	if center != 0 && raw.SpanHz > 0 {
		half := raw.SpanHz / 2
		source := "goals:window:center_span"
		if raw.CenterHz == 0 {
			source = "goals:window:span_default"
		}
		return MonitorWindow{
			Label:    raw.Label,
			StartHz:  center - half,
			EndHz:    center + half,
			CenterHz: center,
			SpanHz:   raw.SpanHz,
			Source:   source,
		}, true
	}
	return MonitorWindow{}, false
}

func monitorBounds(policy Policy) (float64, float64, bool) {
	if len(policy.MonitorWindows) > 0 {
		return MonitorWindowBounds(policy.MonitorWindows)
	}
	start := policy.MonitorStartHz
	end := policy.MonitorEndHz
	if start != 0 && end != 0 && end > start {
		return start, end, true
	}
	if policy.MonitorSpanHz > 0 && policy.MonitorCenterHz != 0 {
		half := policy.MonitorSpanHz / 2
		return policy.MonitorCenterHz - half, policy.MonitorCenterHz + half, true
	}
	return 0, 0, false
}

func candidateInMonitor(policy Policy, candidate Candidate) bool {
	if len(policy.MonitorWindows) > 0 {
		left, right := candidateBounds(candidate)
		for _, win := range policy.MonitorWindows {
			if win.StartHz <= 0 || win.EndHz <= 0 || win.EndHz <= win.StartHz {
				continue
			}
			if right >= win.StartHz && left <= win.EndHz {
				return true
			}
		}
		return false
	}
	start, end, ok := monitorBounds(policy)
	if !ok {
		return true
	}
	left, right := candidateBounds(candidate)
	return right >= start && left <= end
}

func candidateBounds(candidate Candidate) (float64, float64) {
	left := candidate.CenterHz
	right := candidate.CenterHz
	if candidate.BandwidthHz > 0 {
		left = candidate.CenterHz - candidate.BandwidthHz/2
		right = candidate.CenterHz + candidate.BandwidthHz/2
	}
	return left, right
}

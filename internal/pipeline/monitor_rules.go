package pipeline

import (
	"math"

	"sdr-wideband-suite/internal/config"
)

const maxMonitorWindowBias = 0.2

func NormalizeMonitorWindows(goals config.PipelineGoalConfig, centerHz float64) []MonitorWindow {
	if len(goals.MonitorWindows) > 0 {
		windows := make([]MonitorWindow, 0, len(goals.MonitorWindows))
		for _, raw := range goals.MonitorWindows {
			if win, ok := normalizeGoalWindow(raw, centerHz); ok {
				windows = append(windows, win)
			}
		}
		if len(windows) > 0 {
			return finalizeMonitorWindows(windows)
		}
	}
	if goals.MonitorStartHz > 0 && goals.MonitorEndHz > goals.MonitorStartHz {
		start := goals.MonitorStartHz
		end := goals.MonitorEndHz
		span := end - start
		return finalizeMonitorWindows([]MonitorWindow{{
			Label:    "primary",
			StartHz:  start,
			EndHz:    end,
			CenterHz: (start + end) / 2,
			SpanHz:   span,
			Source:   "goals:bounds",
		}})
	}
	if goals.MonitorSpanHz > 0 && centerHz != 0 {
		half := goals.MonitorSpanHz / 2
		start := centerHz - half
		end := centerHz + half
		return finalizeMonitorWindows([]MonitorWindow{{
			Label:    "primary",
			StartHz:  start,
			EndHz:    end,
			CenterHz: centerHz,
			SpanHz:   goals.MonitorSpanHz,
			Source:   "goals:span",
		}})
	}
	return nil
}

func finalizeMonitorWindows(windows []MonitorWindow) []MonitorWindow {
	if len(windows) == 0 {
		return nil
	}
	maxSpan := 0.0
	for _, w := range windows {
		if w.SpanHz > maxSpan {
			maxSpan = w.SpanHz
		}
	}
	for i := range windows {
		windows[i].Index = i
		priority := normalizeMonitorPriority(windows[i].Priority)
		windows[i].Priority = priority
		spanBias := 0.0
		if maxSpan > 0 && len(windows) > 1 && windows[i].SpanHz > 0 {
			spanBias = maxMonitorWindowBias * (1 - (windows[i].SpanHz / maxSpan))
			if spanBias < 0 {
				spanBias = 0
			}
		}
		policyBias := priority * maxMonitorWindowBias
		totalBias := spanBias + policyBias
		if totalBias > maxMonitorWindowBias {
			totalBias = maxMonitorWindowBias
		} else if totalBias < -maxMonitorWindowBias {
			totalBias = -maxMonitorWindowBias
		}
		windows[i].PriorityBias = totalBias
	}
	return windows
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
			Priority: raw.Priority,
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
			Priority: raw.Priority,
		}, true
	}
	return MonitorWindow{}, false
}

func normalizeMonitorPriority(priority float64) float64 {
	if math.IsNaN(priority) || math.IsInf(priority, 0) {
		return 0
	}
	if priority > 1 {
		return 1
	}
	if priority < -1 {
		return -1
	}
	return priority
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
		matches := MonitorWindowMatchesForCandidate(policy.MonitorWindows, candidate)
		return len(matches) > 0
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

func ApplyMonitorWindowMatches(policy Policy, candidate *Candidate) bool {
	if candidate == nil {
		return true
	}
	if len(policy.MonitorWindows) == 0 {
		candidate.MonitorMatches = nil
		if start, end, ok := monitorBounds(policy); ok {
			left, right := candidateBounds(*candidate)
			if right < start || left > end {
				return false
			}
		}
		return true
	}
	matches := MonitorWindowMatchesForCandidate(policy.MonitorWindows, *candidate)
	if len(matches) == 0 {
		candidate.MonitorMatches = nil
		return false
	}
	candidate.MonitorMatches = matches
	return true
}

func ApplyMonitorWindowMatchesToCandidates(policy Policy, candidates []Candidate) {
	if len(candidates) == 0 || len(policy.MonitorWindows) == 0 {
		return
	}
	for i := range candidates {
		_ = ApplyMonitorWindowMatches(policy, &candidates[i])
	}
}

func MonitorWindowMatches(policy Policy, candidate Candidate) []MonitorWindowMatch {
	return MonitorWindowMatchesForCandidate(policy.MonitorWindows, candidate)
}

func MonitorWindowMatchesForCandidate(windows []MonitorWindow, candidate Candidate) []MonitorWindowMatch {
	if len(windows) == 0 {
		return nil
	}
	left, right := candidateBounds(candidate)
	pointCandidate := candidate.BandwidthHz <= 0
	matches := make([]MonitorWindowMatch, 0, len(windows))
	for _, win := range windows {
		if win.StartHz <= 0 || win.EndHz <= 0 || win.EndHz <= win.StartHz {
			continue
		}
		if right < win.StartHz || left > win.EndHz {
			continue
		}
		overlap := math.Min(right, win.EndHz) - math.Max(left, win.StartHz)
		coverage := 0.0
		if win.SpanHz > 0 && overlap > 0 {
			coverage = overlap / win.SpanHz
		}
		if pointCandidate && candidate.CenterHz >= win.StartHz && candidate.CenterHz <= win.EndHz {
			coverage = 1
		}
		if coverage < 0 {
			coverage = 0
		}
		if coverage > 1 {
			coverage = 1
		}
		center := win.CenterHz
		if center == 0 {
			center = (win.StartHz + win.EndHz) / 2
		}
		distance := math.Abs(candidate.CenterHz - center)
		bias := win.PriorityBias * coverage
		matches = append(matches, MonitorWindowMatch{
			Index:      win.Index,
			Label:      win.Label,
			Source:     win.Source,
			StartHz:    win.StartHz,
			EndHz:      win.EndHz,
			CenterHz:   center,
			SpanHz:     win.SpanHz,
			OverlapHz:  overlap,
			Coverage:   coverage,
			DistanceHz: distance,
			Bias:       bias,
		})
	}
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func MonitorWindowBias(policy Policy, candidate Candidate) (float64, *MonitorWindowMatch) {
	matches := candidate.MonitorMatches
	if len(matches) == 0 {
		matches = MonitorWindowMatches(policy, candidate)
	}
	if len(matches) == 0 {
		return 0, nil
	}
	bestIdx := 0
	for i := 1; i < len(matches); i++ {
		if matches[i].Bias > matches[bestIdx].Bias {
			bestIdx = i
			continue
		}
		if matches[i].Bias == matches[bestIdx].Bias && matches[i].Coverage > matches[bestIdx].Coverage {
			bestIdx = i
		}
	}
	best := matches[bestIdx]
	return best.Bias, &best
}

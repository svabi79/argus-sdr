package main

import (
	"sort"
	"strings"

	"sdr-wideband-suite/internal/pipeline"
)

type WindowSummary struct {
	Refinement     *RefinementWindowStats        `json:"refinement,omitempty"`
	MonitorWindows []pipeline.MonitorWindowStats `json:"monitor_windows,omitempty"`
	Outcomes       *WindowOutcomeSummary         `json:"outcomes,omitempty"`
}

type WindowOutcomeSummary struct {
	Windows []MonitorWindowOutcome `json:"windows,omitempty"`
	Zones   []MonitorZoneOutcome   `json:"zones,omitempty"`
}

type MonitorWindowOutcome struct {
	Index      int           `json:"index"`
	Label      string        `json:"label,omitempty"`
	Zone       string        `json:"zone,omitempty"`
	Refinement OutcomeCounts `json:"refinement,omitempty"`
	Record     OutcomeCounts `json:"record,omitempty"`
	Decode     OutcomeCounts `json:"decode,omitempty"`
}

type MonitorZoneOutcome struct {
	Zone       string        `json:"zone"`
	Refinement OutcomeCounts `json:"refinement,omitempty"`
	Record     OutcomeCounts `json:"record,omitempty"`
	Decode     OutcomeCounts `json:"decode,omitempty"`
}

type OutcomeCounts struct {
	Admit    int `json:"admit,omitempty"`
	Hold     int `json:"hold,omitempty"`
	Displace int `json:"displace,omitempty"`
	Defer    int `json:"defer,omitempty"`
	Drop     int `json:"drop,omitempty"`
	Enabled  int `json:"enabled,omitempty"`
}

func (o *OutcomeCounts) addClass(class string) {
	switch class {
	case pipeline.AdmissionClassAdmit:
		o.Admit++
	case pipeline.AdmissionClassHold:
		o.Hold++
	case pipeline.AdmissionClassDisplace:
		o.Displace++
	case pipeline.AdmissionClassDefer:
		o.Defer++
	case pipeline.AdmissionClassDrop:
		o.Drop++
	}
}

func (o *OutcomeCounts) addEnabled(enabled bool) {
	if enabled {
		o.Enabled++
	}
}

func (o OutcomeCounts) hasAny() bool {
	return o.Admit > 0 || o.Hold > 0 || o.Displace > 0 || o.Defer > 0 || o.Drop > 0 || o.Enabled > 0
}

func (o *OutcomeCounts) addTotals(in OutcomeCounts) {
	o.Admit += in.Admit
	o.Hold += in.Hold
	o.Displace += in.Displace
	o.Defer += in.Defer
	o.Drop += in.Drop
	o.Enabled += in.Enabled
}

func buildWindowSummary(plan pipeline.RefinementPlan, refinementWindows []pipeline.RefinementWindow, candidates []pipeline.Candidate, workItems []pipeline.RefinementWorkItem, decisions []pipeline.SignalDecision) *WindowSummary {
	refinementStats := buildWindowStats(refinementWindows)
	monitorSummary := buildMonitorWindowSummary(plan.MonitorWindows, plan.MonitorWindowStats, candidates)
	outcomes := buildWindowOutcomeSummary(plan.MonitorWindows, plan.MonitorWindowStats, workItems, decisions)
	if refinementStats == nil && len(monitorSummary) == 0 && outcomes == nil {
		return nil
	}
	return &WindowSummary{
		Refinement:     refinementStats,
		MonitorWindows: monitorSummary,
		Outcomes:       outcomes,
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

func buildWindowOutcomeSummary(windows []pipeline.MonitorWindow, stats []pipeline.MonitorWindowStats, workItems []pipeline.RefinementWorkItem, decisions []pipeline.SignalDecision) *WindowOutcomeSummary {
	base := windows
	if len(base) == 0 && len(stats) > 0 {
		base = monitorWindowsFromStats(stats)
	}
	if len(base) == 0 {
		return nil
	}
	outcomes := make([]MonitorWindowOutcome, 0, len(base))
	index := make(map[int]int, len(base))
	for _, win := range base {
		outcomes = append(outcomes, MonitorWindowOutcome{
			Index: win.Index,
			Label: win.Label,
			Zone:  win.Zone,
		})
		index[win.Index] = len(outcomes) - 1
	}
	windowsForMatch := base
	if len(workItems) > 0 {
		for _, item := range workItems {
			class := outcomeClassForWorkItem(item)
			if class == "" {
				continue
			}
			matches := item.Candidate.MonitorMatches
			if len(matches) == 0 {
				matches = pipeline.MonitorWindowMatchesForCandidate(windowsForMatch, item.Candidate)
			}
			for _, match := range matches {
				if idx, ok := index[match.Index]; ok {
					outcomes[idx].Refinement.addClass(class)
				}
			}
		}
	}
	if len(decisions) > 0 {
		for _, decision := range decisions {
			if match := decisionWindowMatch(decision, "record"); match != nil {
				if idx, ok := index[match.Index]; ok {
					outcomes[idx].Record.addEnabled(decision.ShouldRecord)
					if decision.RecordAdmission != nil {
						outcomes[idx].Record.addClass(decision.RecordAdmission.Class)
					}
				}
			}
			if match := decisionWindowMatch(decision, "decode"); match != nil {
				if idx, ok := index[match.Index]; ok {
					outcomes[idx].Decode.addEnabled(decision.ShouldAutoDecode)
					if decision.DecodeAdmission != nil {
						outcomes[idx].Decode.addClass(decision.DecodeAdmission.Class)
					}
				}
			}
		}
	}
	hasOutcome := false
	for i := range outcomes {
		if outcomes[i].Refinement.hasAny() || outcomes[i].Record.hasAny() || outcomes[i].Decode.hasAny() {
			hasOutcome = true
			break
		}
	}
	if !hasOutcome {
		return nil
	}
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].Index < outcomes[j].Index
	})
	zoneIndex := map[string]int{}
	zones := make([]MonitorZoneOutcome, 0, len(outcomes))
	for _, outcome := range outcomes {
		zone := strings.TrimSpace(outcome.Zone)
		if zone == "" {
			continue
		}
		idx, ok := zoneIndex[zone]
		if !ok {
			zones = append(zones, MonitorZoneOutcome{Zone: zone})
			idx = len(zones) - 1
			zoneIndex[zone] = idx
		}
		zones[idx].Refinement.addTotals(outcome.Refinement)
		zones[idx].Record.addTotals(outcome.Record)
		zones[idx].Decode.addTotals(outcome.Decode)
	}
	if len(zones) > 0 {
		sort.Slice(zones, func(i, j int) bool {
			return zones[i].Zone < zones[j].Zone
		})
	}
	return &WindowOutcomeSummary{Windows: outcomes, Zones: zones}
}

func outcomeClassForWorkItem(item pipeline.RefinementWorkItem) string {
	if item.Admission != nil && item.Admission.Class != "" {
		return item.Admission.Class
	}
	switch item.Status {
	case pipeline.RefinementStatusAdmitted, pipeline.RefinementStatusRunning, pipeline.RefinementStatusCompleted:
		return pipeline.AdmissionClassAdmit
	case pipeline.RefinementStatusDisplaced:
		return pipeline.AdmissionClassDisplace
	case pipeline.RefinementStatusSkipped:
		return pipeline.AdmissionClassDefer
	case pipeline.RefinementStatusDropped:
		return pipeline.AdmissionClassDrop
	default:
		return ""
	}
}

func decisionWindowMatch(decision pipeline.SignalDecision, action string) *pipeline.MonitorWindowMatch {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "record":
		if decision.RecordWindow != nil {
			return decision.RecordWindow
		}
	case "decode":
		if decision.DecodeWindow != nil {
			return decision.DecodeWindow
		}
	}
	return decision.MonitorDetail
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

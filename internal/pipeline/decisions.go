package pipeline

import (
	"strings"

	"sdr-wideband-suite/internal/classifier"
)

type SignalDecision struct {
	Candidate        Candidate           `json:"candidate"`
	Class            string              `json:"class,omitempty"`
	ShouldRecord     bool                `json:"should_record"`
	ShouldAutoDecode bool                `json:"should_auto_decode"`
	Reason           string              `json:"reason,omitempty"`
	MonitorBias      float64             `json:"monitor_bias,omitempty"`
	MonitorDetail    *MonitorWindowMatch `json:"monitor_detail,omitempty"`
	RecordWindow     *MonitorWindowMatch `json:"record_window,omitempty"`
	DecodeWindow     *MonitorWindowMatch `json:"decode_window,omitempty"`
	RecordAdmission  *PriorityAdmission  `json:"record_admission,omitempty"`
	DecodeAdmission  *PriorityAdmission  `json:"decode_admission,omitempty"`
}

func DecideSignalAction(policy Policy, candidate Candidate, cls *classifier.Classification) SignalDecision {
	if len(policy.MonitorWindows) > 0 {
		_ = ApplyMonitorWindowMatches(policy, &candidate)
	}
	decision := SignalDecision{Candidate: candidate}
	classTag := ""
	hintTag := strings.TrimSpace(candidate.Hint)
	if cls != nil {
		decision.Class = string(cls.ModType)
		classTag = decision.Class
	}
	if classTag != "" && WantsClass(policy.AutoRecordClasses, classTag) {
		decision.ShouldRecord = true
		decision.Reason = DecisionReasonRecordClass
	} else if classTag == "" && hintTag != "" && WantsClass(policy.AutoRecordClasses, hintTag) {
		decision.ShouldRecord = true
		decision.Reason = DecisionReasonRecordHint
	}
	if classTag != "" && WantsClass(policy.AutoDecodeClasses, classTag) {
		decision.ShouldAutoDecode = true
		if decision.Reason == "" {
			decision.Reason = DecisionReasonDecodeClass
		}
	} else if classTag == "" && hintTag != "" && WantsClass(policy.AutoDecodeClasses, hintTag) {
		decision.ShouldAutoDecode = true
		if decision.Reason == "" {
			decision.Reason = DecisionReasonDecodeHint
		}
	}
	recordMatch := bestMonitorActionMatch(candidate.MonitorMatches, true, false)
	if !decision.ShouldRecord && recordMatch != nil {
		decision.ShouldRecord = true
		decision.RecordWindow = recordMatch
		if decision.Reason == "" {
			decision.Reason = DecisionReasonRecordWindow
		}
	}
	decodeMatch := bestMonitorActionMatch(candidate.MonitorMatches, false, true)
	if !decision.ShouldAutoDecode && decodeMatch != nil {
		decision.ShouldAutoDecode = true
		decision.DecodeWindow = decodeMatch
		if decision.Reason == "" {
			decision.Reason = DecisionReasonDecodeWindow
		}
	}
	if decision.Reason == "" && candidate.Hint != "" {
		decision.Reason = DecisionReasonHintOnly
	}
	monitorBias, monitorDetail := MonitorWindowBias(policy, candidate)
	if monitorDetail == nil {
		monitorDetail = selectMonitorDetail(recordMatch, decodeMatch)
	}
	if monitorBias != 0 {
		decision.MonitorBias = monitorBias
	}
	if monitorDetail != nil {
		decision.MonitorDetail = monitorDetail
	}
	return decision
}

func bestMonitorActionMatch(matches []MonitorWindowMatch, wantRecord bool, wantDecode bool) *MonitorWindowMatch {
	if len(matches) == 0 || (!wantRecord && !wantDecode) {
		return nil
	}
	best := -1
	for i := range matches {
		match := matches[i]
		if wantRecord && !match.AutoRecord {
			continue
		}
		if wantDecode && !match.AutoDecode {
			continue
		}
		if best == -1 || betterMonitorActionMatch(match, matches[best]) {
			best = i
		}
	}
	if best == -1 {
		return nil
	}
	return &matches[best]
}

func betterMonitorActionMatch(candidate MonitorWindowMatch, best MonitorWindowMatch) bool {
	if candidate.Coverage != best.Coverage {
		return candidate.Coverage > best.Coverage
	}
	if candidate.DistanceHz != best.DistanceHz {
		return candidate.DistanceHz < best.DistanceHz
	}
	if candidate.Bias != best.Bias {
		return candidate.Bias > best.Bias
	}
	return candidate.Index < best.Index
}

func selectMonitorDetail(recordMatch *MonitorWindowMatch, decodeMatch *MonitorWindowMatch) *MonitorWindowMatch {
	if recordMatch == nil {
		return decodeMatch
	}
	if decodeMatch == nil {
		return recordMatch
	}
	if betterMonitorActionMatch(*recordMatch, *decodeMatch) {
		return recordMatch
	}
	return decodeMatch
}

package pipeline

import (
	"sort"
	"time"
)

type DecisionQueueStats struct {
	RecordQueued   int     `json:"record_queued"`
	DecodeQueued   int     `json:"decode_queued"`
	RecordSelected int     `json:"record_selected"`
	DecodeSelected int     `json:"decode_selected"`
	RecordActive   int     `json:"record_active"`
	DecodeActive   int     `json:"decode_active"`
	RecordOldestS  float64 `json:"record_oldest_sec"`
	DecodeOldestS  float64 `json:"decode_oldest_sec"`
	RecordBudget   int     `json:"record_budget"`
	DecodeBudget   int     `json:"decode_budget"`
	HoldMs         int     `json:"hold_ms"`
	RecordHoldMs   int     `json:"record_hold_ms"`
	DecodeHoldMs   int     `json:"decode_hold_ms"`
	RecordDropped  int     `json:"record_dropped"`
	DecodeDropped  int     `json:"decode_dropped"`
}

type queuedDecision struct {
	ID        int64
	SNRDb     float64
	Hint      string
	Class     string
	FirstSeen time.Time
	LastSeen  time.Time
}

type queueSelection struct {
	selected map[int64]struct{}
	held     map[int64]struct{}
	scores   map[int64]float64
	minScore float64
	maxScore float64
	cutoff   float64
}

type decisionQueues struct {
	record     map[int64]*queuedDecision
	decode     map[int64]*queuedDecision
	recordHold map[int64]time.Time
	decodeHold map[int64]time.Time
}

func newDecisionQueues() *decisionQueues {
	return &decisionQueues{
		record:     map[int64]*queuedDecision{},
		decode:     map[int64]*queuedDecision{},
		recordHold: map[int64]time.Time{},
		decodeHold: map[int64]time.Time{},
	}
}

func (dq *decisionQueues) Apply(decisions []SignalDecision, budget BudgetModel, now time.Time, policy Policy) DecisionQueueStats {
	if dq == nil {
		return DecisionQueueStats{}
	}
	holdPolicy := HoldPolicyFromPolicy(policy)
	recordHold := time.Duration(holdPolicy.RecordMs) * time.Millisecond
	decodeHold := time.Duration(holdPolicy.DecodeMs) * time.Millisecond
	recSeen := map[int64]bool{}
	decSeen := map[int64]bool{}
	for i := range decisions {
		id := decisions[i].Candidate.ID
		if id == 0 {
			continue
		}
		if decisions[i].ShouldRecord {
			qd := dq.record[id]
			if qd == nil {
				qd = &queuedDecision{ID: id, FirstSeen: now}
				dq.record[id] = qd
			}
			qd.SNRDb = decisions[i].Candidate.SNRDb
			qd.Hint = decisions[i].Candidate.Hint
			qd.Class = decisions[i].Class
			qd.LastSeen = now
			recSeen[id] = true
		}
		if decisions[i].ShouldAutoDecode {
			qd := dq.decode[id]
			if qd == nil {
				qd = &queuedDecision{ID: id, FirstSeen: now}
				dq.decode[id] = qd
			}
			qd.SNRDb = decisions[i].Candidate.SNRDb
			qd.Hint = decisions[i].Candidate.Hint
			qd.Class = decisions[i].Class
			qd.LastSeen = now
			decSeen[id] = true
		}
	}
	for id := range dq.record {
		if !recSeen[id] {
			delete(dq.record, id)
		}
	}
	for id := range dq.decode {
		if !decSeen[id] {
			delete(dq.decode, id)
		}
	}

	purgeExpired(dq.recordHold, now)
	purgeExpired(dq.decodeHold, now)

	recSelected := selectQueued("record", dq.record, dq.recordHold, budget.Record.Max, recordHold, now, policy)
	decSelected := selectQueued("decode", dq.decode, dq.decodeHold, budget.Decode.Max, decodeHold, now, policy)
	recPressure := buildQueuePressure(budget.Record, len(dq.record), len(recSelected.selected), len(dq.recordHold))
	decPressure := buildQueuePressure(budget.Decode, len(dq.decode), len(decSelected.selected), len(dq.decodeHold))
	recPressureTag := pressureReasonTag(recPressure)
	decPressureTag := pressureReasonTag(decPressure)

	stats := DecisionQueueStats{
		RecordQueued:   len(dq.record),
		DecodeQueued:   len(dq.decode),
		RecordSelected: len(recSelected.selected),
		DecodeSelected: len(decSelected.selected),
		RecordActive:   len(dq.recordHold),
		DecodeActive:   len(dq.decodeHold),
		RecordOldestS:  oldestAge(dq.record, now),
		DecodeOldestS:  oldestAge(dq.decode, now),
		RecordBudget:   budget.Record.Max,
		DecodeBudget:   budget.Decode.Max,
		HoldMs:         budget.HoldMs,
		RecordHoldMs:   holdPolicy.RecordMs,
		DecodeHoldMs:   holdPolicy.DecodeMs,
	}

	for i := range decisions {
		id := decisions[i].Candidate.ID
		if decisions[i].ShouldRecord {
			decisions[i].RecordAdmission = buildQueueAdmission("record", id, recSelected, policy, holdPolicy, budget.Record.Source, recPressureTag)
			if _, ok := recSelected.selected[id]; !ok {
				decisions[i].ShouldRecord = false
				decisions[i].Reason = admissionReason(DecisionReasonQueueRecord, policy, holdPolicy, recPressureTag, "pressure:budget", "budget:"+slugToken(budget.Record.Source))
				stats.RecordDropped++
			}
		}
		if decisions[i].ShouldAutoDecode {
			decisions[i].DecodeAdmission = buildQueueAdmission("decode", id, decSelected, policy, holdPolicy, budget.Decode.Source, decPressureTag)
			if _, ok := decSelected.selected[id]; !ok {
				decisions[i].ShouldAutoDecode = false
				if decisions[i].Reason == "" {
					decisions[i].Reason = admissionReason(DecisionReasonQueueDecode, policy, holdPolicy, decPressureTag, "pressure:budget", "budget:"+slugToken(budget.Decode.Source))
				}
				stats.DecodeDropped++
			}
		}
	}
	return stats
}

func selectQueued(queueName string, queue map[int64]*queuedDecision, hold map[int64]time.Time, max int, holdDur time.Duration, now time.Time, policy Policy) queueSelection {
	selection := queueSelection{
		selected: map[int64]struct{}{},
		held:     map[int64]struct{}{},
		scores:   map[int64]float64{},
	}
	if len(queue) == 0 {
		return selection
	}
	type scored struct {
		id    int64
		score float64
	}
	scoredList := make([]scored, 0, len(queue))
	for id, qd := range queue {
		age := now.Sub(qd.FirstSeen).Seconds()
		boost := age / 2.0
		if boost > 5 {
			boost = 5
		}
		hint := qd.Hint
		if hint == "" {
			hint = qd.Class
		}
		policyBoost := DecisionPriorityBoost(policy, hint, qd.Class, queueName)
		score := qd.SNRDb + boost + policyBoost
		selection.scores[id] = score
		if len(scoredList) == 0 || score < selection.minScore {
			selection.minScore = score
		}
		if len(scoredList) == 0 || score > selection.maxScore {
			selection.maxScore = score
		}
		scoredList = append(scoredList, scored{id: id, score: score})
	}
	sort.Slice(scoredList, func(i, j int) bool {
		return scoredList[i].score > scoredList[j].score
	})
	limit := max
	if limit <= 0 || limit > len(scoredList) {
		limit = len(scoredList)
	}
	if len(hold) > 0 && len(hold) > limit {
		limit = len(hold)
		if limit > len(scoredList) {
			limit = len(scoredList)
		}
	}
	for id := range hold {
		if _, ok := queue[id]; ok {
			selection.selected[id] = struct{}{}
			selection.held[id] = struct{}{}
		}
	}
	for _, s := range scoredList {
		if len(selection.selected) >= limit {
			break
		}
		if _, ok := selection.selected[s.id]; ok {
			continue
		}
		selection.selected[s.id] = struct{}{}
	}
	if holdDur > 0 {
		for id := range selection.selected {
			hold[id] = now.Add(holdDur)
		}
	}
	if len(selection.selected) > 0 {
		first := true
		for id := range selection.selected {
			score := selection.scores[id]
			if first || score < selection.cutoff {
				selection.cutoff = score
				first = false
			}
		}
	}
	return selection
}

func buildQueueAdmission(queueName string, id int64, selection queueSelection, policy Policy, holdPolicy HoldPolicy, budgetSource string, pressureTag string) *PriorityAdmission {
	score, ok := selection.scores[id]
	if !ok {
		return nil
	}
	admission := &PriorityAdmission{
		Basis:  queueName,
		Score:  score,
		Cutoff: selection.cutoff,
		Tier:   PriorityTierFromRange(score, selection.minScore, selection.maxScore),
	}
	if _, ok := selection.selected[id]; ok {
		if _, held := selection.held[id]; held {
			admission.Class = AdmissionClassHold
			admission.Reason = admissionReason("queue:"+queueName+":hold", policy, holdPolicy, pressureTag, "pressure:hold", "budget:"+slugToken(budgetSource))
		} else {
			admission.Class = AdmissionClassAdmit
			admission.Reason = admissionReason("queue:"+queueName+":admit", policy, holdPolicy, pressureTag, "budget:"+slugToken(budgetSource))
		}
		return admission
	}
	admission.Class = AdmissionClassDefer
	admission.Reason = admissionReason("queue:"+queueName+":budget", policy, holdPolicy, pressureTag, "pressure:budget", "budget:"+slugToken(budgetSource))
	return admission
}

func purgeExpired(hold map[int64]time.Time, now time.Time) {
	for id, until := range hold {
		if now.After(until) {
			delete(hold, id)
		}
	}
}

func oldestAge(queue map[int64]*queuedDecision, now time.Time) float64 {
	oldest := 0.0
	first := true
	for _, qd := range queue {
		age := now.Sub(qd.FirstSeen).Seconds()
		if first || age > oldest {
			oldest = age
			first = false
		}
	}
	if first {
		return 0
	}
	return oldest
}

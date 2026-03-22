package pipeline

import (
	"sort"
	"time"
)

type DecisionQueueStats struct {
	RecordQueued          int     `json:"record_queued"`
	DecodeQueued          int     `json:"decode_queued"`
	RecordSelected        int     `json:"record_selected"`
	DecodeSelected        int     `json:"decode_selected"`
	RecordActive          int     `json:"record_active"`
	DecodeActive          int     `json:"decode_active"`
	RecordOldestS         float64 `json:"record_oldest_sec"`
	DecodeOldestS         float64 `json:"decode_oldest_sec"`
	RecordBudget          int     `json:"record_budget"`
	DecodeBudget          int     `json:"decode_budget"`
	HoldMs                int     `json:"hold_ms"`
	DecisionHoldMs        int     `json:"decision_hold_ms,omitempty"`
	RecordHoldMs          int     `json:"record_hold_ms"`
	DecodeHoldMs          int     `json:"decode_hold_ms"`
	RecordDropped         int     `json:"record_dropped"`
	DecodeDropped         int     `json:"decode_dropped"`
	RecordHoldSelected    int     `json:"record_hold_selected"`
	DecodeHoldSelected    int     `json:"decode_hold_selected"`
	RecordHoldProtected   int     `json:"record_hold_protected"`
	DecodeHoldProtected   int     `json:"decode_hold_protected"`
	RecordHoldExpired     int     `json:"record_hold_expired"`
	DecodeHoldExpired     int     `json:"decode_hold_expired"`
	RecordHoldDisplaced   int     `json:"record_hold_displaced"`
	DecodeHoldDisplaced   int     `json:"decode_hold_displaced"`
	RecordOpportunistic   int     `json:"record_opportunistic"`
	DecodeOpportunistic   int     `json:"decode_opportunistic"`
	RecordDisplaced       int     `json:"record_displaced"`
	DecodeDisplaced       int     `json:"decode_displaced"`
	RecordDisplacedByHold int     `json:"record_displaced_by_hold,omitempty"`
	DecodeDisplacedByHold int     `json:"decode_displaced_by_hold,omitempty"`
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
	selected        map[int64]struct{}
	held            map[int64]struct{}
	protected       map[int64]struct{}
	displacedByHold map[int64]struct{}
	displaced       map[int64]struct{}
	opportunistic   map[int64]struct{}
	expired         map[int64]struct{}
	scores          map[int64]float64
	tiers           map[int64]string
	families        map[int64]string
	familyRanks     map[int64]int
	tierFloors      map[int64]string
	minScore        float64
	maxScore        float64
	cutoff          float64
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

	recExpired := expireHold(dq.recordHold, now)
	decExpired := expireHold(dq.decodeHold, now)

	recSelected := selectQueued("record", dq.record, dq.recordHold, budget.Record.Max, recordHold, now, policy, recExpired)
	decSelected := selectQueued("decode", dq.decode, dq.decodeHold, budget.Decode.Max, decodeHold, now, policy, decExpired)
	recPressure := buildQueuePressure(budget.Record, len(dq.record), len(recSelected.selected), len(dq.recordHold))
	decPressure := buildQueuePressure(budget.Decode, len(dq.decode), len(decSelected.selected), len(dq.decodeHold))
	recPressureTag := pressureReasonTag(recPressure)
	decPressureTag := pressureReasonTag(decPressure)

	stats := DecisionQueueStats{
		RecordQueued:          len(dq.record),
		DecodeQueued:          len(dq.decode),
		RecordSelected:        len(recSelected.selected),
		DecodeSelected:        len(decSelected.selected),
		RecordActive:          len(dq.recordHold),
		DecodeActive:          len(dq.decodeHold),
		RecordOldestS:         oldestAge(dq.record, now),
		DecodeOldestS:         oldestAge(dq.decode, now),
		RecordBudget:          budget.Record.Max,
		DecodeBudget:          budget.Decode.Max,
		HoldMs:                holdPolicy.BaseMs,
		DecisionHoldMs:        holdPolicy.BaseMs,
		RecordHoldMs:          holdPolicy.RecordMs,
		DecodeHoldMs:          holdPolicy.DecodeMs,
		RecordHoldSelected:    len(recSelected.held) - len(recSelected.displaced),
		DecodeHoldSelected:    len(decSelected.held) - len(decSelected.displaced),
		RecordHoldProtected:   len(recSelected.protected),
		DecodeHoldProtected:   len(decSelected.protected),
		RecordHoldExpired:     len(recExpired),
		DecodeHoldExpired:     len(decExpired),
		RecordHoldDisplaced:   len(recSelected.displaced),
		DecodeHoldDisplaced:   len(decSelected.displaced),
		RecordOpportunistic:   len(recSelected.opportunistic),
		DecodeOpportunistic:   len(decSelected.opportunistic),
		RecordDisplaced:       len(recSelected.displacedByHold),
		DecodeDisplaced:       len(decSelected.displacedByHold),
		RecordDisplacedByHold: len(recSelected.displacedByHold),
		DecodeDisplacedByHold: len(decSelected.displacedByHold),
	}

	for i := range decisions {
		id := decisions[i].Candidate.ID
		if decisions[i].ShouldRecord {
			decisions[i].RecordAdmission = buildQueueAdmission("record", id, recSelected, policy, holdPolicy, budget.Record.Source, recPressureTag)
			if _, ok := recSelected.selected[id]; !ok {
				decisions[i].ShouldRecord = false
				extras := []string{recPressureTag, "pressure:budget", "budget:" + slugToken(budget.Record.Source)}
				if _, ok := recSelected.displaced[id]; ok {
					extras = []string{recPressureTag, "pressure:hold", ReasonTagDisplaceOpportunist, ReasonTagDisplaceTier, ReasonTagHoldDisplaced, "budget:" + slugToken(budget.Record.Source)}
				} else if _, ok := recSelected.displacedByHold[id]; ok {
					extras = []string{recPressureTag, "pressure:hold", ReasonTagHoldActive, "budget:" + slugToken(budget.Record.Source)}
				} else if _, ok := recSelected.expired[id]; ok {
					extras = append(extras, ReasonTagHoldExpired)
				}
				decisions[i].Reason = admissionReason(DecisionReasonQueueRecord, policy, holdPolicy, extras...)
				stats.RecordDropped++
			}
		}
		if decisions[i].ShouldAutoDecode {
			decisions[i].DecodeAdmission = buildQueueAdmission("decode", id, decSelected, policy, holdPolicy, budget.Decode.Source, decPressureTag)
			if _, ok := decSelected.selected[id]; !ok {
				decisions[i].ShouldAutoDecode = false
				if decisions[i].Reason == "" {
					extras := []string{decPressureTag, "pressure:budget", "budget:" + slugToken(budget.Decode.Source)}
					if _, ok := decSelected.displaced[id]; ok {
						extras = []string{decPressureTag, "pressure:hold", ReasonTagDisplaceOpportunist, ReasonTagDisplaceTier, ReasonTagHoldDisplaced, "budget:" + slugToken(budget.Decode.Source)}
					} else if _, ok := decSelected.displacedByHold[id]; ok {
						extras = []string{decPressureTag, "pressure:hold", ReasonTagHoldActive, "budget:" + slugToken(budget.Decode.Source)}
					} else if _, ok := decSelected.expired[id]; ok {
						extras = append(extras, ReasonTagHoldExpired)
					}
					decisions[i].Reason = admissionReason(DecisionReasonQueueDecode, policy, holdPolicy, extras...)
				}
				stats.DecodeDropped++
			}
		}
	}
	return stats
}

func selectQueued(queueName string, queue map[int64]*queuedDecision, hold map[int64]time.Time, max int, holdDur time.Duration, now time.Time, policy Policy, expired map[int64]struct{}) queueSelection {
	selection := queueSelection{
		selected:        map[int64]struct{}{},
		held:            map[int64]struct{}{},
		protected:       map[int64]struct{}{},
		displacedByHold: map[int64]struct{}{},
		displaced:       map[int64]struct{}{},
		opportunistic:   map[int64]struct{}{},
		expired:         map[int64]struct{}{},
		scores:          map[int64]float64{},
		tiers:           map[int64]string{},
		families:        map[int64]string{},
		familyRanks:     map[int64]int{},
		tierFloors:      map[int64]string{},
	}
	if len(queue) == 0 {
		return selection
	}
	for id := range expired {
		selection.expired[id] = struct{}{}
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
		family, familyRank := signalPriorityMatch(policy, qd.Hint, qd.Class)
		selection.families[id] = family
		selection.familyRanks[id] = familyRank
		selection.tierFloors[id] = signalPriorityTierFloor(familyRank)
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
	for id, score := range selection.scores {
		baseTier := PriorityTierFromRange(score, selection.minScore, selection.maxScore)
		selection.tiers[id] = applyTierFloor(baseTier, selection.tierFloors[id])
	}
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
			if isProtectedTier(selection.tiers[id]) {
				selection.protected[id] = struct{}{}
			}
		}
	}
	displaceable := buildDisplaceableHold(selection.held, selection.protected, selection.tiers, selection.scores, selection.familyRanks)
	for _, s := range scoredList {
		if _, ok := selection.selected[s.id]; ok {
			continue
		}
		if len(selection.selected) < limit {
			selection.selected[s.id] = struct{}{}
			continue
		}
		if len(displaceable) == 0 {
			continue
		}
		target := displaceable[0]
		if priorityTierRank(selection.tiers[s.id]) <= priorityTierRank(selection.tiers[target]) {
			continue
		}
		displaceable = displaceable[1:]
		delete(selection.selected, target)
		selection.displaced[target] = struct{}{}
		selection.selected[s.id] = struct{}{}
		selection.opportunistic[s.id] = struct{}{}
	}
	if holdDur > 0 {
		for id := range selection.displaced {
			delete(hold, id)
		}
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
	if len(selection.selected) > 0 {
		for id := range selection.scores {
			if _, ok := selection.selected[id]; ok {
				continue
			}
			if _, ok := selection.displaced[id]; ok {
				continue
			}
			if selection.scores[id] >= selection.cutoff {
				selection.displacedByHold[id] = struct{}{}
			}
		}
	}
	return selection
}

func buildDisplaceableHold(held map[int64]struct{}, protected map[int64]struct{}, tiers map[int64]string, scores map[int64]float64, familyRanks map[int64]int) []int64 {
	type entry struct {
		id          int64
		rank        int
		familyOrder int
		score       float64
	}
	candidates := make([]entry, 0, len(held))
	for id := range held {
		if _, ok := protected[id]; ok {
			continue
		}
		score := 0.0
		if scores != nil {
			score = scores[id]
		}
		familyRank := -1
		if familyRanks != nil {
			familyRank = familyRanks[id]
		}
		candidates = append(candidates, entry{
			id:          id,
			rank:        priorityTierRank(tiers[id]),
			familyOrder: familyDisplaceOrder(familyRank),
			score:       score,
		})
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].rank == candidates[j].rank {
			if candidates[i].familyOrder != candidates[j].familyOrder {
				return candidates[i].familyOrder > candidates[j].familyOrder
			}
			return candidates[i].score < candidates[j].score
		}
		return candidates[i].rank < candidates[j].rank
	})
	out := make([]int64, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.id)
	}
	return out
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
		Tier:   selection.tiers[id],
	}
	admission.TierFloor = selection.tierFloors[id]
	admission.Family = selection.families[id]
	admission.FamilyRank = familyRankForOutput(selection.familyRanks[id])
	if _, ok := selection.selected[id]; ok {
		if _, held := selection.held[id]; held {
			admission.Class = AdmissionClassHold
			extras := []string{pressureTag, "pressure:hold", ReasonTagHoldActive, "budget:" + slugToken(budgetSource)}
			if _, ok := selection.protected[id]; ok {
				extras = append(extras, ReasonTagHoldProtected)
			}
			admission.Reason = admissionReason("queue:"+queueName+":hold", policy, holdPolicy, extras...)
		} else {
			admission.Class = AdmissionClassAdmit
			extras := []string{pressureTag, "budget:" + slugToken(budgetSource)}
			if _, ok := selection.opportunistic[id]; ok {
				extras = append(extras, "pressure:hold", ReasonTagDisplaceOpportunist, ReasonTagDisplaceTier, ReasonTagHoldDisplaced)
			}
			admission.Reason = admissionReason("queue:"+queueName+":admit", policy, holdPolicy, extras...)
		}
		return admission
	}
	if _, ok := selection.displaced[id]; ok {
		admission.Class = AdmissionClassDisplace
		admission.Reason = admissionReason("queue:"+queueName+":displace", policy, holdPolicy, pressureTag, "pressure:hold", ReasonTagDisplaceOpportunist, ReasonTagDisplaceTier, ReasonTagHoldDisplaced, "budget:"+slugToken(budgetSource))
		return admission
	}
	if _, ok := selection.displacedByHold[id]; ok {
		admission.Class = AdmissionClassDisplace
		admission.Reason = admissionReason("queue:"+queueName+":displace", policy, holdPolicy, pressureTag, "pressure:hold", ReasonTagHoldActive, "budget:"+slugToken(budgetSource))
		return admission
	}
	admission.Class = AdmissionClassDefer
	extras := []string{pressureTag, "pressure:budget", "budget:" + slugToken(budgetSource)}
	if _, ok := selection.expired[id]; ok {
		extras = append(extras, ReasonTagHoldExpired)
	}
	admission.Reason = admissionReason("queue:"+queueName+":budget", policy, holdPolicy, extras...)
	return admission
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

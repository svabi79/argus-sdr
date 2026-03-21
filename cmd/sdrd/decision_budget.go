package main

import (
	"sort"
	"time"

	"sdr-wideband-suite/internal/pipeline"
)

type decisionQueueStats struct {
	RecordQueued   int     `json:"record_queued"`
	DecodeQueued   int     `json:"decode_queued"`
	RecordSelected int     `json:"record_selected"`
	DecodeSelected int     `json:"decode_selected"`
	RecordOldestS  float64 `json:"record_oldest_sec"`
	DecodeOldestS  float64 `json:"decode_oldest_sec"`
}

type queuedDecision struct {
	ID        int64
	SNRDb     float64
	FirstSeen time.Time
	LastSeen  time.Time
}

type decisionQueues struct {
	record map[int64]*queuedDecision
	decode map[int64]*queuedDecision
}

func newDecisionQueues() *decisionQueues {
	return &decisionQueues{record: map[int64]*queuedDecision{}, decode: map[int64]*queuedDecision{}}
}

func (dq *decisionQueues) Apply(decisions []pipeline.SignalDecision, maxRecord int, maxDecode int, now time.Time) decisionQueueStats {
	if dq == nil {
		return decisionQueueStats{}
	}
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

	recSelected := selectQueued(dq.record, maxRecord, now)
	decSelected := selectQueued(dq.decode, maxDecode, now)

	stats := decisionQueueStats{
		RecordQueued:   len(dq.record),
		DecodeQueued:   len(dq.decode),
		RecordSelected: len(recSelected),
		DecodeSelected: len(decSelected),
		RecordOldestS:  oldestAge(dq.record, now),
		DecodeOldestS:  oldestAge(dq.decode, now),
	}

	for i := range decisions {
		id := decisions[i].Candidate.ID
		if decisions[i].ShouldRecord {
			if _, ok := recSelected[id]; !ok {
				decisions[i].ShouldRecord = false
				decisions[i].Reason = "queued: record budget"
			}
		}
		if decisions[i].ShouldAutoDecode {
			if _, ok := decSelected[id]; !ok {
				decisions[i].ShouldAutoDecode = false
				if decisions[i].Reason == "" {
					decisions[i].Reason = "queued: decode budget"
				}
			}
		}
	}
	return stats
}

func selectQueued(queue map[int64]*queuedDecision, max int, now time.Time) map[int64]struct{} {
	selected := map[int64]struct{}{}
	if len(queue) == 0 {
		return selected
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
		scoredList = append(scoredList, scored{id: id, score: qd.SNRDb + boost})
	}
	sort.Slice(scoredList, func(i, j int) bool {
		return scoredList[i].score > scoredList[j].score
	})
	limit := max
	if limit <= 0 || limit > len(scoredList) {
		limit = len(scoredList)
	}
	for i := 0; i < limit; i++ {
		selected[scoredList[i].id] = struct{}{}
	}
	return selected
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

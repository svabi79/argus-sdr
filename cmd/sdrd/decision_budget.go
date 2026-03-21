package main

import (
	"sort"

	"sdr-wideband-suite/internal/pipeline"
)

func enforceDecisionBudgets(decisions []pipeline.SignalDecision, maxRecord int, maxDecode int) (int, int) {
	recorded := 0
	decoded := 0
	order := make([]int, len(decisions))
	for i := range decisions {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		return decisions[order[i]].Candidate.SNRDb > decisions[order[j]].Candidate.SNRDb
	})
	for _, idx := range order {
		if decisions[idx].ShouldRecord {
			if maxRecord > 0 && recorded >= maxRecord {
				decisions[idx].ShouldRecord = false
				decisions[idx].Reason = "recording budget exceeded"
			} else {
				recorded++
			}
		}
		if decisions[idx].ShouldAutoDecode {
			if maxDecode > 0 && decoded >= maxDecode {
				decisions[idx].ShouldAutoDecode = false
				if decisions[idx].Reason == "" {
					decisions[idx].Reason = "decode budget exceeded"
				}
			} else {
				decoded++
			}
		}
	}
	return recorded, decoded
}

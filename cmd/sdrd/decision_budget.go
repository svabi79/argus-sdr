package main

import "sdr-wideband-suite/internal/pipeline"

func enforceDecisionBudgets(decisions []pipeline.SignalDecision, maxRecord int, maxDecode int) (int, int) {
	recorded := 0
	decoded := 0
	for i := range decisions {
		if decisions[i].ShouldRecord {
			if maxRecord > 0 && recorded >= maxRecord {
				decisions[i].ShouldRecord = false
				decisions[i].Reason = "recording budget exceeded"
			} else {
				recorded++
			}
		}
		if decisions[i].ShouldAutoDecode {
			if maxDecode > 0 && decoded >= maxDecode {
				decisions[i].ShouldAutoDecode = false
				if decisions[i].Reason == "" {
					decisions[i].Reason = "decode budget exceeded"
				}
			} else {
				decoded++
			}
		}
	}
	return recorded, decoded
}

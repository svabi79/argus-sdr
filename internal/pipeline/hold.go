package pipeline

import "time"

func expireHold(hold map[int64]time.Time, now time.Time) map[int64]struct{} {
	if len(hold) == 0 {
		return map[int64]struct{}{}
	}
	expired := map[int64]struct{}{}
	for id, until := range hold {
		if now.After(until) {
			expired[id] = struct{}{}
			delete(hold, id)
		}
	}
	return expired
}

func isProtectedTier(tier string) bool {
	return priorityTierRank(tier) >= priorityTierRank(PriorityTierHigh)
}

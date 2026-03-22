package pipeline

func monitorBounds(policy Policy) (float64, float64, bool) {
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
	start, end, ok := monitorBounds(policy)
	if !ok {
		return true
	}
	left := candidate.CenterHz
	right := candidate.CenterHz
	if candidate.BandwidthHz > 0 {
		left = candidate.CenterHz - candidate.BandwidthHz/2
		right = candidate.CenterHz + candidate.BandwidthHz/2
	}
	return right >= start && left <= end
}

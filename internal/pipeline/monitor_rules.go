package pipeline

func candidateInMonitor(policy Policy, candidate Candidate) bool {
	start := policy.MonitorStartHz
	end := policy.MonitorEndHz
	if start == 0 || end == 0 || end <= start {
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

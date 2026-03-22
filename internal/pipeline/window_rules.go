package pipeline

import "strings"

// AutoSpanForHint returns a suggested refinement span based on the candidate hint.
// It is intentionally conservative: spans are wide enough for robust demod/classify,
// but not so wide that refinement becomes wasteful.
func AutoSpanForHint(hint string) (float64, string) {
	h := strings.ToLower(hint)
	switch {
	case strings.Contains(h, "wfm"):
		return 200000, "auto:wfm"
	case strings.Contains(h, "nfm"):
		return 18000, "auto:nfm"
	case strings.Contains(h, "usb") || strings.Contains(h, "lsb") || strings.Contains(h, "ssb"):
		return 6000, "auto:ssb"
	case strings.Contains(h, "cw"):
		return 500, "auto:cw"
	case strings.Contains(h, "dmr") || strings.Contains(h, "d-star") || strings.Contains(h, "dstar"):
		return 15000, "auto:dig_voice"
	case strings.Contains(h, "ft8") || strings.Contains(h, "wspr"):
		return 4000, "auto:dig_weak"
	case strings.Contains(h, "fsk") || strings.Contains(h, "psk"):
		return 6000, "auto:dig"
	case strings.Contains(h, "am"):
		return 12000, "auto:am"
	default:
		return 0, ""
	}
}

// RefinementWindowForCandidate applies policy-aware span rules to a candidate.
func RefinementWindowForCandidate(policy Policy, candidate Candidate) RefinementWindow {
	span := candidate.BandwidthHz
	windowSource := "candidate"
	if policy.RefinementAutoSpan && (span <= 0 || span < 2000 || span > 400000) {
		autoSpan, autoSource := AutoSpanForHint(candidate.Hint)
		if autoSpan > 0 {
			span = autoSpan
			windowSource = autoSource
		}
	}
	if policy.RefinementMinSpanHz > 0 && span < policy.RefinementMinSpanHz {
		span = policy.RefinementMinSpanHz
	}
	if policy.RefinementMaxSpanHz > 0 && span > policy.RefinementMaxSpanHz {
		span = policy.RefinementMaxSpanHz
	}
	if span <= 0 {
		span = 12000
	}
	return RefinementWindow{
		CenterHz: candidate.CenterHz,
		SpanHz:   span,
		Source:   windowSource,
	}
}

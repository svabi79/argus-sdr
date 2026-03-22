package pipeline

import "testing"

func TestRefinementWindowClampsToPolicy(t *testing.T) {
	policy := Policy{RefinementMinSpanHz: 12000, RefinementMaxSpanHz: 20000, RefinementAutoSpan: false}
	win := RefinementWindowForCandidate(policy, Candidate{CenterHz: 1e6, BandwidthHz: 8000})
	if win.SpanHz != 12000 || win.Source != "policy:min_span" {
		t.Fatalf("expected min clamp, got span %.0f source %q", win.SpanHz, win.Source)
	}
	win = RefinementWindowForCandidate(policy, Candidate{CenterHz: 1e6, BandwidthHz: 50000})
	if win.SpanHz != 20000 || win.Source != "policy:max_span" {
		t.Fatalf("expected max clamp, got span %.0f source %q", win.SpanHz, win.Source)
	}
}

func TestRefinementWindowDefaultsWhenEmpty(t *testing.T) {
	policy := Policy{RefinementAutoSpan: false}
	win := RefinementWindowForCandidate(policy, Candidate{CenterHz: 1e6, BandwidthHz: 0})
	if win.SpanHz != 12000 || win.Source != "default" {
		t.Fatalf("expected default span, got span %.0f source %q", win.SpanHz, win.Source)
	}
}

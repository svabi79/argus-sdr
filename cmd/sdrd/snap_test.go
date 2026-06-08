package main

import "testing"

func TestSnapOutRateIntegerDecimation(t *testing.T) {
	cases := []struct{ sr, target int }{
		{4096000, 200000}, // live NFM: 20.48 -> must snap to a divisor
		{4096000, 512000}, // live WFM: 8.0 exact
		{2000000, 200000}, // 2M capture
		{2048000, 200000},
		{4000000, 200000},
	}
	for _, c := range cases {
		got := snapOutRate(c.sr, c.target)
		if got <= 0 || c.sr%got != 0 {
			t.Errorf("snapOutRate(%d,%d)=%d: not an integer divisor (sr%%out=%d)", c.sr, c.target, got, c.sr%got)
		}
		decim := c.sr / got
		// within ~30% of target (nearest divisor)
		ratio := float64(got) / float64(c.target)
		if ratio < 0.7 || ratio > 1.4 {
			t.Errorf("snapOutRate(%d,%d)=%d (decim %d): too far from target", c.sr, c.target, got, decim)
		}
		t.Logf("snapOutRate(%d,%d)=%d decim=%d", c.sr, c.target, got, decim)
	}
}

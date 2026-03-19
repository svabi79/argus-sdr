package cfar

import "math"

// cellAvg implements CA-CFAR with a sliding sum window.
type cellAvg struct {
	guard      int
	train      int
	scaleDb    float64
	wrapAround bool
}

func newCA(cfg Config) CFAR {
	return &cellAvg{
		guard:      cfg.GuardCells,
		train:      cfg.TrainCells,
		scaleDb:    cfg.ScaleDb,
		wrapAround: cfg.WrapAround,
	}
}

func (c *cellAvg) Thresholds(spectrum []float64) []float64 {
	n := len(spectrum)
	if n == 0 {
		return nil
	}
	out := make([]float64, n)
	train := c.train
	guard := c.guard
	total := 2 * train
	if total == 0 {
		return out
	}

	at := func(i int) float64 {
		if c.wrapAround {
			return spectrum[((i%n)+n)%n]
		}
		if i < 0 || i >= n {
			return spectrum[clampInt(i, 0, n-1)]
		}
		return spectrum[i]
	}

	toLinear := func(db float64) float64 {
		return math.Pow(10, db/10.0)
	}

	var leftSum, rightSum float64
	for k := 1; k <= train; k++ {
		leftSum += toLinear(at(0 - guard - k))
		rightSum += toLinear(at(0 + guard + k))
	}

	invN := 1.0 / float64(total)
	out[0] = 10*math.Log10((leftSum+rightSum)*invN) + c.scaleDb

	for i := 1; i < n; i++ {
		leftSum -= toLinear(at(i - 1 - guard - train))
		leftSum += toLinear(at(i - guard - 1))

		rightSum -= toLinear(at(i - 1 + guard + 1))
		rightSum += toLinear(at(i + guard + train))

		out[i] = 10*math.Log10((leftSum+rightSum)*invN) + c.scaleDb
	}
	return out
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

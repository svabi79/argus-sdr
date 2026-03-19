package cfar

import "math"

// gosca implements Greatest-Of Selection with Cell Averaging.
type gosca struct {
	guard      int
	train      int
	scaleDb    float64
	wrapAround bool
}

func newGOSCA(cfg Config) CFAR {
	return &gosca{
		guard:      cfg.GuardCells,
		train:      cfg.TrainCells,
		scaleDb:    cfg.ScaleDb,
		wrapAround: cfg.WrapAround,
	}
}

func (g *gosca) Thresholds(spectrum []float64) []float64 {
	n := len(spectrum)
	if n == 0 {
		return nil
	}
	out := make([]float64, n)
	train := g.train
	guard := g.guard
	if train == 0 {
		return out
	}
	inv := 1.0 / float64(train)

	at := func(i int) float64 {
		if g.wrapAround {
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

	leftMean := leftSum * inv
	rightMean := rightSum * inv
	noise := leftMean
	if rightMean > noise {
		noise = rightMean
	}
	out[0] = 10*math.Log10(noise) + g.scaleDb

	for i := 1; i < n; i++ {
		leftSum -= toLinear(at(i - 1 - guard - train))
		leftSum += toLinear(at(i - guard - 1))
		rightSum -= toLinear(at(i - 1 + guard + 1))
		rightSum += toLinear(at(i + guard + train))

		leftMean = leftSum * inv
		rightMean = rightSum * inv
		noise = leftMean
		if rightMean > noise {
			noise = rightMean
		}
		out[i] = 10*math.Log10(noise) + g.scaleDb
	}
	return out
}

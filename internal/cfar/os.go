package cfar

import "sort"

type orderedStat struct {
	guard      int
	train      int
	rank       int
	scaleDb    float64
	wrapAround bool
}

func newOS(cfg Config) CFAR {
	rank := cfg.Rank - 1
	total := 2 * cfg.TrainCells
	if rank < 0 {
		rank = 0
	}
	if rank >= total {
		rank = total - 1
	}
	return &orderedStat{
		guard:      cfg.GuardCells,
		train:      cfg.TrainCells,
		rank:       rank,
		scaleDb:    cfg.ScaleDb,
		wrapAround: cfg.WrapAround,
	}
}

func (o *orderedStat) Thresholds(spectrum []float64) []float64 {
	n := len(spectrum)
	if n == 0 {
		return nil
	}
	out := make([]float64, n)
	train := o.train
	guard := o.guard

	at := func(i int) float64 {
		if o.wrapAround {
			return spectrum[((i%n)+n)%n]
		}
		if i < 0 || i >= n {
			return spectrum[clampInt(i, 0, n-1)]
		}
		return spectrum[i]
	}

	win := make([]float64, 0, 2*train)
	for k := 1; k <= train; k++ {
		win = append(win, at(0-guard-k))
		win = append(win, at(0+guard+k))
	}
	sort.Float64s(win)
	out[0] = win[o.rank] + o.scaleDb

	rebuildWindow := func(bin int) {
		win = win[:0]
		for k := 1; k <= train; k++ {
			win = append(win, at(bin-guard-k))
			win = append(win, at(bin+guard+k))
		}
		sort.Float64s(win)
	}

	for i := 1; i < n; i++ {
		removeFromSorted(&win, at(i-1-guard-train))
		removeFromSorted(&win, at(i-1+guard+1))

		insertSorted(&win, at(i-guard-1))
		insertSorted(&win, at(i+guard+train))

		if len(win) != 2*train {
			rebuildWindow(i)
		}
		out[i] = win[o.rank] + o.scaleDb
	}
	return out
}

func insertSorted(s *[]float64, v float64) {
	idx := sort.SearchFloat64s(*s, v)
	*s = append(*s, 0)
	copy((*s)[idx+1:], (*s)[idx:])
	(*s)[idx] = v
}

func removeFromSorted(s *[]float64, v float64) {
	idx := sort.SearchFloat64s(*s, v)
	if idx < len(*s) && (*s)[idx] == v {
		*s = append((*s)[:idx], (*s)[idx+1:]...)
		return
	}
	for i := idx - 1; i >= 0; i-- {
		if (*s)[i] == v {
			*s = append((*s)[:i], (*s)[i+1:]...)
			return
		}
		if (*s)[i] < v {
			break
		}
	}
	for i := idx + 1; i < len(*s); i++ {
		if (*s)[i] == v {
			*s = append((*s)[:i], (*s)[i+1:]...)
			return
		}
		if (*s)[i] > v {
			break
		}
	}
}

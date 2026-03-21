package mock

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"sdr-wideband-suite/internal/sdr"
)

type Source struct {
	mu         sync.Mutex
	phase      float64
	phase2     float64
	phase3     float64
	sampleRate float64
	noise      float64
	rng        *rand.Rand
}

func New(sampleRate int) *Source {
	return &Source{
		sampleRate: float64(sampleRate),
		noise:      0.02,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *Source) Start() error { return nil }
func (s *Source) Stop() error  { return nil }

func (s *Source) UpdateConfig(sampleRate int, centerHz float64, gainDb float64, agc bool, bwKHz int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sampleRate > 0 {
		s.sampleRate = float64(sampleRate)
	}
	return nil
}

func (s *Source) ReadIQ(n int) ([]complex64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]complex64, n)
	f1 := 50e3
	f2 := -120e3
	f3 := 300e3
	const twoPi = 2 * math.Pi
	for i := 0; i < n; i++ {
		s.phase += twoPi * f1 / s.sampleRate
		s.phase2 += twoPi * f2 / s.sampleRate
		s.phase3 += twoPi * f3 / s.sampleRate
		if s.phase > twoPi {
			s.phase -= twoPi
		}
		if s.phase2 > twoPi {
			s.phase2 -= twoPi
		}
		if s.phase2 < 0 {
			s.phase2 += twoPi
		}
		if s.phase3 > twoPi {
			s.phase3 -= twoPi
		}
		re := math.Cos(s.phase) + 0.7*math.Cos(s.phase2) + 0.4*math.Cos(s.phase3)
		im := math.Sin(s.phase) + 0.7*math.Sin(s.phase2) + 0.4*math.Sin(s.phase3)
		re += s.noise * s.rng.NormFloat64()
		im += s.noise * s.rng.NormFloat64()
		out[i] = complex(float32(re), float32(im))
	}
	return out, nil
}

func (s *Source) Stats() sdr.SourceStats {
	return sdr.SourceStats{}
}

func (s *Source) Flush() {}

package sdr

import "errors"

type Source interface {
	Start() error
	Stop() error
	ReadIQ(n int) ([]complex64, error)
}

type ConfigurableSource interface {
	UpdateConfig(sampleRate int, centerHz float64, gainDb float64, agc bool, bwKHz int) error
}

type SourceStats struct {
	BufferSamples   int    `json:"buffer_samples"`
	Dropped         uint64 `json:"dropped"`
	Resets          uint64 `json:"resets"`
	LastSampleAgoMs int64  `json:"last_sample_ago_ms"`
}

type StatsProvider interface {
	Stats() SourceStats
}

type Flushable interface {
	Flush()
}

var ErrNotImplemented = errors.New("sdrplay support not built; build with -tags sdrplay or use --mock")

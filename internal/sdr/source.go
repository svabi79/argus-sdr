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

var ErrNotImplemented = errors.New("sdrplay support not built; build with -tags sdrplay or use --mock")

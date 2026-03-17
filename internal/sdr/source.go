package sdr

import "errors"

type Source interface {
	Start() error
	Stop() error
	ReadIQ(n int) ([]complex64, error)
}

var ErrNotImplemented = errors.New("sdrplay support not built; build with -tags sdrplay or use --mock")

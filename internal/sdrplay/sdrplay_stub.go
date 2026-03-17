//go:build !sdrplay

package sdrplay

import "sdr-visual-suite/internal/sdr"

func New(sampleRate int, centerHz float64, gainDb float64) (sdr.Source, error) {
	return nil, sdr.ErrNotImplemented
}

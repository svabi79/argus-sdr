//go:build !cufft

package gpudemod

import "errors"

type DemodType int

const (
	DemodNFM DemodType = iota
	DemodWFM
	DemodAM
	DemodUSB
	DemodLSB
	DemodCW
)

type Engine struct {
	maxSamples       int
	sampleRate       int
	lastShiftUsedGPU bool
}

func Available() bool { return false }

func New(maxSamples int, sampleRate int) (*Engine, error) {
	return nil, errors.New("CUDA demod not available: cufft build tag not enabled")
}

func (e *Engine) SetFIR(taps []float32) {}

func (e *Engine) LastShiftUsedGPU() bool { return false }
func (e *Engine) LastDemodUsedGPU() bool { return false }

func (e *Engine) Demod(iq []complex64, offsetHz float64, bw float64, mode DemodType) ([]float32, int, error) {
	return nil, 0, errors.New("CUDA demod not available: cufft build tag not enabled")
}

func (e *Engine) Close() {}

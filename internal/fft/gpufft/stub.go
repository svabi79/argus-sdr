//go:build !cufft

package gpufft

import "errors"

type Engine struct{}

func Available() bool { return false }

func New(n int) (*Engine, error) {
	return nil, errors.New("cufft build tag not enabled")
}

func (e *Engine) Close() {}

func (e *Engine) Exec(in []complex64) ([]complex64, error) {
	return nil, errors.New("cufft build tag not enabled")
}

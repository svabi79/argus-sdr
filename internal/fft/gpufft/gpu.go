//go:build cufft

package gpufft

/*
#cgo windows LDFLAGS: -lcufft -lcudart
#include <cuda_runtime.h>
#include <cufft.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

type Engine struct {
	plan  C.cufftHandle
	n     int
	data  *C.cufftComplex
	bytes C.size_t
}

func Available() bool {
	var count C.int
	if C.cudaGetDeviceCount(&count) != C.cudaSuccess {
		return false
	}
	return count > 0
}

func New(n int) (*Engine, error) {
	if n <= 0 {
		return nil, errors.New("invalid fft size")
	}
	if !Available() {
		return nil, errors.New("cuda device not available")
	}
	var plan C.cufftHandle
	if C.cufftPlan1d(&plan, C.int(n), C.CUFFT_C2C, 1) != C.CUFFT_SUCCESS {
		return nil, errors.New("cufftPlan1d failed")
	}
	var ptr unsafe.Pointer
	bytes := C.size_t(n) * C.size_t(unsafe.Sizeof(C.cufftComplex{}))
	if C.cudaMalloc(&ptr, bytes) != C.cudaSuccess {
		C.cufftDestroy(plan)
		return nil, errors.New("cudaMalloc failed")
	}
	return &Engine{plan: plan, n: n, data: (*C.cufftComplex)(ptr), bytes: bytes}, nil
}

func (e *Engine) Close() {
	if e == nil {
		return
	}
	if e.plan != 0 {
		_ = C.cufftDestroy(e.plan)
		e.plan = 0
	}
	if e.data != nil {
		_ = C.cudaFree(unsafe.Pointer(e.data))
		e.data = nil
	}
}

func (e *Engine) Exec(in []complex64) ([]complex64, error) {
	if e == nil {
		return nil, errors.New("gpu fft not initialized")
	}
	if len(in) != e.n {
		return nil, fmt.Errorf("expected %d samples, got %d", e.n, len(in))
	}
	if len(in) == 0 {
		return nil, nil
	}
	if C.cudaMemcpy(unsafe.Pointer(e.data), unsafe.Pointer(&in[0]), e.bytes, C.cudaMemcpyHostToDevice) != C.cudaSuccess {
		return nil, errors.New("cudaMemcpy H2D failed")
	}
	if C.cufftExecC2C(e.plan, e.data, e.data, C.CUFFT_FORWARD) != C.CUFFT_SUCCESS {
		return nil, errors.New("cufftExecC2C failed")
	}
	if C.cudaMemcpy(unsafe.Pointer(&in[0]), unsafe.Pointer(e.data), e.bytes, C.cudaMemcpyDeviceToHost) != C.cudaSuccess {
		return nil, errors.New("cudaMemcpy D2H failed")
	}
	_ = C.cudaDeviceSynchronize()
	return in, nil
}

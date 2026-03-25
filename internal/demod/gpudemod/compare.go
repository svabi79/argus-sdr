package gpudemod

import "math/cmplx"

type CompareStats struct {
	MaxAbsErr float64
	RMSErr    float64
	Count     int
}

func CompareComplexSlices(a []complex64, b []complex64) CompareStats {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return CompareStats{}
	}
	var sumSq float64
	var maxAbs float64
	for i := 0; i < n; i++ {
		err := cmplx.Abs(complex128(a[i] - b[i]))
		if err > maxAbs {
			maxAbs = err
		}
		sumSq += err * err
	}
	return CompareStats{
		MaxAbsErr: maxAbs,
		RMSErr:    mathSqrt(sumSq / float64(n)),
		Count:     n,
	}
}

func mathSqrt(v float64) float64 {
	// tiny shim to keep the compare helper self-contained and easy to move
	// without importing additional logic elsewhere
	z := v
	if z <= 0 {
		return 0
	}
	x := z
	for i := 0; i < 12; i++ {
		x = 0.5 * (x + z/x)
	}
	return x
}

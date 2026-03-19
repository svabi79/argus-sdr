//go:build cufft

package gpudemod

import "testing"

func TestValidateDecimateRejectsBadLength(t *testing.T) {
	iq := []complex64{1 + 0i, 2 + 0i, 3 + 0i, 4 + 0i}
	out := []complex64{1 + 0i}
	if ValidateDecimate(iq, 2, out, 1e-6) {
		t.Fatal("expected decimate validation to fail on bad length")
	}
}

func TestValidateFIRRejectsBadLength(t *testing.T) {
	iq := []complex64{1 + 0i, 2 + 0i}
	taps := []float32{1}
	out := []complex64{1 + 0i}
	if ValidateFIR(iq, taps, out, 1e-6) {
		t.Fatal("expected FIR validation to fail on bad length")
	}
}

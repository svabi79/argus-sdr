//go:build cufft

package gpudemod

import (
	"testing"

	"sdr-visual-suite/internal/dsp"
)

func TestValidateFreqShiftRejectsMismatchedLength(t *testing.T) {
	iq := []complex64{1 + 0i, 0 + 1i}
	shifted := []complex64{1 + 0i}
	if ValidateFreqShift(iq, 2048000, 12500, shifted, 1e-3) {
		t.Fatal("expected mismatched lengths to fail validation")
	}
}

func TestValidateFreqShiftAcceptsCPUReference(t *testing.T) {
	iq := []complex64{1 + 0i, 0.5 + 0.25i, -0.25 + 0.75i, 0.1 - 0.3i}
	shifted := dsp.FreqShift(iq, 2048000, 256000)
	if !ValidateFreqShift(iq, 2048000, 256000, shifted, 1e-6) {
		t.Fatal("expected CPU reference shifted IQ to pass validation")
	}
}

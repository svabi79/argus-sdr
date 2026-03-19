package gpudemod

import "testing"

func TestStubAvailableFalseWithoutCufft(t *testing.T) {
	if Available() {
		t.Fatal("expected CUDA demod to be unavailable without cufft build tag")
	}
}

func TestStubNewReturnsErrorWithoutCufft(t *testing.T) {
	if _, err := New(4096, 2048000); err == nil {
		t.Fatal("expected New to fail without cufft build tag")
	}
}

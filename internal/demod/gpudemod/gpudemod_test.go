package gpudemod

import "testing"

func TestStubAvailableFalseWithoutCufft(t *testing.T) {
	if Available() {
		t.Fatal("expected CUDA demod to be unavailable without cufft build tag")
	}
}

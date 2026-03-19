//go:build cufft

package gpudemod

import "testing"

func TestDemodTypeConstantsExist(t *testing.T) {
	if DemodNFM != 0 {
		t.Fatal("expected DemodNFM constant to be defined")
	}
}

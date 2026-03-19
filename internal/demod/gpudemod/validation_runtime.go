//go:build cufft

package gpudemod

import "os"

func validationEnabled() bool {
	return os.Getenv("SDR_GPU_VALIDATE") == "1"
}

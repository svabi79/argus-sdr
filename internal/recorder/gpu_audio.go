package recorder

import (
	"log"

	"sdr-visual-suite/internal/demod/gpudemod"
)

func tryGPUAudio(gpu *gpudemod.Engine, label string, iq []complex64, offset float64, bw float64, gpuMode gpudemod.DemodType) ([]float32, int, bool) {
	if gpu == nil {
		return nil, 0, false
	}
	if gpuAudio, gpuRate, err := gpu.DemodFused(iq, offset, bw, gpuMode); err == nil {
		log.Printf("gpudemod: fused GPU demod used (%s)", label)
		return gpuAudio, gpuRate, true
	} else {
		log.Printf("gpudemod: fused GPU demod failed (%s): %v", label, err)
	}
	if gpuAudio, gpuRate, err := gpu.Demod(iq, offset, bw, gpuMode); err == nil {
		log.Printf("gpudemod: legacy GPU demod used (%s)", label)
		return gpuAudio, gpuRate, true
	} else {
		log.Printf("gpudemod: legacy GPU demod failed (%s): %v", label, err)
	}
	return nil, 0, false
}

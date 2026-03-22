package recorder

import (
	"log"

	"sdr-wideband-suite/internal/demod"
	"sdr-wideband-suite/internal/demod/gpudemod"
)

type wfmHybridResult struct {
	Audio     []float32
	AudioRate int
	Channels  int
	RDS       []float32
	RDSRate   int
}

func demodWFMStereoHybrid(gpu *gpudemod.Engine, iq []complex64, sampleRate int, offset float64, bw float64, deemphasisUs float64) wfmHybridResult {
	audio, rate := demodWFMStereoBatchAudio(iq, sampleRate, offset, bw, deemphasisUs)

	var rdsSamples []float32
	var rdsRate int
	if gpu != nil {
		rdsIQ, gpuRate, err := gpu.ShiftFilterDecimate(iq, 57000, 4800, 4800)
		if err == nil && len(rdsIQ) > 0 {
			rdsSamples = make([]float32, len(rdsIQ))
			for i, v := range rdsIQ {
				rdsSamples[i] = real(v)
			}
			rdsRate = gpuRate
			log.Printf("gpudemod: GPU RDS extraction used (%d samples at %d Hz)", len(rdsSamples), rdsRate)
		} else if err != nil {
			log.Printf("gpudemod: GPU RDS extraction failed: %v - CPU fallback", err)
		}
	}
	if rdsSamples == nil {
		rds := demod.RDSBasebandDecimated(iq, sampleRate)
		rdsSamples = rds.Samples
		rdsRate = rds.SampleRate
	}

	return wfmHybridResult{
		Audio:     audio,
		AudioRate: rate,
		Channels:  2,
		RDS:       rdsSamples,
		RDSRate:   rdsRate,
	}
}

package recorder

import "sdr-visual-suite/internal/demod"

type wfmHybridResult struct {
	Audio     []float32
	AudioRate int
	Channels  int
	RDS       []float32
	RDSRate   int
}

func demodWFMStereoHybrid(iq []complex64, sampleRate int, offset float64, bw float64) wfmHybridResult {
	audio, rate := demodAudioCPU(demod.Get("WFM_STEREO"), iq, sampleRate, offset, bw)
	rds := demod.RDSBaseband(iq, sampleRate)
	return wfmHybridResult{
		Audio:     audio,
		AudioRate: rate,
		Channels:  2,
		RDS:       rds,
		RDSRate:   sampleRate,
	}
}

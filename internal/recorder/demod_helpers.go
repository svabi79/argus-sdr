package recorder

import "sdr-wideband-suite/internal/demod/gpudemod"

func gpuModeFor(name string) (gpudemod.DemodType, bool) {
	switch name {
	case "NFM":
		return gpudemod.DemodNFM, true
	case "WFM", "WFM_STEREO":
		return gpudemod.DemodWFM, true
	case "AM":
		return gpudemod.DemodAM, true
	case "USB":
		return gpudemod.DemodUSB, true
	case "LSB":
		return gpudemod.DemodLSB, true
	case "CW":
		return gpudemod.DemodCW, true
	default:
		return 0, false
	}
}

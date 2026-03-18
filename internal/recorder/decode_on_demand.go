package recorder

import (
	"errors"

	"sdr-visual-suite/internal/decoder"
)

func DecodeOnDemand(cmd string, iqPath string, sampleRate int, audioPath string) (decoder.Result, error) {
	if cmd == "" {
		return decoder.Result{}, errors.New("decoder command empty")
	}
	return decoder.Run(cmd, iqPath, sampleRate, audioPath)
}

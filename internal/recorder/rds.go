package recorder

import "sdr-wideband-suite/internal/rds"

type rdsdecoder struct{ rds.Decoder }

// DecodeFloat32 wraps Decode for float32 input (converts to complex64)
func (d *rdsdecoder) DecodeFloat32(samples []float32, sampleRate int) rds.Result {
	cplx := make([]complex64, len(samples))
	for i, v := range samples {
		cplx[i] = complex(v, 0)
	}
	return d.Decode(cplx, sampleRate)
}

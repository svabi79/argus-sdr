package dsp

// StatefulDecimatingFIRComplex combines FIR filtering and decimation into a
// single stateful stage. This avoids exposing FIR settling/transient output as
// ordinary block-leading samples before decimation.
type StatefulDecimatingFIRComplex struct {
	taps   []float64
	delayR []float64
	delayI []float64
	factor int
	phase  int // number of input samples until next output sample (0 => emit now)
}

func (f *StatefulDecimatingFIRComplex) Phase() int {
	if f == nil {
		return 0
	}
	return f.phase
}

func (f *StatefulDecimatingFIRComplex) TapsLen() int {
	if f == nil {
		return 0
	}
	return len(f.taps)
}

func NewStatefulDecimatingFIRComplex(taps []float64, factor int) *StatefulDecimatingFIRComplex {
	if factor < 1 {
		factor = 1
	}
	t := make([]float64, len(taps))
	copy(t, taps)
	return &StatefulDecimatingFIRComplex{
		taps:   t,
		delayR: make([]float64, len(taps)),
		delayI: make([]float64, len(taps)),
		factor: factor,
		phase:  0,
	}
}

func (f *StatefulDecimatingFIRComplex) Reset() {
	for i := range f.delayR {
		f.delayR[i] = 0
		f.delayI[i] = 0
	}
	f.phase = 0
}

func (f *StatefulDecimatingFIRComplex) Process(iq []complex64) []complex64 {
	if len(iq) == 0 || len(f.taps) == 0 {
		return nil
	}
	if f.factor <= 1 {
		out := make([]complex64, len(iq))
		for i := 0; i < len(iq); i++ {
			copy(f.delayR[1:], f.delayR[:len(f.taps)-1])
			copy(f.delayI[1:], f.delayI[:len(f.taps)-1])
			f.delayR[0] = float64(real(iq[i]))
			f.delayI[0] = float64(imag(iq[i]))
			var accR, accI float64
			for k := 0; k < len(f.taps); k++ {
				w := f.taps[k]
				accR += f.delayR[k] * w
				accI += f.delayI[k] * w
			}
			out[i] = complex(float32(accR), float32(accI))
		}
		return out
	}

	out := make([]complex64, 0, len(iq)/f.factor+1)
	n := len(f.taps)
	for i := 0; i < len(iq); i++ {
		copy(f.delayR[1:], f.delayR[:n-1])
		copy(f.delayI[1:], f.delayI[:n-1])
		f.delayR[0] = float64(real(iq[i]))
		f.delayI[0] = float64(imag(iq[i]))

		if f.phase == 0 {
			var accR, accI float64
			for k := 0; k < n; k++ {
				w := f.taps[k]
				accR += f.delayR[k] * w
				accI += f.delayI[k] * w
			}
			out = append(out, complex(float32(accR), float32(accI)))
			f.phase = f.factor - 1
		} else {
			f.phase--
		}
	}
	return out
}

package dsp

// StatefulFIRReal is a real-valued FIR filter that preserves its delay line
// between calls to Process(). This eliminates click/pop artifacts at frame
// boundaries in streaming audio pipelines.
type StatefulFIRReal struct {
	taps  []float64
	delay []float64
	pos   int // write position in circular delay buffer
}

// NewStatefulFIRReal creates a stateful FIR filter with the given taps.
func NewStatefulFIRReal(taps []float64) *StatefulFIRReal {
	t := make([]float64, len(taps))
	copy(t, taps)
	return &StatefulFIRReal{
		taps:  t,
		delay: make([]float64, len(taps)),
	}
}

// Process filters the input through the FIR with persistent state.
// Allocates a new output slice. For zero-alloc hot paths, use ProcessInto.
func (f *StatefulFIRReal) Process(x []float32) []float32 {
	out := make([]float32, len(x))
	f.ProcessInto(x, out)
	return out
}

// ProcessInto filters into a pre-allocated output buffer.
func (f *StatefulFIRReal) ProcessInto(x []float32, out []float32) []float32 {
	if len(x) == 0 || len(f.taps) == 0 {
		return out[:0]
	}
	n := len(f.taps)
	for i := 0; i < len(x); i++ {
		copy(f.delay[1:], f.delay[:n-1])
		f.delay[0] = float64(x[i])

		var acc float64
		for k := 0; k < n; k++ {
			acc += f.delay[k] * f.taps[k]
		}
		out[i] = float32(acc)
	}
	return out[:len(x)]
}

// Reset clears the delay line.
func (f *StatefulFIRReal) Reset() {
	for i := range f.delay {
		f.delay[i] = 0
	}
}

// StatefulFIRComplex is a complex-valued FIR filter with persistent state.
type StatefulFIRComplex struct {
	taps   []float64
	delayR []float64
	delayI []float64
}

// NewStatefulFIRComplex creates a stateful complex FIR filter.
func NewStatefulFIRComplex(taps []float64) *StatefulFIRComplex {
	t := make([]float64, len(taps))
	copy(t, taps)
	return &StatefulFIRComplex{
		taps:   t,
		delayR: make([]float64, len(taps)),
		delayI: make([]float64, len(taps)),
	}
}

// Process filters complex IQ through the FIR with persistent state.
// Allocates a new output slice. For zero-alloc hot paths, use ProcessInto.
func (f *StatefulFIRComplex) Process(iq []complex64) []complex64 {
	out := make([]complex64, len(iq))
	f.ProcessInto(iq, out)
	return out
}

// ProcessInto filters complex IQ into a pre-allocated output buffer.
// out must be at least len(iq) long. Returns the used portion of out.
func (f *StatefulFIRComplex) ProcessInto(iq []complex64, out []complex64) []complex64 {
	if len(iq) == 0 || len(f.taps) == 0 {
		return out[:0]
	}
	n := len(f.taps)
	for i := 0; i < len(iq); i++ {
		copy(f.delayR[1:], f.delayR[:n-1])
		copy(f.delayI[1:], f.delayI[:n-1])
		f.delayR[0] = float64(real(iq[i]))
		f.delayI[0] = float64(imag(iq[i]))

		var accR, accI float64
		for k := 0; k < n; k++ {
			w := f.taps[k]
			accR += f.delayR[k] * w
			accI += f.delayI[k] * w
		}
		out[i] = complex(float32(accR), float32(accI))
	}
	return out[:len(iq)]
}

// Reset clears the delay line.
func (f *StatefulFIRComplex) Reset() {
	for i := range f.delayR {
		f.delayR[i] = 0
		f.delayI[i] = 0
	}
}

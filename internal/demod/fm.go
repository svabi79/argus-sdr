package demod

import (
	"math"

	"sdr-visual-suite/internal/dsp"
)

type NFM struct{}

type WFM struct{}

type WFMStereo struct{}

func (NFM) Name() string          { return "NFM" }
func (WFM) Name() string          { return "WFM" }
func (WFMStereo) Name() string    { return "WFM_STEREO" }
func (NFM) OutputSampleRate() int { return 48000 }
func (WFM) OutputSampleRate() int { return 192000 }
func (WFMStereo) OutputSampleRate() int {
	return 192000
}
func (NFM) Channels() int       { return 1 }
func (WFM) Channels() int       { return 1 }
func (WFMStereo) Channels() int { return 2 }

func wfmMonoBase(iq []complex64) []float32 {
	return fmDiscrim(iq)
}

func (NFM) Demod(iq []complex64, sampleRate int) []float32 {
	return fmDiscrim(iq)
}

func (WFM) Demod(iq []complex64, sampleRate int) []float32 {
	return wfmMonoBase(iq)
}

func (WFMStereo) Demod(iq []complex64, sampleRate int) []float32 {
	return wfmStereo(iq, sampleRate)
}

func fmDiscrim(iq []complex64) []float32 {
	if len(iq) < 2 {
		return nil
	}
	out := make([]float32, len(iq)-1)
	for i := 1; i < len(iq); i++ {
		p := iq[i-1]
		c := iq[i]
		num := float64(real(p))*float64(imag(c)) - float64(imag(p))*float64(real(c))
		den := float64(real(p))*float64(real(c)) + float64(imag(p))*float64(imag(c))
		out[i-1] = float32(math.Atan2(num, den))
	}
	return out
}

func wfmStereo(iq []complex64, sampleRate int) []float32 {
	base := fmDiscrim(iq)
	if len(base) == 0 {
		return nil
	}
	lp := dsp.LowpassFIR(15000, sampleRate, 101)
	lpr := dsp.ApplyFIRReal(base, lp)
	bpHi := dsp.LowpassFIR(53000, sampleRate, 101)
	bpLo := dsp.LowpassFIR(23000, sampleRate, 101)
	hi := dsp.ApplyFIRReal(base, bpHi)
	lo := dsp.ApplyFIRReal(base, bpLo)
	bpf := make([]float32, len(base))
	for i := range base {
		bpf[i] = hi[i] - lo[i]
	}
	lr := make([]float32, len(base))
	phase := 0.0
	inc := 2 * math.Pi * 38000 / float64(sampleRate)
	for i := range bpf {
		phase += inc
		lr[i] = bpf[i] * float32(2*math.Cos(phase))
	}
	lr = dsp.ApplyFIRReal(lr, lp)
	out := make([]float32, len(lpr)*2)
	for i := range lpr {
		l := 0.5 * (lpr[i] + lr[i])
		r := 0.5 * (lpr[i] - lr[i])
		out[i*2] = l
		out[i*2+1] = r
	}
	return out
}

// RDSBaseband returns a rough 57k baseband (not decoded).
func RDSBaseband(iq []complex64, sampleRate int) []float32 {
	base := wfmMonoBase(iq)
	if len(base) == 0 {
		return nil
	}
	bpHi := dsp.LowpassFIR(60000, sampleRate, 101)
	bpLo := dsp.LowpassFIR(54000, sampleRate, 101)
	hi := dsp.ApplyFIRReal(base, bpHi)
	lo := dsp.ApplyFIRReal(base, bpLo)
	bpf := make([]float32, len(base))
	for i := range base {
		bpf[i] = hi[i] - lo[i]
	}
	phase := 0.0
	inc := 2 * math.Pi * 57000 / float64(sampleRate)
	out := make([]float32, len(base))
	for i := range bpf {
		phase += inc
		out[i] = bpf[i] * float32(math.Cos(phase))
	}
	lp := dsp.LowpassFIR(2400, sampleRate, 101)
	return dsp.ApplyFIRReal(out, lp)
}

func deemphasis(x []float32, sampleRate int, tau float64) []float32 {
	if len(x) == 0 || sampleRate <= 0 {
		return x
	}
	alpha := math.Exp(-1.0 / (float64(sampleRate) * tau))
	out := make([]float32, len(x))
	var y float64
	for i, v := range x {
		y = alpha*y + (1-alpha)*float64(v)
		out[i] = float32(y)
	}
	return out
}

func init() {
	Register(NFM{})
	Register(WFM{})
	Register(WFMStereo{})
}

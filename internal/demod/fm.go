package demod

import (
	"math"

	"sdr-wideband-suite/internal/dsp"
	"sdr-wideband-suite/internal/logging"
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
	maxAbs := 0.0
	maxIdx := 0
	largeSteps := 0
	minMag := math.MaxFloat64
	maxMag := 0.0
	for i := 1; i < len(iq); i++ {
		p := iq[i-1]
		c := iq[i]
		pmag := math.Hypot(float64(real(p)), float64(imag(p)))
		cmag := math.Hypot(float64(real(c)), float64(imag(c)))
		if pmag < minMag {
			minMag = pmag
		}
		if cmag < minMag {
			minMag = cmag
		}
		if pmag > maxMag {
			maxMag = pmag
		}
		if cmag > maxMag {
			maxMag = cmag
		}
		num := float64(real(p))*float64(imag(c)) - float64(imag(p))*float64(real(c))
		den := float64(real(p))*float64(real(c)) + float64(imag(p))*float64(imag(c))
		step := math.Atan2(num, den)
		if a := math.Abs(step); a > maxAbs {
			maxAbs = a
			maxIdx = i - 1
		}
		if math.Abs(step) > 1.5 {
			largeSteps++
		}
		out[i-1] = float32(step)
	}
	if logging.EnabledCategory("discrim") {
		logging.Debug("discrim", "fm_meter", "iq_len", len(iq), "audio_len", len(out), "min_mag", minMag, "max_mag", maxMag, "max_abs_step", maxAbs, "max_idx", maxIdx, "large_steps", largeSteps)
		if largeSteps > 0 {
			logging.Warn("discrim", "fm_large_steps", "iq_len", len(iq), "large_steps", largeSteps, "max_abs_step", maxAbs, "max_idx", maxIdx, "min_mag", minMag, "max_mag", maxMag)
		}
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

type RDSBasebandResult struct {
	Samples    []float32
	SampleRate int
}

// RDSBaseband returns a rough 57k baseband (not decoded).
func RDSBaseband(iq []complex64, sampleRate int) []float32 {
	return RDSBasebandDecimated(iq, sampleRate).Samples
}

// RDSComplexResult holds complex baseband samples for the Costas loop RDS decoder.
type RDSComplexResult struct {
	Samples    []complex64
	SampleRate int
}

// RDSBasebandComplex extracts the RDS subcarrier as complex samples.
// The Costas loop in the RDS decoder needs both I and Q to lock.
func RDSBasebandComplex(iq []complex64, sampleRate int) RDSComplexResult {
	base := wfmMonoBase(iq)
	if len(base) == 0 || sampleRate <= 0 {
		return RDSComplexResult{}
	}
	cplx := make([]complex64, len(base))
	for i, v := range base {
		cplx[i] = complex(v, 0)
	}
	cplx = dsp.FreqShift(cplx, sampleRate, -57000)
	lpTaps := dsp.LowpassFIR(7500, sampleRate, 101)
	cplx = dsp.ApplyFIR(cplx, lpTaps)
	targetRate := 19000
	decim := sampleRate / targetRate
	if decim < 1 {
		decim = 1
	}
	cplx = dsp.Decimate(cplx, decim)
	actualRate := sampleRate / decim
	return RDSComplexResult{Samples: cplx, SampleRate: actualRate}
}

// RDSBasebandDecimated returns float32 baseband for WAV writing / recorder.
func RDSBasebandDecimated(iq []complex64, sampleRate int) RDSBasebandResult {
	res := RDSBasebandComplex(iq, sampleRate)
	out := make([]float32, len(res.Samples))
	for i, c := range res.Samples {
		out[i] = real(c)
	}
	return RDSBasebandResult{Samples: out, SampleRate: res.SampleRate}
}

func init() {
	Register(NFM{})
	Register(WFM{})
	Register(WFMStereo{})
}

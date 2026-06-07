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

// fmDiscrimInto is the allocation-free core of fmDiscrim: it writes the FM
// discriminator output into out (grown if needed) and returns the used slice.
// It omits the diagnostic meter/logging fmDiscrim computes; the produced samples
// are identical.
func fmDiscrimInto(iq []complex64, out []float32) []float32 {
	if len(iq) < 2 {
		return out[:0]
	}
	n := len(iq) - 1
	if cap(out) < n {
		out = make([]float32, n)
	}
	out = out[:n]
	for i := 1; i < len(iq); i++ {
		p := iq[i-1]
		c := iq[i]
		num := float64(real(p))*float64(imag(c)) - float64(imag(p))*float64(real(c))
		den := float64(real(p))*float64(real(c)) + float64(imag(p))*float64(imag(c))
		out[i-1] = float32(math.Atan2(num, den))
	}
	return out
}

func decimateComplexInto(iq []complex64, factor int, out []complex64) []complex64 {
	if factor <= 1 {
		if cap(out) < len(iq) {
			out = make([]complex64, len(iq))
		}
		out = out[:len(iq)]
		copy(out, iq)
		return out
	}
	need := len(iq)/factor + 1
	if cap(out) < need {
		out = make([]complex64, 0, need)
	}
	out = out[:0]
	for i := 0; i < len(iq); i += factor {
		out = append(out, iq[i])
	}
	return out
}

// RDSScratch holds the per-signal reusable buffers (and cached taps) for
// RDSBasebandComplexInto. One per stable tracker ID; the caller must not use it
// concurrently (the RDS decode goroutine is serialized per signal by a busy flag).
type RDSScratch struct {
	disc   []float32
	cplx   []complex64
	filt   []complex64
	decim  []complex64
	lpTaps []float64
	tapsSR int
}

// RDSBasebandComplexInto is RDSBasebandComplex with all per-call allocations
// served from s (reused across decodes), and the lowpass taps cached. The output
// is byte-identical to RDSBasebandComplex (see fm_rds_test.go). The returned
// Samples alias s.decim and stay valid until the next call on the same scratch.
func RDSBasebandComplexInto(iq []complex64, sampleRate int, s *RDSScratch) RDSComplexResult {
	if s == nil {
		return RDSBasebandComplex(iq, sampleRate)
	}
	if len(iq) < 2 || sampleRate <= 0 {
		return RDSComplexResult{}
	}
	s.disc = fmDiscrimInto(iq, s.disc)
	n := len(s.disc)
	if n == 0 {
		return RDSComplexResult{}
	}
	// Complex convert (imag=0) fused with FreqShift(-57000) into s.cplx.
	if cap(s.cplx) < n {
		s.cplx = make([]complex64, n)
	}
	s.cplx = s.cplx[:n]
	phase := 0.0
	inc := -2 * math.Pi * (-57000.0) / float64(sampleRate)
	for i := 0; i < n; i++ {
		phase += inc
		re := math.Cos(phase)
		im := math.Sin(phase)
		b := float64(s.disc[i])
		s.cplx[i] = complex(float32(b*re), float32(b*im))
	}
	if s.lpTaps == nil || s.tapsSR != sampleRate {
		s.lpTaps = dsp.LowpassFIR(7500, sampleRate, 101)
		s.tapsSR = sampleRate
	}
	s.filt = dsp.ApplyFIRInto(s.cplx, s.lpTaps, s.filt)
	decim := sampleRate / 19000
	if decim < 1 {
		decim = 1
	}
	s.decim = decimateComplexInto(s.filt, decim, s.decim)
	return RDSComplexResult{Samples: s.decim, SampleRate: sampleRate / decim}
}

func init() {
	Register(NFM{})
	Register(WFM{})
	Register(WFMStereo{})
}

package dsp

import "math"

type DCBlocker struct {
	r     float64
	prevX complex64
	prevY complex64
}

func NewDCBlocker(r float64) *DCBlocker {
	if r <= 0 || r >= 1 {
		r = 0.995
	}
	return &DCBlocker{r: r}
}

func (d *DCBlocker) Reset() {
	d.prevX = 0
	d.prevY = 0
}

func (d *DCBlocker) Apply(iq []complex64) {
	if d == nil {
		return
	}
	for i := 0; i < len(iq); i++ {
		x := iq[i]
		y := complex(
			float32(float64(real(x)-real(d.prevX))+d.r*float64(real(d.prevY))),
			float32(float64(imag(x)-imag(d.prevX))+d.r*float64(imag(d.prevY))),
		)
		d.prevX = x
		d.prevY = y
		iq[i] = y
	}
}

func IQBalance(iq []complex64) {
	if len(iq) == 0 {
		return
	}
	var sumI, sumQ float64
	for _, v := range iq {
		sumI += float64(real(v))
		sumQ += float64(imag(v))
	}
	meanI := sumI / float64(len(iq))
	meanQ := sumQ / float64(len(iq))

	var varI, varQ, cov float64
	for _, v := range iq {
		i := float64(real(v)) - meanI
		q := float64(imag(v)) - meanQ
		varI += i * i
		varQ += q * q
		cov += i * q
	}
	n := float64(len(iq))
	varI /= n
	varQ /= n
	cov /= n
	if varI <= 0 || varQ <= 0 {
		return
	}

	gain := math.Sqrt(varI / varQ)
	phi := 0.5 * math.Atan2(2*cov, varI-varQ)
	cosP := math.Cos(phi)
	sinP := math.Sin(phi)

	for i := 0; i < len(iq); i++ {
		re := float64(real(iq[i])) - meanI
		im := (float64(imag(iq[i])) - meanQ) * gain
		i2 := re*cosP - im*sinP
		q2 := re*sinP + im*cosP
		iq[i] = complex(float32(i2+meanI), float32(q2+meanQ))
	}
}

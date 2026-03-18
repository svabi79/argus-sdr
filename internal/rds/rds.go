package rds

import (
	"math"
)

// Decoder performs a simple RDS baseband decode (BPSK, 1187.5 bps).
type Decoder struct {
	lastPS string
	lastPI uint16
}

type Result struct {
	PI uint16 `json:"pi"`
	PS string `json:"ps"`
}

// Decode takes baseband samples at ~2400 Hz and attempts to extract PI/PS.
func (d *Decoder) Decode(base []float32, sampleRate int) Result {
	if len(base) == 0 || sampleRate <= 0 {
		return Result{}
	}
	// crude clock: 1187.5 bps
	baud := 1187.5
	spb := float64(sampleRate) / baud
	// carrier recovery simplified: assume baseband already mixed
	bits := make([]int, 0, int(float64(len(base))/spb))
	phase := 0.0
	for i := 0; i < len(base); i++ {
		phase += 1.0
		if phase >= spb {
			phase -= spb
			// slice decision
			v := base[i]
			if v >= 0 {
				bits = append(bits, 1)
			} else {
				bits = append(bits, 0)
			}
		}
	}
	// parse groups (very naive): look for 16-bit blocks and decode group type 0A for PS
	// This is a placeholder: real RDS needs CRC and block sync.
	if len(bits) < 104 {
		return Result{PI: d.lastPI, PS: d.lastPS}
	}
	// best effort: just map first 16 bits to PI and next 8 chars from consecutive bytes
	pi := bitsToU16(bits[0:16])
	ps := decodePS(bits)
	if pi != 0 {
		d.lastPI = pi
	}
	if ps != "" {
		d.lastPS = ps
	}
	return Result{PI: d.lastPI, PS: d.lastPS}
}

func bitsToU16(bits []int) uint16 {
	var v uint16
	for _, b := range bits {
		v = (v << 1) | uint16(b&1)
	}
	return v
}

func decodePS(bits []int) string {
	// naive: take next 64 bits as 8 ASCII chars
	if len(bits) < 16+64 {
		return ""
	}
	start := 16
	out := make([]rune, 0, 8)
	for i := 0; i < 8; i++ {
		var c byte
		for j := 0; j < 8; j++ {
			c = (c << 1) | byte(bits[start+i*8+j]&1)
		}
		if c < 32 || c > 126 {
			c = ' '
		}
		out = append(out, rune(c))
	}
	// trim
	for len(out) > 0 && out[len(out)-1] == ' ' {
		out = out[:len(out)-1]
	}
	return string(out)
}

// BPSKCostas returns a simple carrier-locked version of baseband (placeholder).
func BPSKCostas(in []float32) []float32 {
	out := make([]float32, len(in))
	var phase float64
	for i, v := range in {
		phase += 0.0001 * float64(v) * math.Sin(phase)
		out[i] = float32(float64(v) * math.Cos(phase))
	}
	return out
}

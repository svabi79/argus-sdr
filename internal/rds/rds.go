package rds

import "math"

// Decoder performs a simple RDS baseband decode (BPSK, 1187.5 bps).
type Decoder struct {
	ps     [8]rune
	rt     [64]rune
	lastPI uint16
}

type Result struct {
	PI uint16 `json:"pi"`
	PS string `json:"ps"`
	RT string `json:"rt"`
}

// Decode takes baseband samples at ~2400 Hz and attempts to extract PI/PS/RT.
// NOTE: lightweight decoder with CRC+block sync; not a full RDS implementation.
func (d *Decoder) Decode(base []float32, sampleRate int) Result {
	if len(base) == 0 || sampleRate <= 0 {
		return Result{}
	}
	// crude clock: 1187.5 bps
	baud := 1187.5
	spb := float64(sampleRate) / baud
	bits := make([]int, 0, int(float64(len(base))/spb))
	phase := 0.0
	for i := 0; i < len(base); i++ {
		phase += 1.0
		if phase >= spb {
			phase -= spb
			if base[i] >= 0 {
				bits = append(bits, 1)
			} else {
				bits = append(bits, 0)
			}
		}
	}
	if len(bits) < 26*4 {
		return Result{PI: d.lastPI, PS: d.psString(), RT: d.rtString()}
	}
	// search for block sync
	for i := 0; i+26*4 <= len(bits); i++ {
		bA, okA := decodeBlock(bits[i : i+26])
		bB, okB := decodeBlock(bits[i+26 : i+52])
		bC, okC := decodeBlock(bits[i+52 : i+78])
		bD, okD := decodeBlock(bits[i+78 : i+104])
		if !(okA && okB && okC && okD) {
			continue
		}
		if bA.offset != offA || bB.offset != offB || bC.offset != offC || bD.offset != offD {
			continue
		}
		pi := bA.data
		if pi != 0 {
			d.lastPI = pi
		}
		groupType := (bB.data >> 12) & 0xF
		versionA := ((bB.data >> 11) & 0x1) == 0
		if groupType == 0 && versionA {
			addr := bB.data & 0x3
			chars := []byte{byte(bD.data >> 8), byte(bD.data & 0xFF)}
			idx := int(addr) * 2
			if idx+1 < len(d.ps) {
				d.ps[idx] = sanitizeRune(chars[0])
				d.ps[idx+1] = sanitizeRune(chars[1])
			}
		}
		if groupType == 2 && versionA {
			addr := bB.data & 0xF
			chars := []byte{byte(bC.data >> 8), byte(bC.data & 0xFF), byte(bD.data >> 8), byte(bD.data & 0xFF)}
			idx := int(addr) * 4
			for j := 0; j < 4 && idx+j < len(d.rt); j++ {
				d.rt[idx+j] = sanitizeRune(chars[j])
			}
		}
		break
	}
	return Result{PI: d.lastPI, PS: d.psString(), RT: d.rtString()}
}

type block struct {
	data   uint16
	offset uint16
}

const (
	offA uint16 = 0x0FC
	offB uint16 = 0x198
	offC uint16 = 0x168
	offD uint16 = 0x1B4
)

func decodeBlock(bits []int) (block, bool) {
	if len(bits) != 26 {
		return block{}, false
	}
	var raw uint32
	for _, b := range bits {
		raw = (raw << 1) | uint32(b&1)
	}
	data := uint16(raw >> 10)
	synd := crcSyndrome(raw)
	switch synd {
	case offA, offB, offC, offD:
		return block{data: data, offset: uint16(synd)}, true
	default:
		return block{}, false
	}
}

func crcSyndrome(raw uint32) uint16 {
	// polynomial 0x1B9 (10-bit)
	var reg uint32 = raw
	poly := uint32(0x1B9)
	for i := 25; i >= 10; i-- {
		if (reg>>uint(i))&1 == 1 {
			reg ^= poly << uint(i-10)
		}
	}
	return uint16(reg & 0x3FF)
}

func sanitizeRune(b byte) rune {
	if b < 32 || b > 126 {
		return ' '
	}
	return rune(b)
}

func (d *Decoder) psString() string {
	out := make([]rune, 0, len(d.ps))
	for _, r := range d.ps {
		if r == 0 {
			r = ' '
		}
		out = append(out, r)
	}
	return trimRight(out)
}

func (d *Decoder) rtString() string {
	out := make([]rune, 0, len(d.rt))
	for _, r := range d.rt {
		if r == 0 {
			r = ' '
		}
		out = append(out, r)
	}
	return trimRight(out)
}

func trimRight(in []rune) string {
	end := len(in)
	for end > 0 && in[end-1] == ' ' {
		end--
	}
	return string(in[:end])
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

package rds

import (
	"fmt"
	"math"
)

// Decoder performs RDS baseband decode with Costas loop carrier recovery
// and Mueller & Muller symbol timing synchronization.
type Decoder struct {
	ps     [8]rune
	rt     [64]rune
	lastPI uint16
	// Costas loop state (persistent across calls)
	costasPhase float64
	costasFreq  float64
	// Symbol sync state
	syncMu float64
	// Diagnostic counters
	TotalDecodes  int
	BlockAHits    int
	GroupsFound   int
	LastDiag      string
}

type Result struct {
	PI uint16 `json:"pi"`
	PS string `json:"ps"`
	RT string `json:"rt"`
}

// Decode takes complex baseband samples at ~20kHz and extracts RDS data.
func (d *Decoder) Decode(samples []complex64, sampleRate int) Result {
	if len(samples) < 104 || sampleRate <= 0 {
		return Result{PI: d.lastPI, PS: d.psString(), RT: d.rtString()}
	}
	d.TotalDecodes++

	sps := float64(sampleRate) / 1187.5 // samples per symbol

	// === Mueller & Muller symbol timing recovery ===
	// Reset state each call — accumulated samples have phase gaps between frames
	mu := sps / 2
	symbols := make([]complex64, 0, len(samples)/int(sps)+1)
	var prev, prevDecision complex64
	for mu < float64(len(samples)-1) {
		idx := int(mu)
		frac := mu - float64(idx)
		if idx+1 >= len(samples) {
			break
		}
		samp := complex64(complex(
			float64(real(samples[idx]))*(1-frac)+float64(real(samples[idx+1]))*frac,
			float64(imag(samples[idx]))*(1-frac)+float64(imag(samples[idx+1]))*frac,
		))

		var decision complex64
		if real(samp) > 0 {
			decision = 1
		} else {
			decision = -1
		}

		if len(symbols) >= 2 {
			errR := float64(real(decision)-real(prevDecision))*float64(real(prev)) -
				float64(real(samp)-real(prev))*float64(real(prevDecision))
			mu += sps + 0.01*errR
		} else {
			mu += sps
		}

		prevDecision = decision
		prev = samp
		symbols = append(symbols, samp)
	}
	

	if len(symbols) < 26*4 {
		d.LastDiag = fmt.Sprintf("too few symbols: %d", len(symbols))
		return Result{PI: d.lastPI, PS: d.psString(), RT: d.rtString()}
	}

	// === Costas loop for fine frequency/phase synchronization ===
	// Reset each call — phase gaps between accumulated frames break continuity
	alpha := 0.132
	beta := alpha * alpha / 4.0
	phase := 0.0
	freq := 0.0
	synced := make([]complex64, len(symbols))
	for i, s := range symbols {
		// Multiply by exp(-j*phase) to de-rotate
		cosP := float32(math.Cos(phase))
		sinP := float32(math.Sin(phase))
		synced[i] = complex(
			real(s)*cosP+imag(s)*sinP,
			imag(s)*cosP-real(s)*sinP,
		)
		// BPSK phase error: sign(I) * Q
		var err float64
		if real(synced[i]) > 0 {
			err = float64(imag(synced[i]))
		} else {
			err = -float64(imag(synced[i]))
		}
		freq += beta * err
		phase += freq + alpha*err
		for phase > math.Pi {
			phase -= 2 * math.Pi
		}
		for phase < -math.Pi {
			phase += 2 * math.Pi
		}
	}
	// state not persisted — samples have gaps
	

	// Measure signal quality: average |I| and |Q| after Costas
	var sumI, sumQ float64
	for _, s := range synced {
		ri := float64(real(s))
		rq := float64(imag(s))
		if ri < 0 { ri = -ri }
		if rq < 0 { rq = -rq }
		sumI += ri
		sumQ += rq
	}
	avgI := sumI / float64(len(synced))
	avgQ := sumQ / float64(len(synced))

	// === BPSK demodulation ===
	hardBits := make([]int, len(synced))
	for i, s := range synced {
		if real(s) > 0 {
			hardBits[i] = 1
		} else {
			hardBits[i] = 0
		}
	}

	// === Differential decoding ===
	bits := make([]int, len(hardBits)-1)
	for i := 1; i < len(hardBits); i++ {
		bits[i-1] = hardBits[i] ^ hardBits[i-1]
	}

	// === Block sync + CRC decode (try both polarities) ===
	// Count block A CRC hits for diagnostics
	blockAHits := 0
	for i := 0; i+26 <= len(bits); i++ {
		if _, ok := decodeBlock(bits[i : i+26]); ok {
			blockAHits++
		}
	}
	d.BlockAHits += blockAHits

	found1 := d.tryDecode(bits)
	invBits := make([]int, len(bits))
	for i, b := range bits {
		invBits[i] = 1 - b
	}
	found2 := d.tryDecode(invBits)
	if found1 || found2 {
		d.GroupsFound++
	}

	d.LastDiag = fmt.Sprintf("syms=%d sps=%.1f costasFreq=%.4f avgI=%.4f avgQ=%.4f blockAHits=%d groups=%d",
		len(symbols), sps, freq, avgI, avgQ, blockAHits, d.GroupsFound)

	return Result{PI: d.lastPI, PS: d.psString(), RT: d.rtString()}
}

func (d *Decoder) tryDecode(bits []int) bool {
	found := false
	for i := 0; i+26*4 <= len(bits); i++ {
		bA, okA := decodeBlock(bits[i : i+26])
		if !okA || bA.offset != offA {
			continue
		}
		bB, okB := decodeBlock(bits[i+26 : i+52])
		if !okB || bB.offset != offB {
			continue
		}
		bC, okC := decodeBlock(bits[i+52 : i+78])
		if !okC || (bC.offset != offC && bC.offset != offCp) {
			continue
		}
		bD, okD := decodeBlock(bits[i+78 : i+104])
		if !okD || bD.offset != offD {
			continue
		}
		found = true
		pi := bA.data
		if pi != 0 {
			d.lastPI = pi
		}
		groupType := (bB.data >> 12) & 0xF
		versionA := ((bB.data >> 11) & 0x1) == 0
		if groupType == 0 {
			addr := bB.data & 0x3
			if versionA {
				chars := []byte{byte(bD.data >> 8), byte(bD.data & 0xFF)}
				idx := int(addr) * 2
				if idx+1 < len(d.ps) {
					d.ps[idx] = sanitizeRune(chars[0])
					d.ps[idx+1] = sanitizeRune(chars[1])
				}
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
		i += 103
	}
	return found
}

type block struct {
	data   uint16
	offset uint16
}

const (
	offA  uint16 = 0x0FC
	offB  uint16 = 0x198
	offC  uint16 = 0x168
	offCp uint16 = 0x350
	offD  uint16 = 0x1B4
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
	case offA, offB, offC, offCp, offD:
		return block{data: data, offset: synd}, true
	default:
		return block{}, false
	}
}

func crcSyndrome(raw uint32) uint16 {
	poly := uint32(0x1B9)
	reg := raw
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

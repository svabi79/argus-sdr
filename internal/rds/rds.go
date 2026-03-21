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
	syncMu           float64
	syncPrev         complex64
	syncPrevDecision complex64
	syncHasPrev      bool
	lastSampleRate   int
	// Differential decode state across Decode() calls
	lastHardBit    int
	hasLastHardBit bool
	// Diagnostic counters
	TotalDecodes int
	BlockAHits   int
	GroupsFound  int
	LastDiag     string
}

type Result struct {
	PI uint16 `json:"pi"`
	PS string `json:"ps"`
	RT string `json:"rt"`
}

type scanDiag struct {
	pol         string
	blockHits   int
	offAHits    int
	offBHits    int
	offCHits    int
	offCpHits   int
	offDHits    int
	abSeq       int
	abcSeq      int
	groups      int
	piHint      uint16
	piHintCount int
	fecBlockFix int
	grpFecFix   int
	blockAmbig  int
	seqAmbig    int
}

func (s scanDiag) score() int {
	return s.groups*100000 + s.abcSeq*1000 + s.abSeq*100 + s.blockHits
}

func bestDiag(a, b scanDiag) scanDiag {
	if b.score() > a.score() {
		return b
	}
	if b.score() == a.score() {
		if b.piHintCount > a.piHintCount {
			return b
		}
		if b.fecBlockFix > a.fecBlockFix {
			return b
		}
	}
	return a
}

// Decode takes complex baseband samples at ~20kHz and extracts RDS data.
func (d *Decoder) Decode(samples []complex64, sampleRate int) Result {
	if len(samples) < 104 || sampleRate <= 0 {
		return Result{PI: d.lastPI, PS: d.psString(), RT: d.rtString()}
	}
	d.TotalDecodes++

	if d.lastSampleRate != 0 && d.lastSampleRate != sampleRate {
		d.costasPhase = 0
		d.costasFreq = 0
		d.syncMu = 0
		d.syncPrev = 0
		d.syncPrevDecision = 0
		d.syncHasPrev = false
		d.lastHardBit = 0
		d.hasLastHardBit = false
	}
	d.lastSampleRate = sampleRate

	sps := float64(sampleRate) / 1187.5 // samples per symbol

	// === Mueller & Muller symbol timing recovery (persistent across calls) ===
	mu := d.syncMu
	if mu < 0 || mu >= sps || math.IsNaN(mu) || math.IsInf(mu, 0) {
		mu = sps / 2
	}
	symbols := make([]complex64, 0, len(samples)/int(sps)+1)
	prev := d.syncPrev
	prevDecision := d.syncPrevDecision
	havePrev := d.syncHasPrev
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
		if real(samp) >= 0 {
			decision = 1
		} else {
			decision = -1
		}

		if havePrev {
			errR := float64(real(decision)-real(prevDecision))*float64(real(prev)) -
				float64(real(samp)-real(prev))*float64(real(prevDecision))
			mu += sps + 0.01*errR
		} else {
			mu += sps
			havePrev = true
		}

		prevDecision = decision
		prev = samp
		symbols = append(symbols, samp)
	}
	residualMu := mu - float64(len(samples))
	for residualMu < 0 {
		residualMu += sps
	}
	for residualMu >= sps {
		residualMu -= sps
	}
	d.syncMu = residualMu
	d.syncPrev = prev
	d.syncPrevDecision = prevDecision
	d.syncHasPrev = havePrev

	if len(symbols) < 26*4 {
		d.LastDiag = fmt.Sprintf("too few symbols: %d sps=%.1f mu=%.3f", len(symbols), sps, d.syncMu)
		return Result{PI: d.lastPI, PS: d.psString(), RT: d.rtString()}
	}

	// === Costas loop for fine frequency/phase synchronization (persistent across calls) ===
	alpha := 0.132
	beta := alpha * alpha / 4.0
	phase := d.costasPhase
	freq := d.costasFreq
	synced := make([]complex64, len(symbols))
	for i, s := range symbols {
		// Multiply by exp(-j*phase) to de-rotate.
		cosP := float32(math.Cos(phase))
		sinP := float32(math.Sin(phase))
		synced[i] = complex(
			real(s)*cosP+imag(s)*sinP,
			imag(s)*cosP-real(s)*sinP,
		)
		// BPSK phase error: sign(I) * Q.
		var err float64
		if real(synced[i]) >= 0 {
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
	d.costasPhase = phase
	d.costasFreq = freq

	// Measure signal quality: average |I| and |Q| after Costas.
	var sumI, sumQ float64
	for _, s := range synced {
		ri := math.Abs(float64(real(s)))
		rq := math.Abs(float64(imag(s)))
		sumI += ri
		sumQ += rq
	}
	avgI := sumI / float64(len(synced))
	avgQ := sumQ / float64(len(synced))

	// === BPSK demodulation ===
	hardBits := make([]int, len(synced))
	for i, s := range synced {
		if real(s) >= 0 {
			hardBits[i] = 1
		} else {
			hardBits[i] = 0
		}
	}

	// === Differential decoding ===
	// Preserve the differential transition across Decode() calls. On the very
	// first call we keep the historical behavior (N hard bits -> N-1 diff bits);
	// once carry state exists we prepend the cross-chunk transition bit.
	var bits []int
	if len(hardBits) > 0 {
		if d.hasLastHardBit {
			bits = make([]int, len(hardBits))
			bits[0] = hardBits[0] ^ d.lastHardBit
			for i := 1; i < len(hardBits); i++ {
				bits[i] = hardBits[i] ^ hardBits[i-1]
			}
		} else if len(hardBits) >= 2 {
			bits = make([]int, len(hardBits)-1)
			for i := 1; i < len(hardBits); i++ {
				bits[i-1] = hardBits[i] ^ hardBits[i-1]
			}
		}
		d.lastHardBit = hardBits[len(hardBits)-1]
		d.hasLastHardBit = true
	}
	invBits := make([]int, len(bits))
	for i, b := range bits {
		invBits[i] = 1 - b
	}

	// === Diagnostics before/after 1-bit FEC ===
	rawBest := bestDiag(analyzeStream(bits, false, "dir"), analyzeStream(invBits, false, "inv"))
	fecBest := bestDiag(analyzeStream(bits, true, "dir"), analyzeStream(invBits, true, "inv"))
	d.BlockAHits += rawBest.offAHits

	// === Block sync + CRC decode with conservative 1-bit FEC ===
	groupsFound := d.tryDecode(bits, true)
	usedPol := "dir"
	if groupsFound == 0 {
		groupsFound = d.tryDecode(invBits, true)
		if groupsFound > 0 {
			usedPol = "inv"
		} else {
			usedPol = "none"
		}
	}
	if groupsFound > 0 {
		d.GroupsFound++
	}

	d.LastDiag = fmt.Sprintf(
		"syms=%d sps=%.1f mu=%.3f costasFreq=%.4f avgI=%.4f avgQ=%.4f diffCarry=%t raw[%s blk=%d A/B/C/Cp/D=%d/%d/%d/%d/%d AB=%d ABC=%d grp=%d pi=%04Xx%d] fec[%s blk=%d A/B/C/Cp/D=%d/%d/%d/%d/%d AB=%d ABC=%d grp=%d pi=%04Xx%d fixBlk=%d fixGrp=%d ambBlk=%d ambSeq=%d] use=%s found=%d okCalls=%d",
		len(symbols), sps, d.syncMu, d.costasFreq, avgI, avgQ, d.hasLastHardBit,
		rawBest.pol, rawBest.blockHits, rawBest.offAHits, rawBest.offBHits, rawBest.offCHits, rawBest.offCpHits, rawBest.offDHits, rawBest.abSeq, rawBest.abcSeq, rawBest.groups, rawBest.piHint, rawBest.piHintCount,
		fecBest.pol, fecBest.blockHits, fecBest.offAHits, fecBest.offBHits, fecBest.offCHits, fecBest.offCpHits, fecBest.offDHits, fecBest.abSeq, fecBest.abcSeq, fecBest.groups, fecBest.piHint, fecBest.piHintCount, fecBest.fecBlockFix, fecBest.grpFecFix, fecBest.blockAmbig, fecBest.seqAmbig,
		usedPol, groupsFound, d.GroupsFound,
	)

	return Result{PI: d.lastPI, PS: d.psString(), RT: d.rtString()}
}

func analyzeStream(bits []int, useFEC bool, pol string) scanDiag {
	diag := scanDiag{pol: pol}
	if len(bits) < 26 {
		return diag
	}

	piCounts := make(map[uint16]int)
	for i := 0; i+26 <= len(bits); i++ {
		blk, ok, corrected, ambiguous := decodeBlockAny(bits[i:i+26], useFEC)
		if ambiguous {
			diag.blockAmbig++
		}
		if !ok {
			continue
		}
		diag.blockHits++
		if corrected {
			diag.fecBlockFix++
		}
		switch blk.offset {
		case offA:
			diag.offAHits++
			if blk.data != 0 {
				piCounts[blk.data]++
			}
		case offB:
			diag.offBHits++
		case offC:
			diag.offCHits++
		case offCp:
			diag.offCpHits++
		case offD:
			diag.offDHits++
		}
	}
	for pi, n := range piCounts {
		if n > diag.piHintCount {
			diag.piHint = pi
			diag.piHintCount = n
		}
	}

	const groupBits = 26 * 4
	for i := 0; i+groupBits <= len(bits); i++ {
		fixes := 0

		_, okA, fixedA, ambA := decodeBlockExpected(bits[i:i+26], []uint16{offA}, useFEC)
		if ambA {
			diag.seqAmbig++
		}
		if !okA {
			continue
		}
		if fixedA {
			fixes++
		}

		_, okB, fixedB, ambB := decodeBlockExpected(bits[i+26:i+52], []uint16{offB}, useFEC)
		if ambB {
			diag.seqAmbig++
		}
		if !okB {
			continue
		}
		diag.abSeq++
		if fixedB {
			fixes++
		}

		_, okC, fixedC, ambC := decodeBlockExpected(bits[i+52:i+78], []uint16{offC, offCp}, useFEC)
		if ambC {
			diag.seqAmbig++
		}
		if !okC {
			continue
		}
		diag.abcSeq++
		if fixedC {
			fixes++
		}

		_, okD, fixedD, ambD := decodeBlockExpected(bits[i+78:i+104], []uint16{offD}, useFEC)
		if ambD {
			diag.seqAmbig++
		}
		if !okD {
			continue
		}
		if fixedD {
			fixes++
		}

		diag.groups++
		diag.grpFecFix += fixes
	}

	return diag
}

func (d *Decoder) tryDecode(bits []int, useFEC bool) int {
	const (
		groupBits      = 26 * 4
		flywheelJitter = 3
	)
	groups := 0
	for i := 0; i+groupBits <= len(bits); {
		grp, _, ok := decodeGroupAt(bits, i, useFEC)
		if !ok {
			i++
			continue
		}

		groups++
		d.applyGroup(grp)

		// Flywheel: once a valid group was found, prefer the next expected 104-bit
		// boundary and search only in a tiny jitter window around it before falling
		// back to full bitwise scanning.
		nextExpected := i + groupBits
		locked := true
		for locked && nextExpected+groupBits <= len(bits) {
			if nextGrp, _, ok := decodeGroupAt(bits, nextExpected, useFEC); ok {
				d.applyGroup(nextGrp)
				groups++
				nextExpected += groupBits
				continue
			}

			matched := false
			for delta := 1; delta <= flywheelJitter && !matched; delta++ {
				left := nextExpected - delta
				if left >= 0 {
					if nextGrp, _, ok := decodeGroupAt(bits, left, useFEC); ok {
						d.applyGroup(nextGrp)
						groups++
						nextExpected = left + groupBits
						matched = true
						break
					}
				}
				right := nextExpected + delta
				if right+groupBits <= len(bits) {
					if nextGrp, _, ok := decodeGroupAt(bits, right, useFEC); ok {
						d.applyGroup(nextGrp)
						groups++
						nextExpected = right + groupBits
						matched = true
						break
					}
				}
			}

			if !matched {
				locked = false
			}
		}

		resume := nextExpected - flywheelJitter
		if resume <= i {
			resume = i + 1
		}
		i = resume
	}
	return groups
}

func decodeGroupAt(bits []int, start int, useFEC bool) ([4]block, int, bool) {
	const groupBits = 26 * 4
	var grp [4]block
	if start < 0 || start+groupBits > len(bits) {
		return grp, 0, false
	}
	fixes := 0

	bA, okA, fixedA, _ := decodeBlockExpected(bits[start:start+26], []uint16{offA}, useFEC)
	if !okA {
		return grp, 0, false
	}
	if fixedA {
		fixes++
	}
	bB, okB, fixedB, _ := decodeBlockExpected(bits[start+26:start+52], []uint16{offB}, useFEC)
	if !okB {
		return grp, 0, false
	}
	if fixedB {
		fixes++
	}
	bC, okC, fixedC, _ := decodeBlockExpected(bits[start+52:start+78], []uint16{offC, offCp}, useFEC)
	if !okC {
		return grp, 0, false
	}
	if fixedC {
		fixes++
	}
	bD, okD, fixedD, _ := decodeBlockExpected(bits[start+78:start+104], []uint16{offD}, useFEC)
	if !okD {
		return grp, 0, false
	}
	if fixedD {
		fixes++
	}
	grp[0] = bA
	grp[1] = bB
	grp[2] = bC
	grp[3] = bD
	return grp, fixes, true
}

func (d *Decoder) applyGroup(grp [4]block) {
	bA, bB, bC, bD := grp[0], grp[1], grp[2], grp[3]
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

var allOffsets = []uint16{offA, offB, offC, offCp, offD}

func decodeBlock(bits []int) (block, bool) {
	if len(bits) != 26 {
		return block{}, false
	}
	return decodeRawBlock(bitsToRaw(bits))
}

func bitsToRaw(bits []int) uint32 {
	var raw uint32
	for _, b := range bits {
		raw = (raw << 1) | uint32(b&1)
	}
	return raw
}

func decodeRawBlock(raw uint32) (block, bool) {
	data := uint16(raw >> 10)
	synd := crcSyndrome(raw)
	switch synd {
	case offA, offB, offC, offCp, offD:
		return block{data: data, offset: synd}, true
	default:
		return block{}, false
	}
}

func decodeBlockAny(bits []int, useFEC bool) (block, bool, bool, bool) {
	return decodeBlockExpected(bits, allOffsets, useFEC)
}

func decodeBlockExpected(bits []int, allowed []uint16, useFEC bool) (block, bool, bool, bool) {
	if len(bits) != 26 {
		return block{}, false, false, false
	}
	if blk, ok := decodeBlock(bits); ok && offsetAllowed(blk.offset, allowed) {
		return blk, true, false, false
	}
	if !useFEC {
		return block{}, false, false, false
	}

	raw := bitsToRaw(bits)
	var candidate block
	candidateCount := 0
	for i := 0; i < 26; i++ {
		flipped := raw ^ (uint32(1) << uint(25-i))
		blk, ok := decodeRawBlock(flipped)
		if !ok || !offsetAllowed(blk.offset, allowed) {
			continue
		}
		candidate = blk
		candidateCount++
		if candidateCount > 1 {
			return block{}, false, false, true
		}
	}
	if candidateCount == 1 {
		return candidate, true, true, false
	}
	return block{}, false, false, false
}

func offsetAllowed(offset uint16, allowed []uint16) bool {
	for _, want := range allowed {
		if offset == want {
			return true
		}
	}
	return false
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

//go:build bench

// Diagnostic for the live FM-BC "random RDS disco + audio noise" report
// (operator, 2026-06-08). Reads a live baseband capture and checks two things the
// /api/signals snapshot cannot show:
//   - TestFMLiveLevel: raw IQ RMS over time. A fixed-gain front end is flat; abrupt
//     steps = SDRplay AGC gain-stepping, which clicks the FM demod and drops every
//     PLL/RDS lock at once (random + all-stations = the reported symptom).
//   - TestFMLivePilotRDS: extract RADIO 7 (102.5, +0.5 MHz off the 102.0 center),
//     FM-demod, and track the 19 kHz stereo pilot + 57 kHz RDS subcarrier power over
//     time — does the MPX itself flicker in the raw recording (RF/hardware), or is
//     the recording clean (so the fault is in sdrd's extraction/decode)?
//
//	go test -tags bench -run 'TestFMLive' ./internal/synth/ -v
package synth_test

import (
	"math"
	"math/cmplx"
	"testing"

	"sdr-wideband-suite/internal/dsp"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/iqfile"
)

const fmLivePath = "../../data/snapshots/fm_live.cf32"

func TestFMLiveLevel(t *testing.T) {
	iq, meta, err := iqfile.Read(fmLivePath)
	if err != nil {
		t.Skipf("capture: %v", err)
	}
	fs := meta.SampleRate
	win := fs / 100 // 10 ms windows
	nwin := len(iq) / win
	levels := make([]float64, nwin)
	var gmin, gmax = math.Inf(1), math.Inf(-1)
	for w := 0; w < nwin; w++ {
		var sum float64
		base := w * win
		for i := 0; i < win; i++ {
			v := iq[base+i]
			sum += float64(real(v))*float64(real(v)) + float64(imag(v))*float64(imag(v))
		}
		db := 10 * math.Log10(sum/float64(win)+1e-20)
		levels[w] = db
		if db < gmin {
			gmin = db
		}
		if db > gmax {
			gmax = db
		}
	}
	// Count abrupt steps (|delta| > 2 dB between adjacent 10 ms windows).
	steps := 0
	var maxStep float64
	for w := 1; w < nwin; w++ {
		d := math.Abs(levels[w] - levels[w-1])
		if d > 2.0 {
			steps++
		}
		if d > maxStep {
			maxStep = d
		}
	}
	t.Logf("=== raw IQ level: %d x 10ms windows over %.1fs ===", nwin, float64(len(iq))/float64(fs))
	t.Logf("  level range = %.1f .. %.1f dB (span %.1f dB)   abrupt steps>2dB = %d   maxStep = %.1f dB",
		gmin, gmax, gmax-gmin, steps, maxStep)
	// Coarse timeline: mean level per 1 s, so a slow AGC ramp or sudden step shows.
	perSec := nwin / int(float64(len(iq))/float64(fs))
	if perSec < 1 {
		perSec = 1
	}
	line := ""
	for s := 0; s*perSec < nwin; s++ {
		var sum float64
		cnt := 0
		for w := s * perSec; w < (s+1)*perSec && w < nwin; w++ {
			sum += levels[w]
			cnt++
		}
		line += "  " + formatDb(sum/float64(cnt))
	}
	t.Logf("  per-second mean dB:%s", line)
}

func formatDb(v float64) string {
	return (func(f float64) string {
		s := ""
		if f >= 0 {
			s = "+"
		}
		return s + trim1(f)
	})(v)
}

func trim1(f float64) string {
	return floatStr(math.Round(f*10) / 10)
}

func floatStr(f float64) string {
	// minimal %.1f
	neg := f < 0
	if neg {
		f = -f
	}
	whole := int(f)
	frac := int(math.Round((f-float64(whole))*10)) % 10
	s := ""
	if neg {
		s = "-"
	}
	return s + itoa(whole) + "." + itoa(frac)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// TestFMLivePilotRDS extracts RADIO 7 and tracks the 19 kHz pilot and 57 kHz RDS
// subcarrier power across the capture, to see if the MPX flickers in the raw IQ.
func TestFMLivePilotRDS(t *testing.T) {
	iq, meta, err := iqfile.Read(fmLivePath)
	if err != nil {
		t.Skipf("capture: %v", err)
	}
	fs := meta.SampleRate
	const decim = 16
	fsd := fs / decim // 256 kHz: covers pilot 19k + RDS 57k
	offset := 0.5e6   // RADIO 7 at 102.5, capture center 102.0

	// Mix RADIO 7 to baseband, low-pass to 120 kHz, decimate to 256 kHz.
	taps := dsp.LowpassFIR(120000, fs, 121)
	shifted := make([]complex64, len(iq))
	w0 := -2 * math.Pi * offset / float64(fs)
	for i, v := range iq {
		r := cmplx.Exp(complex(0, w0*float64(i)))
		shifted[i] = complex64(complex128(v) * r)
	}
	base := dsp.ApplyFIR(shifted, taps)
	dec := make([]complex64, 0, len(base)/decim)
	for i := 0; i < len(base); i += decim {
		dec = append(dec, base[i])
	}
	// FM discriminator -> real MPX baseband.
	mpx := make([]complex64, len(dec)-1)
	for i := 1; i < len(dec); i++ {
		d := float64(cmplx.Phase(complex128(dec[i]) * cmplx.Conj(complex128(dec[i-1]))))
		mpx[i-1] = complex(float32(d), 0)
	}
	// Per 100 ms window, FFT and read pilot (19k) + RDS (57k) power over the floor.
	winN := fsd / 10 // 100 ms
	hann := fftutil.Hann(winN)
	binHz := float64(fsd) / float64(winN)
	pilotBin := int(19000/binHz) + winN/2
	rdsBin := int(57000/binHz) + winN/2
	t.Logf("=== RADIO 7 MPX over %.1fs (100ms windows, fsd=%d, bin=%.0f Hz) ===", float64(len(mpx))/float64(fsd), fsd, binHz)
	t.Logf("  win  pilot19k(dB>flr)  rds57k(dB>flr)")
	nwin := len(mpx) / winN
	var pilotDrop, rdsDrop int
	plt, rlt := "", ""
	for w := 0; w < nwin; w++ {
		seg := mpx[w*winN : (w+1)*winN]
		spec := fftutil.Spectrum(seg, hann)
		floor := medianf(spec)
		pilot := spec[pilotBin] - floor
		rds := spec[rdsBin] - floor
		if pilot < 10 {
			pilotDrop++
		}
		if rds < 6 {
			rdsDrop++
		}
		plt += pilotSym(pilot)
		rlt += pilotSym(rds)
		if w%5 == 0 {
			t.Logf("  %3d  %14.1f  %12.1f", w, pilot, rds)
		}
	}
	t.Logf("  pilot present(>10dB): %d/%d   RDS present(>6dB): %d/%d", nwin-pilotDrop, nwin, nwin-rdsDrop, nwin)
	t.Logf("  pilot timeline (#=strong .=weak): %s", plt)
	t.Logf("  rds   timeline (#=strong .=weak): %s", rlt)
}

func pilotSym(db float64) string {
	switch {
	case db > 20:
		return "#"
	case db > 10:
		return "+"
	case db > 4:
		return "."
	default:
		return " "
	}
}

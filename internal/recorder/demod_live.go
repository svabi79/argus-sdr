package recorder

import (
	"bytes"
	"errors"
	"time"

	"sdr-visual-suite/internal/demod"
	"sdr-visual-suite/internal/dsp"
)

// DemodLive demodulates a recent window and returns WAV bytes.
func (m *Manager) DemodLive(centerHz float64, bw float64, mode string, seconds int) ([]byte, int, error) {
	if m == nil || m.ring == nil {
		return nil, 0, errors.New("recorder not ready")
	}
	if seconds <= 0 {
		seconds = 2
	}
	end := time.Now()
	start := end.Add(-time.Duration(seconds) * time.Second)
	segment := m.ring.Slice(start, end)
	if len(segment) == 0 {
		return nil, 0, errors.New("no iq in ring")
	}
	name := mode
	if name == "" {
		name = "NFM"
	}
	switch name {
	case "AM", "NFM", "WFM", "WFM_STEREO", "USB", "LSB", "CW":
	default:
		name = "NFM"
	}
	d := demod.Get(name)
	if d == nil {
		return nil, 0, errors.New("demodulator not found")
	}
	offset := centerHz - m.centerHz
	shifted := dsp.FreqShift(segment, m.sampleRate, offset)
	if bw <= 0 {
		bw = 12000
	}
	cutoff := bw / 2
	if cutoff < 200 {
		cutoff = 200
	}
	taps := dsp.LowpassFIR(cutoff, m.sampleRate, 101)
	filtered := dsp.ApplyFIR(shifted, taps)
	decim := m.sampleRate / d.OutputSampleRate()
	if decim < 1 {
		decim = 1
	}
	dec := dsp.Decimate(filtered, decim)
	audio := d.Demod(dec, m.sampleRate/decim)
	buf := &bytes.Buffer{}
	if err := writeWAVTo(buf, audio, d.OutputSampleRate(), d.Channels()); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), d.OutputSampleRate(), nil
}

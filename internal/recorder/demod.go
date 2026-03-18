package recorder

import (
	"errors"
	"path/filepath"

	"sdr-visual-suite/internal/classifier"
	"sdr-visual-suite/internal/demod"
	"sdr-visual-suite/internal/detector"
	"sdr-visual-suite/internal/dsp"
)

func (m *Manager) demodAndWrite(dir string, ev detector.Event, iq []complex64, files map[string]any) error {
	if ev.Class == nil {
		return nil
	}
	name := mapClassToDemod(ev.Class.ModType)
	if name == "" {
		return nil
	}
	d := demod.Get(name)
	if d == nil {
		return errors.New("demodulator not found")
	}
	// band-extract around signal
	bw := ev.Bandwidth
	offset := ev.CenterHz - m.centerHz
	shifted := dsp.FreqShift(iq, m.sampleRate, offset)
	cutoff := bw / 2
	if cutoff < 200 {
		cutoff = 200
	}
	taps := dsp.LowpassFIR(cutoff, m.sampleRate, 101)
	filtered := dsp.ApplyFIR(shifted, taps)
	decim := m.sampleRate / (d.OutputSampleRate() * 4)
	if decim < 1 {
		decim = 1
	}
	dec := dsp.Decimate(filtered, decim)
	audio := d.Demod(dec, m.sampleRate/decim)
	wav := filepath.Join(dir, "audio.wav")
	if err := writeWAV(wav, audio, d.OutputSampleRate(), d.Channels()); err != nil {
		return err
	}
	files["audio"] = "audio.wav"
	files["audio_sample_rate"] = d.OutputSampleRate()
	files["audio_channels"] = d.Channels()
	files["audio_demod"] = name
	if name == "WFM_STEREO" {
		if rds := demod.RDSBaseband(iq, m.sampleRate); len(rds) > 0 {
			rdsPath := filepath.Join(dir, "rds.wav")
			_ = writeWAV(rdsPath, rds, 2400, 1)
			files["rds_baseband"] = "rds.wav"
			files["rds_sample_rate"] = 2400
			// naive decode
			dec := rdsdecoder{}
			res := dec.Decode(rds, 2400)
			if res.PI != 0 {
				files["rds_pi"] = res.PI
			}
			if res.PS != "" {
				files["rds_ps"] = res.PS
			}
		}
	}
	return nil
}

func mapClassToDemod(c classifier.SignalClass) string {
	switch c {
	case classifier.ClassAM:
		return "AM"
	case classifier.ClassNFM:
		return "NFM"
	case classifier.ClassWFM:
		return "WFM"
	case classifier.ClassSSBUSB:
		return "USB"
	case classifier.ClassSSBLSB:
		return "LSB"
	case classifier.ClassCW:
		return "CW"
	default:
		return ""
	}
}

package recorder

import (
	"errors"
	"log"
	"math"
	"path/filepath"

	"sdr-visual-suite/internal/classifier"
	"sdr-visual-suite/internal/demod"
	"sdr-visual-suite/internal/demod/gpudemod"
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
	bw := ev.Bandwidth
	offset := ev.CenterHz - m.centerHz
	var audio []float32
	var inputRate int
	if m.gpuDemod != nil {
		var gpuMode gpudemod.DemodType
		var useGPU bool
		switch name {
		case "NFM":
			gpuMode, useGPU = gpudemod.DemodNFM, true
		case "WFM":
			gpuMode, useGPU = gpudemod.DemodWFM, true
		case "AM":
			gpuMode, useGPU = gpudemod.DemodAM, true
		case "USB":
			gpuMode, useGPU = gpudemod.DemodUSB, true
		case "LSB":
			gpuMode, useGPU = gpudemod.DemodLSB, true
		case "CW":
			gpuMode, useGPU = gpudemod.DemodCW, true
		}
		if useGPU {
			if gpuAudio, gpuRate, err := m.gpuDemod.DemodFused(iq, offset, bw, gpuMode); err == nil {
				audio = gpuAudio
				inputRate = gpuRate
				if m.gpuDemod.LastDemodUsedGPU() {
					log.Printf("gpudemod: fused GPU demod used for event %d (%s)", ev.ID, name)
				}
			} else {
				log.Printf("gpudemod: fused GPU demod failed for event %d (%s): %v", ev.ID, name, err)
				if gpuAudio, gpuRate, err := m.gpuDemod.Demod(iq, offset, bw, gpuMode); err == nil {
					audio = gpuAudio
					inputRate = gpuRate
					if m.gpuDemod.LastDemodUsedGPU() {
						log.Printf("gpudemod: legacy GPU demod used for event %d (%s)", ev.ID, name)
					}
				} else {
					log.Printf("gpudemod: legacy GPU demod failed for event %d (%s): %v", ev.ID, name, err)
				}
			}
		}
	}
	if audio == nil {
		log.Printf("gpudemod: CPU demod fallback used for event %d (%s)", ev.ID, name)
		shifted := dsp.FreqShift(iq, m.sampleRate, offset)
		cutoff := bw / 2
		if cutoff < 200 {
			cutoff = 200
		}
		taps := dsp.LowpassFIR(cutoff, m.sampleRate, 101)
		filtered := dsp.ApplyFIR(shifted, taps)
		decim := int(math.Round(float64(m.sampleRate) / float64(d.OutputSampleRate())))
		if decim < 1 {
			decim = 1
		}
		dec := dsp.Decimate(filtered, decim)
		inputRate = m.sampleRate / decim
		audio = d.Demod(dec, inputRate)
	}
	wav := filepath.Join(dir, "audio.wav")
	if err := writeWAV(wav, audio, inputRate, d.Channels()); err != nil {
		return err
	}
	files["audio"] = "audio.wav"
	files["audio_sample_rate"] = inputRate
	files["audio_channels"] = d.Channels()
	files["audio_demod"] = name
	if name == "WFM_STEREO" {
		if rds := demod.RDSBaseband(iq, m.sampleRate); len(rds) > 0 {
			rdsPath := filepath.Join(dir, "rds.wav")
			_ = writeWAV(rdsPath, rds, 2400, 1)
			files["rds_baseband"] = "rds.wav"
			files["rds_sample_rate"] = 2400
			dec := rdsdecoder{}
			res := dec.Decode(rds, 2400)
			if res.PI != 0 {
				files["rds_pi"] = res.PI
			}
			if res.PS != "" {
				files["rds_ps"] = res.PS
			}
			if res.RT != "" {
				files["rds_rt"] = res.RT
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
	case classifier.ClassFT8, classifier.ClassWSPR, classifier.ClassFSK, classifier.ClassPSK:
		return "USB"
	default:
		return ""
	}
}

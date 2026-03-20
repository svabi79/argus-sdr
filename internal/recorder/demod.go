package recorder

import (
	"errors"
	"log"
	"path/filepath"

	"sdr-visual-suite/internal/classifier"
	"sdr-visual-suite/internal/demod"
	"sdr-visual-suite/internal/detector"
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
	gpu := m.gpuEngine()
	if gpu != nil {
		gpuMode, useGPU := gpuModeFor(name)
		if useGPU {
			if gpuAudio, gpuRate, ok := tryGPUAudio(gpu, name, iq, offset, bw, gpuMode); ok {
				audio = gpuAudio
				inputRate = gpuRate
			}
		}
	}
	var stereoHybrid *wfmHybridResult
	if audio == nil {
		if name == "WFM_STEREO" {
			log.Printf("gpudemod: WFM_STEREO using hybrid stereo/RDS post-process for event %d", ev.ID)
			res := demodWFMStereoHybrid(m.gpuEngine(), iq, m.sampleRate, offset, bw)
			stereoHybrid = &res
			audio = res.Audio
			inputRate = res.AudioRate
		} else {
			log.Printf("gpudemod: CPU demod fallback used for event %d (%s)", ev.ID, name)
			audio, inputRate = demodAudioCPU(d, iq, m.sampleRate, offset, bw)
		}
	}
	wav := filepath.Join(dir, "audio.wav")
	if err := writeWAV(wav, audio, inputRate, d.Channels()); err != nil {
		return err
	}
	files["audio"] = "audio.wav"
	files["audio_sample_rate"] = inputRate
	files["audio_channels"] = d.Channels()
	files["audio_demod"] = name
	if name == "WFM_STEREO" && stereoHybrid != nil {
		if len(stereoHybrid.RDS) > 0 {
			rdsPath := filepath.Join(dir, "rds.wav")
			_ = writeWAV(rdsPath, stereoHybrid.RDS, stereoHybrid.RDSRate, 1)
			files["rds_baseband"] = "rds.wav"
			files["rds_sample_rate"] = stereoHybrid.RDSRate
			dec := rdsdecoder{}
			res := dec.DecodeFloat32(stereoHybrid.RDS, stereoHybrid.RDSRate)
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
	case classifier.ClassWFMStereo:
		return "WFM_STEREO"
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

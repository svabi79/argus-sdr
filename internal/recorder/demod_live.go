package recorder

import (
	"bytes"
	"errors"
	"log"
	"math"
	"time"

	"sdr-visual-suite/internal/demod"
	"sdr-visual-suite/internal/demod/gpudemod"
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
	if bw <= 0 {
		bw = 12000
	}

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
			if gpuAudio, gpuRate, err := m.gpuDemod.DemodFused(segment, offset, bw, gpuMode); err == nil {
				audio = gpuAudio
				inputRate = gpuRate
				log.Printf("gpudemod: fused GPU live demod used (%s)", name)
			} else {
				log.Printf("gpudemod: fused GPU live demod failed (%s): %v", name, err)
				if gpuAudio, gpuRate, err := m.gpuDemod.Demod(segment, offset, bw, gpuMode); err == nil {
					audio = gpuAudio
					inputRate = gpuRate
					log.Printf("gpudemod: legacy GPU live demod used (%s)", name)
				} else {
					log.Printf("gpudemod: legacy GPU live demod failed (%s): %v", name, err)
				}
			}
		}
	}
	if audio == nil {
		log.Printf("gpudemod: CPU live demod fallback used (%s)", name)
		shifted := dsp.FreqShift(segment, m.sampleRate, offset)
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
	buf := &bytes.Buffer{}
	if err := writeWAVTo(buf, audio, inputRate, d.Channels()); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), inputRate, nil
}

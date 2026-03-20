package recorder

import (
	"bytes"
	"errors"
	"log"
	"math"
	"time"

	"sdr-visual-suite/internal/demod"
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
	actualDuration := float64(len(segment)) / float64(m.sampleRate)
	if actualDuration < float64(seconds)*0.8 {
		log.Printf("DEMOD WARNING: requested %ds but ring only has %.2fs of IQ data (ring may be underfilled due to sample drops)", seconds, actualDuration)
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
	gpu := m.gpuEngine()
	if gpu != nil {
		gpuMode, useGPU := gpuModeFor(name)
		if useGPU {
			if gpuAudio, gpuRate, ok := tryGPUAudio(gpu, name, segment, offset, bw, gpuMode); ok {
				audio = gpuAudio
				inputRate = gpuRate
			}
		}
	}
	if audio == nil {
		if name == "WFM_STEREO" {
			log.Printf("gpudemod: WFM_STEREO live path using CPU stereo/RDS post-process")
		} else {
			log.Printf("gpudemod: CPU live demod fallback used (%s)", name)
		}
		audio, inputRate = demodAudioCPU(d, segment, m.sampleRate, offset, bw)
	}

	log.Printf("DEMOD DIAG: mode=%s iqSamples=%d sampleRate=%d audioSamples=%d inputRate=%d bw=%.0f offset=%.0f",
		name, len(segment), m.sampleRate, len(audio), inputRate, bw, offset)

	// Resample to 48 kHz for browser-compatible playback.
	const browserRate = 48000
	channels := d.Channels()
	if inputRate > browserRate && len(audio) > 0 {
		decim := int(math.Round(float64(inputRate) / float64(browserRate)))
		if decim < 1 {
			decim = 1
		}
		if channels > 1 {
			nFrames := len(audio) / channels
			outFrames := nFrames / decim
			if outFrames < 1 {
				outFrames = 1
			}
			resampled := make([]float32, outFrames*channels)
			for i := 0; i < outFrames; i++ {
				srcIdx := i * decim * channels
				for ch := 0; ch < channels; ch++ {
					if srcIdx+ch < len(audio) {
						resampled[i*channels+ch] = audio[srcIdx+ch]
					}
				}
			}
			audio = resampled
		} else {
			resampled := make([]float32, 0, len(audio)/decim+1)
			for i := 0; i < len(audio); i += decim {
				resampled = append(resampled, audio[i])
			}
			audio = resampled
		}
		inputRate = inputRate / decim
	}

	log.Printf("DEMOD DIAG: after resample audioSamples=%d finalRate=%d duration=%.2fs",
		len(audio), inputRate, float64(len(audio))/float64(inputRate)/float64(channels))

	// Use actual sample rate for WAV — don't lie about rate
	buf := &bytes.Buffer{}
	if err := writeWAVTo(buf, audio, inputRate, channels); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), inputRate, nil
}

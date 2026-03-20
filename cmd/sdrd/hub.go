package main

import (
	"encoding/binary"
	"encoding/json"
	"log"
	"math"
	"time"

	"sdr-visual-suite/internal/detector"
)

func (s *signalSnapshot) set(sig []detector.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signals = append([]detector.Signal(nil), sig...)
}

func (s *signalSnapshot) get() []detector.Signal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]detector.Signal(nil), s.signals...)
}

func (g *gpuStatus) set(active bool, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Active = active
	if err != nil {
		g.Error = err.Error()
	} else {
		g.Error = ""
	}
}

func (g *gpuStatus) snapshot() gpuStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return gpuStatus{Available: g.Available, Active: g.Active, Error: g.Error}
}

func newHub() *hub {
	return &hub{clients: map[*client]struct{}{}, lastLogTs: time.Now()}
}

func (h *hub) add(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	log.Printf("ws connected (%d clients)", len(h.clients))
}

func (h *hub) remove(c *client) {
	c.closeOnce.Do(func() { close(c.done) })
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	log.Printf("ws disconnected (%d clients)", len(h.clients))
}

func (h *hub) broadcast(frame SpectrumFrame) {
	// Pre-encode JSON for legacy clients (only if needed)
	var jsonBytes []byte
	// Pre-encode binary for binary clients at various decimation levels
	// We cache per unique maxBins value to avoid re-encoding
	type binCacheEntry struct {
		bins int
		data []byte
	}
	var binCache []binCacheEntry

	h.mu.Lock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		// Frame rate limiting
		if c.targetFps > 0 && c.frameSkip > 1 {
			c.frameN++
			if c.frameN%c.frameSkip != 0 {
				continue
			}
		}

		if c.binary {
			// Find or create cached binary encoding for this bin count
			bins := c.maxBins
			if bins <= 0 || bins >= len(frame.Spectrum) {
				bins = len(frame.Spectrum)
			}
			var encoded []byte
			for _, entry := range binCache {
				if entry.bins == bins {
					encoded = entry.data
					break
				}
			}
			if encoded == nil {
				encoded = encodeBinaryFrame(frame, bins)
				binCache = append(binCache, binCacheEntry{bins: bins, data: encoded})
			}
			select {
			case c.send <- encoded:
			default:
				h.remove(c)
			}
		} else {
			// JSON path (legacy)
			if jsonBytes == nil {
				var err error
				jsonBytes, err = json.Marshal(frame)
				if err != nil {
					log.Printf("marshal frame: %v", err)
					return
				}
			}
			select {
			case c.send <- jsonBytes:
			default:
				h.remove(c)
			}
		}
	}

	h.frameCnt++
	if time.Since(h.lastLogTs) > 2*time.Second {
		h.lastLogTs = time.Now()
		log.Printf("broadcast frames=%d clients=%d", h.frameCnt, len(clients))
	}
}

// ---------------------------------------------------------------------------
// Binary spectrum protocol v4
// ---------------------------------------------------------------------------
//
// Hybrid approach: spectrum data as compact binary, signals + debug as JSON.
//
// Layout (32-byte header):
//   [0:1]   magic: 0x53 0x50 ("SP")
//   [2:3]   version: uint16 LE = 4
//   [4:11]  timestamp: int64 LE (Unix millis)
//   [12:19] center_hz: float64 LE
//   [20:23] bin_count: uint32 LE  (supports FFT up to 4 billion)
//   [24:27] sample_rate_hz: uint32 LE (Hz, max ~4.29 GHz)
//   [28:31] json_offset: uint32 LE (byte offset where JSON starts)
//
//   [32 .. 32+bins*2-1]  spectrum: int16 LE, dB × 100
//   [json_offset ..]     JSON: {"signals":[...],"debug":{...}}

const binaryHeaderSize = 32

func encodeBinaryFrame(frame SpectrumFrame, targetBins int) []byte {
	spectrum := frame.Spectrum
	srcBins := len(spectrum)
	if targetBins <= 0 || targetBins > srcBins {
		targetBins = srcBins
	}

	var decimated []float64
	if targetBins < srcBins && targetBins > 0 {
		decimated = decimateSpectrum(spectrum, targetBins)
	} else {
		decimated = spectrum
		targetBins = srcBins
	}

	// JSON-encode signals + debug (full fidelity)
	jsonPart, _ := json.Marshal(struct {
		Signals []detector.Signal `json:"signals"`
		Debug   *SpectrumDebug   `json:"debug,omitempty"`
	}{
		Signals: frame.Signals,
		Debug:   frame.Debug,
	})

	specBytes := targetBins * 2
	jsonOffset := uint32(binaryHeaderSize + specBytes)
	totalSize := int(jsonOffset) + len(jsonPart)
	buf := make([]byte, totalSize)

	// Header
	buf[0] = 0x53 // 'S'
	buf[1] = 0x50 // 'P'
	binary.LittleEndian.PutUint16(buf[2:4], 4) // version 4
	binary.LittleEndian.PutUint64(buf[4:12], uint64(frame.Timestamp))
	binary.LittleEndian.PutUint64(buf[12:20], math.Float64bits(frame.CenterHz))
	binary.LittleEndian.PutUint32(buf[20:24], uint32(targetBins))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(frame.SampleHz))
	binary.LittleEndian.PutUint32(buf[28:32], jsonOffset)

	// Spectrum (int16, dB × 100)
	off := binaryHeaderSize
	for i := 0; i < targetBins; i++ {
		v := decimated[i] * 100
		if v > 32767 {
			v = 32767
		} else if v < -32767 {
			v = -32767
		}
		binary.LittleEndian.PutUint16(buf[off:off+2], uint16(int16(v)))
		off += 2
	}

	// JSON signals + debug
	copy(buf[jsonOffset:], jsonPart)

	return buf
}

// decimateSpectrum reduces bins via peak-hold within each group.
func decimateSpectrum(spectrum []float64, targetBins int) []float64 {
	src := len(spectrum)
	out := make([]float64, targetBins)
	ratio := float64(src) / float64(targetBins)
	for i := 0; i < targetBins; i++ {
		lo := int(float64(i) * ratio)
		hi := int(float64(i+1) * ratio)
		if hi > src {
			hi = src
		}
		if lo >= hi {
			if lo < src {
				out[i] = spectrum[lo]
			}
			continue
		}
		peak := spectrum[lo]
		for j := lo + 1; j < hi; j++ {
			if spectrum[j] > peak {
				peak = spectrum[j]
			}
		}
		out[i] = peak
	}
	return out
}



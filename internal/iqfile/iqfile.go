// Package iqfile reads and writes raw complex64 baseband IQ snapshots and
// provides a replay sdr.Source, so a real capture can be run deterministically
// through the whole pipeline (real-world regression + synth calibration basis).
//
// File format (little-endian):
//
//	magic     [8]byte = "SDRIQ001"
//	sampleRate int64
//	centerHz   float64
//	numSamples int64
//	data       numSamples × { re float32, im float32 }
package iqfile

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"time"

	"sdr-wideband-suite/internal/sdr"
)

var magic = [8]byte{'S', 'D', 'R', 'I', 'Q', '0', '0', '1'}

// Meta describes a snapshot.
type Meta struct {
	SampleRate int
	CenterHz   float64
	Samples    int
}

// Write stores iq to path with the given sample rate and center frequency.
func Write(path string, iq []complex64, sampleRate int, centerHz float64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriterSize(f, 1<<20)
	if _, err := w.Write(magic[:]); err != nil {
		return err
	}
	hdr := make([]byte, 24)
	binary.LittleEndian.PutUint64(hdr[0:], uint64(sampleRate))
	binary.LittleEndian.PutUint64(hdr[8:], math.Float64bits(centerHz))
	binary.LittleEndian.PutUint64(hdr[16:], uint64(len(iq)))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	buf := make([]byte, 8)
	for _, v := range iq {
		binary.LittleEndian.PutUint32(buf[0:], math.Float32bits(real(v)))
		binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(imag(v)))
		if _, err := w.Write(buf); err != nil {
			return err
		}
	}
	return w.Flush()
}

// Read loads a snapshot.
func Read(path string) ([]complex64, Meta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Meta{}, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	var m [8]byte
	if _, err := io.ReadFull(r, m[:]); err != nil {
		return nil, Meta{}, err
	}
	if m != magic {
		return nil, Meta{}, fmt.Errorf("iqfile: bad magic")
	}
	hdr := make([]byte, 24)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, Meta{}, err
	}
	sampleRate := int(binary.LittleEndian.Uint64(hdr[0:]))
	centerHz := math.Float64frombits(binary.LittleEndian.Uint64(hdr[8:]))
	n := int(binary.LittleEndian.Uint64(hdr[16:]))
	iq := make([]complex64, n)
	buf := make([]byte, 8)
	for i := 0; i < n; i++ {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, Meta{}, err
		}
		re := math.Float32frombits(binary.LittleEndian.Uint32(buf[0:]))
		im := math.Float32frombits(binary.LittleEndian.Uint32(buf[4:]))
		iq[i] = complex(re, im)
	}
	return iq, Meta{SampleRate: sampleRate, CenterHz: centerHz, Samples: n}, nil
}

// Source replays a snapshot as an sdr.Source, looping at end. Delivery is paced
// to real-time (the recorded sample rate) so the time-based ring buffer and RDS
// throttling behave exactly as with live hardware. Set Paced=false (via
// NewSourceUnpaced) for fast offline processing in tests.
type Source struct {
	mu        sync.Mutex
	iq        []complex64
	pos       int
	meta      Meta
	paced     bool
	start     time.Time
	delivered int64
}

// NewSource loads path and returns a real-time-paced replay source.
func NewSource(path string) (*Source, Meta, error) {
	iq, meta, err := Read(path)
	if err != nil {
		return nil, Meta{}, err
	}
	return &Source{iq: iq, meta: meta, paced: true}, meta, nil
}

func (s *Source) Start() error { s.start = time.Now(); return nil }
func (s *Source) Stop() error  { return nil }

func (s *Source) ReadIQ(n int) ([]complex64, error) {
	if n <= 0 {
		return nil, nil
	}
	s.mu.Lock()
	if len(s.iq) == 0 {
		s.mu.Unlock()
		return make([]complex64, n), nil
	}
	out := make([]complex64, n)
	for i := 0; i < n; i++ {
		out[i] = s.iq[s.pos]
		s.pos++
		if s.pos >= len(s.iq) {
			s.pos = 0 // loop
		}
	}
	s.delivered += int64(n)
	s.mu.Unlock()
	return out, nil
}

// Stats reports a real-time backlog: the number of samples that have "arrived"
// since Start (at the recorded sample rate) but not yet been read. The DSP loop
// drains this each frame, so replay paces exactly like live hardware without
// per-read sleeps. (Unpaced sources report the whole file as available.)
func (s *Source) Stats() sdr.SourceStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.paced || s.meta.SampleRate <= 0 || s.start.IsZero() {
		return sdr.SourceStats{BufferSamples: len(s.iq)}
	}
	arrived := int64(time.Since(s.start).Seconds() * float64(s.meta.SampleRate))
	backlog := arrived - s.delivered
	if backlog < 0 {
		backlog = 0
	}
	return sdr.SourceStats{BufferSamples: int(backlog)}
}

func (s *Source) Flush() {}

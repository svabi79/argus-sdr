package gpudemod

import (
	"math"
	"math/cmplx"
	"testing"

	"sdr-wideband-suite/internal/dsp"
)

func TestMixedBandwidthBatch(t *testing.T) {
	if !Available() {
		t.Skip("no GPU")
	}
	sampleRate := 2048000
	n := 2048
	iq := makeSyntheticIQ(n, sampleRate, []float64{50e3, -120e3, 300e3, -80e3})
	jobs := []ExtractJob{
		{OffsetHz: 50e3, BW: 12000, OutRate: 48000},
		{OffsetHz: -120e3, BW: 150000, OutRate: 192000},
		{OffsetHz: 300e3, BW: 3000, OutRate: 48000},
		{OffsetHz: -80e3, BW: 500, OutRate: 48000},
	}
	cpuOuts := make([][]complex64, len(jobs))
	for i, job := range jobs {
		cpuOuts[i] = extractCPU(iq, sampleRate, job)
	}
	runner, err := NewBatchRunner(n, sampleRate)
	if err != nil {
		t.Fatalf("NewBatchRunner: %v", err)
	}
	defer runner.Close()
	gpuOuts, rates, err := runner.ShiftFilterDecimateBatch(iq, jobs)
	if err != nil {
		t.Fatalf("Batch: %v", err)
	}
	for i := range jobs {
		if !complexSliceClose(cpuOuts[i], gpuOuts[i], 1e-3) {
			t.Errorf("job %d: GPU/CPU mismatch (rate=%d)", i, rates[i])
		}
	}
}

func makeSyntheticIQ(n int, sr int, freqs []float64) []complex64 {
	iq := make([]complex64, n)
	for _, f := range freqs {
		for i := range iq {
			phase := 2 * math.Pi * f * float64(i) / float64(sr)
			iq[i] += complex(float32(math.Cos(phase)), float32(math.Sin(phase)))
		}
	}
	return iq
}

func extractCPU(iq []complex64, sr int, job ExtractJob) []complex64 {
	shifted := dsp.FreqShift(iq, sr, job.OffsetHz)
	cutoff := job.BW / 2
	if cutoff < 200 {
		cutoff = 200
	}
	taps := dsp.LowpassFIR(cutoff, sr, 101)
	filtered := dsp.ApplyFIR(shifted, taps)
	decim := int(math.Round(float64(sr) / float64(job.OutRate)))
	if decim < 1 {
		decim = 1
	}
	return dsp.Decimate(filtered, decim)
}

func complexSliceClose(a, b []complex64, tol float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if cmplx.Abs(complex128(a[i]-b[i])) > tol {
			return false
		}
	}
	return true
}

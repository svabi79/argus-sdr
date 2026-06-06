// Package estimate provides physically meaningful signal-parameter estimators
// used by the refinement layer: occupied bandwidth, center, and SNR.
//
// The detector's geometric bandwidth (threshold-crossing bin count) is biased
// per modulation kind (FM under-measured, narrowband over-measured). These
// estimators replace it with a power-containment ("occupied bandwidth", ITU-R
// SM.328 style) measure that is robust to spectral shape.
//
// See docs/detection-rework-plan-2026-06-06.md (Phase R, step R1).
package estimate

import (
	"math"
	"sort"
)

const (
	// noiseMarginDb is how far a bin must exceed the estimated noise floor to be
	// part of the signal blob.
	noiseMarginDb = 3.0
	// minSignalDb is the minimum peak-over-noise for a region to hold a signal.
	minSignalDb = 6.0
	// smoothHz is the dB-domain smoothing width used for peak/blob detection.
	smoothHz = 1800.0
)

// Occupancy holds the result of an occupied-bandwidth estimate over a local
// spectral region.
type Occupancy struct {
	BandwidthHz   float64 // beta-occupied bandwidth
	CenterBin     float64 // power centroid, in bins relative to the region start
	LowBin        float64 // lower occupancy edge (fractional bin, region-relative)
	HighBin       float64 // upper occupancy edge (fractional bin, region-relative)
	BlobLowBin    int     // lower edge of the signal blob (region-relative)
	BlobHighBin   int     // upper edge of the signal blob (region-relative)
	NoiseFloorDb  float64 // estimated noise pedestal in the region (dB)
	PeakDb        float64 // raw peak power within the blob (dB)
	SignalPowerDb float64 // total noise-subtracted in-band power (dB)
	OK            bool
}

// SNRDb returns the peak-over-noise of the occupancy in dB.
func (o Occupancy) SNRDb() float64 { return o.PeakDb - o.NoiseFloorDb }

// OccupiedBandwidthDb estimates the fraction-occupied bandwidth of a signal
// within a local dB power region (e.g. a slice of the surveillance spectrum
// around a candidate). binWidthHz is the frequency span of one bin; fraction is
// the containment target (e.g. 0.99).
//
// A noise pedestal is estimated from the quiet outer edges of the region and
// subtracted, so the result reflects the signal rather than the region's total
// power. The region therefore must be somewhat wider than the signal so its
// edges actually contain noise.
func OccupiedBandwidthDb(regionDb []float64, binWidthHz, fraction float64) Occupancy {
	n := len(regionDb)
	if n < 4 || binWidthHz <= 0 {
		return Occupancy{}
	}
	if fraction <= 0 || fraction >= 1 {
		fraction = 0.99
	}

	noiseDb := edgeNoiseDb(regionDb)
	noiseLin := dbToLin(noiseDb)

	// Smooth the dB region for peak/blob detection only (power is measured on the
	// raw spectrum). Smoothing fills the small dips of structured spectra (FM)
	// while averaging down isolated noise spikes, so the blob neither fragments
	// nor chains out into the noise.
	smooth := boxSmoothDb(regionDb, smoothBins(binWidthHz))

	// 1) Peak. A region with no bin clearly above noise carries no signal.
	peakBin, peakDb := 0, math.Inf(-1)
	for i, db := range smooth {
		if db > peakDb {
			peakDb, peakBin = db, i
		}
	}
	if peakDb-noiseDb < minSignalDb {
		return Occupancy{NoiseFloorDb: noiseDb}
	}

	// 2) Contiguous signal blob around the peak on the smoothed spectrum: extend
	// while bins stay above a small margin over noise, tolerating a couple of
	// bins of dip. Bounds the estimate to the signal and excludes far noise.
	thr := noiseDb + noiseMarginDb
	const gap = 2
	blo, bhi := peakBin, peakBin
	for i, miss := peakBin-1, 0; i >= 0; i-- {
		if smooth[i] > thr {
			blo, miss = i, 0
		} else if miss++; miss > gap {
			break
		}
	}
	for i, miss := peakBin+1, 0; i < n; i++ {
		if smooth[i] > thr {
			bhi, miss = i, 0
		} else if miss++; miss > gap {
			break
		}
	}

	// 3) Noise-subtracted power inside the blob only; track the raw peak for SNR.
	sig := make([]float64, n)
	var total, wsum float64
	rawPeakDb := math.Inf(-1)
	for i := blo; i <= bhi; i++ {
		if regionDb[i] > rawPeakDb {
			rawPeakDb = regionDb[i]
		}
		if regionDb[i] <= thr {
			continue
		}
		p := dbToLin(regionDb[i]) - noiseLin
		if p < 0 {
			p = 0
		}
		sig[i] = p
		total += p
		wsum += float64(i) * p
	}
	if total <= 0 {
		return Occupancy{NoiseFloorDb: noiseDb}
	}
	centroid := wsum / total

	// 4) Occupied band: trim `(1-fraction)/2` of the blob power from each edge.
	// Walking from the blob edges (not the region edges) keeps the estimate
	// bounded to the signal while still crossing zero gaps inside line spectra
	// (single-tone FM/AM produce discrete Bessel/sideband lines).
	tail := (1 - fraction) / 2 * total
	low := walkFromEdge(sig, blo, bhi, tail, false)
	high := walkFromEdge(sig, blo, bhi, tail, true)
	if high <= low {
		return Occupancy{NoiseFloorDb: noiseDb}
	}

	return Occupancy{
		BandwidthHz:   (high - low) * binWidthHz,
		CenterBin:     centroid,
		LowBin:        low,
		HighBin:       high,
		BlobLowBin:    blo,
		BlobHighBin:   bhi,
		NoiseFloorDb:  noiseDb,
		PeakDb:        rawPeakDb,
		SignalPowerDb: 10 * math.Log10(total+1e-30),
		OK:            true,
	}
}

// walkFromEdge returns the fractional bin where the cumulative power from the
// low edge (or high edge if fromHigh) of [lo,hi] first reaches `tail`.
func walkFromEdge(sig []float64, lo, hi int, tail float64, fromHigh bool) float64 {
	var c float64
	if !fromHigh {
		for i := lo; i <= hi; i++ {
			if c+sig[i] >= tail {
				if sig[i] <= 0 {
					return float64(i)
				}
				return float64(i) + (tail-c)/sig[i]
			}
			c += sig[i]
		}
		return float64(hi)
	}
	for i := hi; i >= lo; i-- {
		if c+sig[i] >= tail {
			if sig[i] <= 0 {
				return float64(i + 1)
			}
			return float64(i+1) - (tail-c)/sig[i]
		}
		c += sig[i]
	}
	return float64(lo + 1)
}

// edgeNoiseDb estimates the noise pedestal (dB) as the median of the outer 15%
// of bins on each side of the region, where signal energy is least likely.
func edgeNoiseDb(regionDb []float64) float64 {
	n := len(regionDb)
	edge := n * 15 / 100
	if edge < 1 {
		edge = 1
	}
	edges := make([]float64, 0, 2*edge)
	for i := 0; i < edge; i++ {
		edges = append(edges, regionDb[i])
	}
	for i := n - edge; i < n; i++ {
		edges = append(edges, regionDb[i])
	}
	sort.Float64s(edges)
	return edges[len(edges)/2]
}

func dbToLin(db float64) float64 {
	return math.Pow(10, db/10)
}

// smoothBins returns an odd box-filter width (>=3) corresponding to smoothHz.
func smoothBins(binWidthHz float64) int {
	w := int(math.Round(smoothHz / binWidthHz))
	if w < 3 {
		w = 3
	}
	if w%2 == 0 {
		w++
	}
	return w
}

// boxSmoothDb returns a centered box-average of v with the given odd width.
func boxSmoothDb(v []float64, width int) []float64 {
	n := len(v)
	out := make([]float64, n)
	half := width / 2
	for i := 0; i < n; i++ {
		lo := i - half
		hi := i + half
		if lo < 0 {
			lo = 0
		}
		if hi > n-1 {
			hi = n - 1
		}
		var s float64
		for j := lo; j <= hi; j++ {
			s += v[j]
		}
		out[i] = s / float64(hi-lo+1)
	}
	return out
}

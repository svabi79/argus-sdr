package estimate

// Refined is the result of re-estimating a candidate's parameters from the
// surveillance spectrum: occupied bandwidth plus a refined center, expressed in
// absolute bin coordinates of the spectrum that was passed in.
type Refined struct {
	BandwidthHz  float64
	CenterBin    float64 // power centroid, absolute spectrum bin
	LowBin       float64 // occupancy edges, absolute spectrum bins
	HighBin      float64
	NoiseFloorDb float64
	PeakDb       float64
	SNRDb        float64 // peak-over-noise
	OK           bool
}

// RefineFromSpectrum re-estimates a candidate's occupied bandwidth and center
// from a local window of the (dB) surveillance spectrum, given the candidate's
// coarse extent in bins.
//
// The analysis region is grown adaptively: the detector's coarse bandwidth can
// badly under-estimate wide signals (FM), so a region sized from it would clip
// them — but a fixed-wide region would swallow close neighbours. Starting from
// the coarse extent, the region is doubled until the signal blob sits inside it
// with a noise margin on both sides (so the edge-noise estimate is valid and the
// signal is not clipped), or a cap is reached.
func RefineFromSpectrum(specDb []float64, firstBin, lastBin int, binWidthHz, fraction float64) Refined {
	n := len(specDb)
	if n == 0 || firstBin < 0 || lastBin >= n || lastBin < firstBin || binWidthHz <= 0 {
		return Refined{}
	}
	coarse := lastBin - firstBin + 1
	center := (firstBin + lastBin) / 2
	pad := coarse
	if pad < 16 {
		pad = 16
	}

	var last Refined
	for iter := 0; iter < 7; iter++ {
		regStart := center - coarse/2 - pad
		regEnd := center + coarse/2 + pad
		if regStart < 0 {
			regStart = 0
		}
		if regEnd > n-1 {
			regEnd = n - 1
		}
		region := specDb[regStart : regEnd+1]
		occ := OccupiedBandwidthDb(region, binWidthHz, fraction)
		if !occ.OK {
			return Refined{NoiseFloorDb: occ.NoiseFloorDb}
		}
		last = Refined{
			BandwidthHz:  occ.BandwidthHz,
			CenterBin:    float64(regStart) + occ.CenterBin,
			LowBin:       float64(regStart) + occ.LowBin,
			HighBin:      float64(regStart) + occ.HighBin,
			NoiseFloorDb: occ.NoiseFloorDb,
			PeakDb:       occ.PeakDb,
			SNRDb:        occ.SNRDb(),
			OK:           true,
		}
		// Converged if the signal blob has a noise margin on both sides and the
		// region did not have to clamp to the spectrum edge on a side the blob
		// reaches.
		regLen := len(region)
		margin := regLen / 8
		blobClearLow := occ.BlobLowBin > margin || regStart == 0
		blobClearHigh := occ.BlobHighBin < regLen-margin || regEnd == n-1
		if blobClearLow && blobClearHigh {
			break
		}
		pad *= 2
	}
	return last
}

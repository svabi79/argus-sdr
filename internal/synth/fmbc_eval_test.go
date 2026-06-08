//go:build bench

// FM-BC detection/classification evaluation (operator-directed, 2026-06-08).
//
// Live symptom on the FM broadcast band (102 MHz): the strong WFM stations ARE
// found, but with "extrem viele Nebendetektionen" (each station shatters into many
// narrow detections), which starves the classification budget so almost every
// signal shows as "carrier" (nil class) and nothing is demodulable.
//
// Operator's insight: a human reads bandwidth far better off the WATERFALL than off
// one spectrum line, because a real station is PERSISTENT in time while noise/MPX
// structure is not. The DSP equivalent is time-INTEGRATION of the power spectrum
// (Welch within a frame / EMA across frames / per-bin occupancy). This harness
// measures, on the real fm_bc oracle (Constitution IV/V — synth is too clean):
//   - Phase 0: how badly the live detector (multi_scale, ms_k 24, ms_min_bw 200)
//     fragments each WFM station, and how integration collapses the fragments.
//
//	go test -tags bench -run 'TestFMBC' ./internal/synth/ -v
package synth_test

import (
	"math"
	"sort"
	"testing"
	"time"

	"sdr-wideband-suite/internal/config"
	"sdr-wideband-suite/internal/detector"
	fftutil "sdr-wideband-suite/internal/fft"
	"sdr-wideband-suite/internal/iqfile"
)

const fmbcPath = "../../data/snapshots/fm_bc.cf32"

// fmbcLiveConfig mirrors config.autosave.yaml's detector block on the FM-BC band
// (multi-scale, ms_k 24, ms_min_bw 200, EMA 0.025) — the exact config producing
// the live Nebendetektionen.
func fmbcLiveConfig() config.DetectorConfig {
	c := detectorConfig()
	c.ThresholdDb = -60
	c.HoldMs = 700
	c.EmaAlpha = 0.025
	c.HysteresisDb = 10
	c.MinStableFrames = 5
	c.CFARMode = "GOSCA"
	c.CFARGuardHz = 2000
	c.CFARTrainHz = 20000
	c.CFARScaleDb = 8
	c.EdgeMarginDb = 6
	c.MaxSignalBwHz = 260000
	c.MergeGapHz = 5000
	c.MultiScale = true
	c.MSK = 24
	c.MSMinSNRDb = 6
	c.MSMinBwHz = 200
	return c
}

// occupancyConfig is the proposed FM-BC fix: contiguous occupied-band detection on
// the integrated spectrum (one WFM plateau = one signal).
func occupancyConfig() config.DetectorConfig {
	c := fmbcLiveConfig()
	c.MultiScale = false
	c.OccupancyDetect = true
	c.OccThreshDb = 6
	c.OccMinPeakDb = 10
	c.OccMergeGapHz = 6000 // bridge brief in-plateau dips (WFM audio dynamics)
	c.OccMinBwHz = 30000   // FM-BC: stations are wide; drop sub-structure/spurious blips
	c.OccMaxBwHz = 180000  // cap strong WFM at ~channel width (un-bridge, keep MPX/RDS)
	return c
}

// welchPSDSegments returns a Welch PSD that averages about `segs` half-overlapped
// FFT windows of size `seg`, taken from the most recent samples (so a higher segs
// means more time-integration = a "deeper waterfall").
func welchPSDSegments(iq []complex64, seg, segs int) []float64 {
	if segs < 1 {
		segs = 1
	}
	need := seg + (segs-1)*seg/2
	if need > len(iq) {
		need = len(iq)
	}
	win := fftutil.Hann(seg)
	return fftutil.WelchPSD(iq[len(iq)-need:], seg, 0.5, win)
}

// strongRefStations returns the strong (broadcast-tier) reference emissions on the
// FM-BC oracle — the ground-truth station list the detector should reproduce 1:1.
func strongRefStations(iq []complex64, fs int) []refSig {
	refs, _ := welchReference(iq, fs, refSeg, refMinPeakDb, refAboveFloor)
	var strong []refSig
	for _, r := range refs {
		if r.peakDb >= 20 && r.bwHz >= 40e3 { // WFM broadcast: strong + wide
			strong = append(strong, r)
		}
	}
	sort.Slice(strong, func(a, b int) bool { return strong[a].centerHz < strong[b].centerHz })
	return strong
}

// countPerStation buckets detections by which strong station they fall under (center
// within ±matchHz of a station center), and counts the rest as "other" (spurious or
// off-station fragments).
func countPerStation(dets []det, stations []refSig, matchHz float64) (perStation []int, other int) {
	perStation = make([]int, len(stations))
	for _, d := range dets {
		best := -1
		bestDist := matchHz
		for i, s := range stations {
			dist := math.Abs(d.c - s.centerHz)
			if dist <= bestDist {
				best = i
				bestDist = dist
			}
		}
		if best >= 0 {
			perStation[best]++
		} else {
			other++
		}
	}
	return perStation, other
}

// TestFMBCFragmentation (Phase 0) quantifies how the live multi-scale detector
// fragments each WFM station, both via the faithful live path (per-frame single FFT
// + EMA 0.025) and on increasingly time-integrated spectra (Welch N segments) — to
// validate the operator's "integrate like a waterfall" hypothesis with numbers.
func TestFMBCFragmentation(t *testing.T) {
	iq, meta, err := iqfile.Read(fmbcPath)
	if err != nil {
		t.Skipf("oracle: %v", err)
	}
	const fft = 65536
	fs := meta.SampleRate
	stations := strongRefStations(iq, fs)
	t.Logf("== FM-BC %.3f MHz / %.3f MS/s, bin=%.1f Hz ==", meta.CenterHz/1e6, float64(fs)/1e6, float64(fs)/fft)
	t.Logf("ground truth = %d strong WFM stations:", len(stations))
	for i, s := range stations {
		t.Logf("   #%d  %.4f MHz (off %+.0f kHz)  bw=%.0f kHz  +%.0f dB",
			i, (meta.CenterHz+s.centerHz)/1e6, s.centerHz/1e3, s.bwHz/1e3, s.peakDb)
	}

	// (A) Faithful live path: per-frame single FFT + EMA 0.025, multi-scale detector.
	live := realSceneFrames(iq, fs, fft, fmbcLiveConfig(), sweepFrames, sweepWarm)
	var liveDets []det
	for _, c := range live {
		if c.hits >= clusterMinHits {
			liveDets = append(liveDets, det{c: c.center, bw: c.bw})
		}
	}
	ps, other := countPerStation(liveDets, stations, 100e3)
	t.Logf("--- (A) LIVE path (single FFT/frame + EMA 0.025, multi_scale ms_k24 ms_min_bw200) ---")
	t.Logf("   total stable detections = %d  (expected ~%d)", len(liveDets), len(stations))
	for i := range stations {
		t.Logf("   station #%d: %d detections", i, ps[i])
	}
	t.Logf("   off-station/spurious: %d", other)

	// (B) Integration sweep: multi-scale vs occupancy on a single Welch PSD with
	// increasing segment counts (detector EMA off, isolates spectral integration).
	t.Logf("--- (B) integration sweep: multi-scale vs OCCUPANCY (Welch-N PSD, EMA off) ---")
	msCfg := fmbcLiveConfig()
	msCfg.EmaAlpha = 1.0
	occCfg := occupancyConfig()
	occCfg.EmaAlpha = 1.0
	runDet := func(cfg config.DetectorConfig, psd []float64) ([]det, []int, int) {
		d := detector.New(cfg, fs, fft)
		_, raw := d.Process(time.Unix(0, 0), psd, 0)
		var dets []det
		for _, s := range raw {
			dets = append(dets, det{c: s.CenterHz, bw: s.BWHz})
		}
		ps, other := countPerStation(dets, stations, 100e3)
		return dets, ps, other
	}
	for _, segs := range []int{1, 4, 16, 64} {
		psd := welchPSDSegments(iq, fft, segs)
		_, mps, mo := runDet(msCfg, psd)
		_, ops, oo := runDet(occCfg, psd)
		mTot := mps[0] + mps[1] + mo
		oTot := ops[0] + ops[1] + oo
		t.Logf("   segs=%-3d | multiScale total=%-3d st=%v spur=%-3d | OCCUPANCY total=%-3d st=%v spur=%d",
			segs, mTot, mps, mo, oTot, ops, oo)
	}

	// (C) Occupancy via the faithful live path (per-frame single FFT + EMA 0.025).
	// 60 frames / 20 warmup so the slow EMA converges (live runs continuously, so
	// the smooth spectrum is more settled than the 24-frame default proxy).
	occLive := realSceneFrames(iq, fs, fft, occupancyConfig(), 60, 20)
	var occDets []det
	for _, c := range occLive {
		if c.hits >= clusterMinHits {
			occDets = append(occDets, det{c: c.center, bw: c.bw})
		}
	}
	ops, oo := countPerStation(occDets, stations, 100e3)
	t.Logf("--- (C) OCCUPANCY live path (single FFT/frame + EMA 0.025) ---")
	t.Logf("   total stable detections = %d (was %d with multi-scale)", len(occDets), len(liveDets))
	for i := range stations {
		t.Logf("   station #%d: %d detections (bw≈%.0f kHz)", i, ops[i], stationBw(occDets, stations[i]))
	}
	t.Logf("   off-station/spurious: %d", oo)
}

// stationBw returns the bandwidth of the detection nearest a station center (for a
// sanity check that the occupancy detector reports the full plateau width).
func stationBw(dets []det, s refSig) float64 {
	best, bestDist := 0.0, math.MaxFloat64
	for _, d := range dets {
		if dist := math.Abs(d.c - s.centerHz); dist < bestDist {
			bestDist = dist
			best = d.bw
		}
	}
	return best / 1e3
}

// TestFMBCProfile dumps the time-integrated (Welch-32) PSD across station #0's band
// so we can see whether a WFM station is ONE solid plateau (then a single
// occupied-band detector nails it) or has internal nulls the multi-scale detector
// trips on. Profile is in ~5 kHz steps, dB over the median floor.
func TestFMBCProfile(t *testing.T) {
	iq, meta, err := iqfile.Read(fmbcPath)
	if err != nil {
		t.Skipf("oracle: %v", err)
	}
	const fft = 65536
	fs := meta.SampleRate
	psd := welchPSDSegments(iq, fft, 32)
	floor := medianf(psd)
	binHz := float64(fs) / float64(fft)
	// station #0 at off -900 kHz: scan -1050..-750 kHz.
	t.Logf("== integrated (Welch-32) profile, station #0 (off -900 kHz), floor=%.1f dB ==", floor)
	t.Logf("   %-10s %-8s %s", "off-kHz", "dB>flr", "bar")
	for off := -1050e3; off <= -750e3; off += 5e3 {
		bin := int(off/binHz) + fft/2
		if bin < 0 || bin >= len(psd) {
			continue
		}
		over := psd[bin] - floor
		n := int(over)
		if n < 0 {
			n = 0
		}
		bar := ""
		for k := 0; k < n && k < 40; k++ {
			bar += "#"
		}
		t.Logf("   %-10.0f %-8.1f %s", off/1e3, over, bar)
	}
}

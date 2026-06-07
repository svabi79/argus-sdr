package pipeline

import "sort"

// ScaleFuseOptions tunes scale-aware fusion. The zero value uses sane defaults
// via scaleFuseDefaults.
type ScaleFuseOptions struct {
	// MinSplitSNRDb: a fine-pass candidate must exceed this peak-over-noise to
	// count as a separate emission inside a coarse candidate, so a sharp pass's
	// sidelobe/noise spike does not split a single station.
	MinSplitSNRDb float64
	// MinSplitSeparationHz: two fine centers must be at least this far apart to be
	// treated as distinct emissions (below it they are the same station seen twice).
	MinSplitSeparationHz float64
}

func scaleFuseDefaults(o ScaleFuseOptions) ScaleFuseOptions {
	if o.MinSplitSNRDb == 0 {
		o.MinSplitSNRDb = 8
	}
	if o.MinSplitSeparationHz == 0 {
		o.MinSplitSeparationHz = 30000
	}
	return o
}

// FuseScaleAware resolves a coarse detection pass against a finer/sharper pass
// into one candidate set, per docs/detection-architecture.md (L1, contracts C1/C2).
//
// The two passes have complementary failure modes (quantified on the synth
// benchmark): the coarse pass measures occupied bandwidth well but BRIDGES close
// emissions into one over-wide candidate; the fine/sharp pass SEPARATES close
// emissions but under-measures wide ones. So:
//
//   - the fine pass supplies the emission COUNT and CENTERS (separation), and
//   - the coarse pass supplies WIDTH.
//
// For each coarse candidate, the significant, mutually separated fine candidates
// whose centers fall inside it decide the split: with k>=2 the coarse candidate
// is divided at the midpoints between adjacent fine centers (close wide emissions
// genuinely overlap, so the shared span is partitioned), each child carrying the
// fine center and the coarse-derived width of its slice; with k<=1 the coarse
// candidate is kept whole (a single wide emission must not be fragmented). Fine
// candidates that fall inside no coarse candidate are emitted as-is (emissions
// only the fine pass found). The result is sorted by center.
//
// It is a pure function so it is unit- and benchmark-testable independently of
// the live pipeline.
func FuseScaleAware(coarse, fine []Candidate, opts ScaleFuseOptions) []Candidate {
	opts = scaleFuseDefaults(opts)
	if len(coarse) == 0 {
		return append([]Candidate(nil), fine...)
	}

	// Significant, separated fine candidates, sorted by center.
	sig := make([]Candidate, 0, len(fine))
	for _, f := range fine {
		if f.SNRDb >= opts.MinSplitSNRDb {
			sig = append(sig, f)
		}
	}
	sort.Slice(sig, func(i, j int) bool { return sig[i].CenterHz < sig[j].CenterHz })

	usedFine := make([]bool, len(sig))
	out := make([]Candidate, 0, len(coarse)+len(sig))

	for _, c := range coarse {
		left, right := candidateEdgesHz(c)
		// Collect significant fine centers inside this coarse candidate. All of
		// them are absorbed by this coarse candidate (marked used) so a dropped
		// too-close one does not leak into the fine-only pass below.
		inside := make([]int, 0, 4)
		for i := range sig {
			if usedFine[i] {
				continue
			}
			if sig[i].CenterHz >= left && sig[i].CenterHz <= right {
				inside = append(inside, i)
				usedFine[i] = true
			}
		}
		// Drop fine centers that are not separated enough from their predecessor
		// (same station resolved twice): keep the stronger of a too-close pair.
		inside = dropUnseparated(sig, inside, opts.MinSplitSeparationHz)

		if len(inside) <= 1 {
			// Single (or no) emission: keep the coarse candidate whole. Adopt the
			// fine center when available (more accurate), keep the coarse width.
			cc := c
			if len(inside) == 1 {
				cc.CenterHz = sig[inside[0]].CenterHz
			}
			out = append(out, cc)
			continue
		}

		// k >= 2: split the coarse span at midpoints between adjacent fine centers.
		for n, idx := range inside {
			lo := left
			if n > 0 {
				lo = 0.5 * (sig[inside[n-1]].CenterHz + sig[idx].CenterHz)
			}
			hi := right
			if n < len(inside)-1 {
				hi = 0.5 * (sig[idx].CenterHz + sig[inside[n+1]].CenterHz)
			}
			child := sig[idx]
			child.CenterHz = sig[idx].CenterHz
			child.BandwidthHz = hi - lo
			child.Source = "scale-fused-split"
			MergeCandidateEvidence(&child, c)
			out = append(out, child)
		}
	}

	// Fine-only emissions (inside no coarse candidate).
	for i := range sig {
		if !usedFine[i] {
			out = append(out, sig[i])
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].CenterHz < out[j].CenterHz })
	return out
}

// candidateEdgesHz returns the [left,right] frequency span of a candidate.
func candidateEdgesHz(c Candidate) (float64, float64) {
	bw := c.BandwidthHz
	if bw <= 0 {
		if span := candidateSpanHz(c); span > 0 {
			bw = span
		} else {
			bw = 25000
		}
	}
	return c.CenterHz - bw/2, c.CenterHz + bw/2
}

// dropUnseparated removes fine indices whose center is within minSepHz of the
// previously kept one, keeping the stronger (higher SNR) of the pair. idxs is
// assumed sorted by center.
func dropUnseparated(sig []Candidate, idxs []int, minSepHz float64) []int {
	if len(idxs) <= 1 {
		return idxs
	}
	kept := make([]int, 0, len(idxs))
	kept = append(kept, idxs[0])
	for _, idx := range idxs[1:] {
		last := kept[len(kept)-1]
		if sig[idx].CenterHz-sig[last].CenterHz < minSepHz {
			if sig[idx].SNRDb > sig[last].SNRDb {
				kept[len(kept)-1] = idx
			}
			continue
		}
		kept = append(kept, idx)
	}
	return kept
}

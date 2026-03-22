package pipeline

import "math"

func AddCandidateEvidence(candidate *Candidate, evidence LevelEvidence) {
	if candidate == nil {
		return
	}
	levelName := evidence.Level.Name
	if levelName == "" {
		levelName = "unknown"
	}
	for _, ev := range candidate.Evidence {
		evLevel := ev.Level.Name
		if evLevel == "" {
			evLevel = "unknown"
		}
		if evLevel == levelName && ev.Provenance == evidence.Provenance {
			return
		}
	}
	candidate.Evidence = append(candidate.Evidence, evidence)
}

func MergeCandidateEvidence(dst *Candidate, src Candidate) {
	if dst == nil || len(src.Evidence) == 0 {
		return
	}
	for _, ev := range src.Evidence {
		AddCandidateEvidence(dst, ev)
	}
}

func CandidateEvidenceLevelCount(candidate Candidate) int {
	if len(candidate.Evidence) == 0 {
		return 0
	}
	levels := map[string]struct{}{}
	for _, ev := range candidate.Evidence {
		name := ev.Level.Name
		if name == "" {
			name = "unknown"
		}
		levels[name] = struct{}{}
	}
	return len(levels)
}

func FuseCandidates(primary []Candidate, derived []Candidate) []Candidate {
	if len(primary) == 0 && len(derived) == 0 {
		return nil
	}
	out := make([]Candidate, 0, len(primary)+len(derived))
	out = append(out, primary...)
	if len(derived) == 0 {
		return out
	}
	used := make([]bool, len(derived))
	for i := range out {
		for j, cand := range derived {
			if used[j] {
				continue
			}
			if !candidatesOverlap(out[i], cand) {
				continue
			}
			MergeCandidateEvidence(&out[i], cand)
			used[j] = true
		}
	}
	for j, cand := range derived {
		if used[j] {
			continue
		}
		out = append(out, cand)
	}
	return out
}

func candidatesOverlap(a Candidate, b Candidate) bool {
	spanA := candidateSpanHz(a)
	spanB := candidateSpanHz(b)
	if spanA <= 0 {
		spanA = 25000
	}
	if spanB <= 0 {
		spanB = 25000
	}
	guard := 0.0
	if binA, binB := candidateBinHz(a), candidateBinHz(b); binA > 0 || binB > 0 {
		guard = 0.5 * math.Max(binA, binB)
	}
	leftA := a.CenterHz - spanA/2 - guard
	rightA := a.CenterHz + spanA/2 + guard
	leftB := b.CenterHz - spanB/2 - guard
	rightB := b.CenterHz + spanB/2 + guard
	return leftA <= rightB && leftB <= rightA
}

func candidateSpanHz(candidate Candidate) float64 {
	if candidate.BandwidthHz > 0 {
		return candidate.BandwidthHz
	}
	if candidate.LastBin < candidate.FirstBin {
		return 0
	}
	binHz := candidateBinHz(candidate)
	if binHz <= 0 {
		return 0
	}
	return float64(candidate.LastBin-candidate.FirstBin+1) * binHz
}

func candidateBinHz(candidate Candidate) float64 {
	for _, ev := range candidate.Evidence {
		if ev.Level.BinHz > 0 {
			return ev.Level.BinHz
		}
		if ev.Level.SampleRate > 0 && ev.Level.FFTSize > 0 {
			return float64(ev.Level.SampleRate) / float64(ev.Level.FFTSize)
		}
	}
	return 0
}

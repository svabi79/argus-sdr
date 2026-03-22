package pipeline

import (
	"sort"
	"strings"
)

type ScheduledCandidate struct {
	Candidate  Candidate               `json:"candidate"`
	Priority   float64                 `json:"priority"`
	Tier       string                  `json:"tier,omitempty"`
	TierFloor  string                  `json:"tier_floor,omitempty"`
	Family     string                  `json:"family,omitempty"`
	FamilyRank int                     `json:"family_rank,omitempty"`
	Score      *RefinementScore        `json:"score,omitempty"`
	Breakdown  *RefinementScoreDetails `json:"breakdown,omitempty"`
}

type RefinementScoreModel struct {
	SNRWeight       float64 `json:"snr_weight"`
	BandwidthWeight float64 `json:"bandwidth_weight"`
	PeakWeight      float64 `json:"peak_weight"`
	EvidenceWeight  float64 `json:"evidence_weight"`
}

type RefinementScoreDetails struct {
	SNRScore       float64               `json:"snr_score"`
	BandwidthScore float64               `json:"bandwidth_score"`
	PeakScore      float64               `json:"peak_score"`
	PolicyBoost    float64               `json:"policy_boost"`
	MonitorBias    float64               `json:"monitor_bias,omitempty"`
	MonitorDetail  *MonitorWindowMatch   `json:"monitor_detail,omitempty"`
	EvidenceScore  float64               `json:"evidence_score"`
	EvidenceDetail *EvidenceScoreDetails `json:"evidence_detail,omitempty"`
}

type RefinementScore struct {
	Total     float64                `json:"total"`
	Breakdown RefinementScoreDetails `json:"breakdown"`
	Weights   *RefinementScoreModel  `json:"weights,omitempty"`
}

type RefinementWorkItem struct {
	Candidate Candidate               `json:"candidate"`
	Window    RefinementWindow        `json:"window,omitempty"`
	Execution *RefinementExecution    `json:"execution,omitempty"`
	Priority  float64                 `json:"priority,omitempty"`
	Score     *RefinementScore        `json:"score,omitempty"`
	Breakdown *RefinementScoreDetails `json:"breakdown,omitempty"`
	Status    string                  `json:"status,omitempty"`
	Reason    string                  `json:"reason,omitempty"`
	Admission *PriorityAdmission      `json:"admission,omitempty"`
}

type RefinementExecution struct {
	Stage      string  `json:"stage,omitempty"`
	SampleRate int     `json:"sample_rate,omitempty"`
	FFTSize    int     `json:"fft_size,omitempty"`
	CenterHz   float64 `json:"center_hz,omitempty"`
	SpanHz     float64 `json:"span_hz,omitempty"`
	Source     string  `json:"source,omitempty"`
}

const (
	RefinementStatusPlanned   = "planned"
	RefinementStatusAdmitted  = "admitted"
	RefinementStatusRunning   = "running"
	RefinementStatusCompleted = "completed"
	RefinementStatusDropped   = "dropped"
	RefinementStatusSkipped   = "skipped"
	RefinementStatusDisplaced = "displaced"
)

const (
	RefinementReasonPlanned      = "refinement:planned"
	RefinementReasonAdmitted     = "refinement:admitted"
	RefinementReasonRunning      = "refinement:running"
	RefinementReasonCompleted    = "refinement:completed"
	RefinementReasonMonitorGate  = "refinement:drop:monitor"
	RefinementReasonBelowSNR     = "refinement:drop:snr"
	RefinementReasonBudget       = "refinement:skip:budget"
	RefinementReasonDisabled     = "refinement:drop:disabled"
	RefinementReasonUnclassified = "refinement:drop:unclassified"
	RefinementReasonDisplaced    = "refinement:skip:displaced"
)

// BuildRefinementPlan scores and ranks candidates for costly local refinement.
// Admission/budget enforcement is handled by arbitration to keep refinement/record/decode consistent.
// Current heuristic is intentionally simple and deterministic; later phases can add
// richer scoring (novelty, persistence, profile-aware band priorities, decoder value).
func BuildRefinementPlan(candidates []Candidate, policy Policy) RefinementPlan {
	return BuildRefinementPlanWithBudget(candidates, policy, BudgetModelFromPolicy(policy))
}

func BuildRefinementPlanWithBudget(candidates []Candidate, policy Policy, budgetModel BudgetModel) RefinementPlan {
	strategy, strategyReason := refinementStrategy(policy)
	budget := budgetQueueLimit(budgetModel.Refinement)
	holdPolicy := HoldPolicyFromPolicy(policy)
	plan := RefinementPlan{
		TotalCandidates:   len(candidates),
		MinCandidateSNRDb: policy.MinCandidateSNRDb,
		Budget:            budget,
		BudgetSource:      budgetModel.Refinement.Source,
		Strategy:          strategy,
		StrategyReason:    strategyReason,
	}
	if start, end, ok := monitorBounds(policy); ok {
		plan.MonitorStartHz = start
		plan.MonitorEndHz = end
		if end > start {
			plan.MonitorSpanHz = end - start
		}
	}
	if len(policy.MonitorWindows) > 0 {
		plan.MonitorWindows = append([]MonitorWindow(nil), policy.MonitorWindows...)
		plan.MonitorWindowStats = buildMonitorWindowStats(policy.MonitorWindows)
	}
	if len(candidates) == 0 {
		return plan
	}
	snrWeight, bwWeight, peakWeight := refinementIntentWeights(policy.Intent)
	scoreModel := RefinementScoreModel{
		SNRWeight:       snrWeight,
		BandwidthWeight: bwWeight,
		PeakWeight:      peakWeight,
		EvidenceWeight:  0.6,
	}
	scoreModel = applyStrategyWeights(strategy, scoreModel)
	plan.ScoreModel = scoreModel
	scored := make([]ScheduledCandidate, 0, len(candidates))
	workItems := make([]RefinementWorkItem, 0, len(candidates))
	for _, c := range candidates {
		candidate := c
		RefreshCandidateEvidenceState(&candidate)
		family, familyRank := signalPriorityMatch(policy, candidate.Hint, "")
		familyFloor := signalPriorityTierFloor(familyRank)
		familyRankOut := familyRankForOutput(familyRank)
		inMonitor := ApplyMonitorWindowMatches(policy, &candidate)
		if !inMonitor {
			plan.DroppedByMonitor++
			workItems = append(workItems, RefinementWorkItem{
				Candidate: candidate,
				Status:    RefinementStatusDropped,
				Reason:    RefinementReasonMonitorGate,
				Admission: &PriorityAdmission{
					Tier:       PriorityTierBackground,
					TierFloor:  familyFloor,
					Family:     family,
					FamilyRank: familyRankOut,
					Class:      AdmissionClassDrop,
					Basis:      "refinement",
					Reason:     admissionReason(RefinementReasonMonitorGate, policy, holdPolicy),
				},
			})
			continue
		}
		updateMonitorWindowStats(plan.MonitorWindowStats, candidate.MonitorMatches, monitorStatCandidates)
		if candidate.SNRDb < policy.MinCandidateSNRDb {
			plan.DroppedBySNR++
			updateMonitorWindowStats(plan.MonitorWindowStats, candidate.MonitorMatches, monitorStatDropped)
			workItems = append(workItems, RefinementWorkItem{
				Candidate: candidate,
				Status:    RefinementStatusDropped,
				Reason:    RefinementReasonBelowSNR,
				Admission: &PriorityAdmission{
					Tier:       PriorityTierBackground,
					TierFloor:  familyFloor,
					Family:     family,
					FamilyRank: familyRankOut,
					Class:      AdmissionClassDrop,
					Basis:      "refinement",
					Reason:     admissionReason(RefinementReasonBelowSNR, policy, holdPolicy),
				},
			})
			continue
		}
		snrScore := candidate.SNRDb * scoreModel.SNRWeight
		bwScore := 0.0
		peakScore := 0.0
		policyBoost := CandidatePriorityBoost(policy, candidate.Hint)
		monitorBias, monitorDetail := MonitorWindowBias(policy, candidate)
		if candidate.BandwidthHz > 0 {
			bwScore = minFloat64(candidate.BandwidthHz/25000.0, 6) * scoreModel.BandwidthWeight
		}
		if candidate.PeakDb > 0 {
			peakScore = (candidate.PeakDb / 20.0) * scoreModel.PeakWeight
		}
		rawEvidenceScore, evidenceDetail := candidateEvidenceScore(candidate, strategy)
		evidenceDetail.Weight = scoreModel.EvidenceWeight
		evidenceDetail.RawScore = rawEvidenceScore
		evidenceDetail.WeightedScore = rawEvidenceScore * scoreModel.EvidenceWeight
		evidenceScore := evidenceDetail.WeightedScore
		priority := snrScore + bwScore + peakScore + policyBoost + monitorBias
		priority += evidenceScore
		score := &RefinementScore{
			Total: priority,
			Breakdown: RefinementScoreDetails{
				SNRScore:       snrScore,
				BandwidthScore: bwScore,
				PeakScore:      peakScore,
				PolicyBoost:    policyBoost,
				MonitorBias:    monitorBias,
				MonitorDetail:  monitorDetail,
				EvidenceScore:  evidenceScore,
				EvidenceDetail: &evidenceDetail,
			},
			Weights: &scoreModel,
		}
		scored = append(scored, ScheduledCandidate{
			Candidate:  candidate,
			Priority:   priority,
			TierFloor:  familyFloor,
			Family:     family,
			FamilyRank: familyRankOut,
			Score:      score,
			Breakdown:  &score.Breakdown,
		})
		workItems = append(workItems, RefinementWorkItem{
			Candidate: candidate,
			Priority:  priority,
			Score:     score,
			Breakdown: &score.Breakdown,
			Status:    RefinementStatusPlanned,
			Reason:    RefinementReasonPlanned,
			Admission: &PriorityAdmission{
				Class:      AdmissionClassPlanned,
				TierFloor:  familyFloor,
				Family:     family,
				FamilyRank: familyRankOut,
				Score:      priority,
				Basis:      "refinement",
				Reason:     admissionReason(RefinementReasonPlanned, policy, holdPolicy),
			},
		})
		updateMonitorWindowStats(plan.MonitorWindowStats, candidate.MonitorMatches, monitorStatPlanned)
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Priority == scored[j].Priority {
			return scored[i].Candidate.CenterHz < scored[j].Candidate.CenterHz
		}
		return scored[i].Priority > scored[j].Priority
	})
	if len(scored) > 0 {
		minPriority := scored[0].Priority
		maxPriority := scored[0].Priority
		sumPriority := 0.0
		for _, s := range scored {
			if s.Priority < minPriority {
				minPriority = s.Priority
			}
			if s.Priority > maxPriority {
				maxPriority = s.Priority
			}
			sumPriority += s.Priority
		}
		plan.PriorityMin = minPriority
		plan.PriorityMax = maxPriority
		plan.PriorityAvg = sumPriority / float64(len(scored))
		for i := range scored {
			baseTier := PriorityTierFromRange(scored[i].Priority, minPriority, maxPriority)
			scored[i].Tier = applyTierFloor(baseTier, scored[i].TierFloor)
		}
		for i := range workItems {
			if workItems[i].Admission == nil {
				continue
			}
			if workItems[i].Status != RefinementStatusPlanned {
				continue
			}
			baseTier := PriorityTierFromRange(workItems[i].Priority, minPriority, maxPriority)
			workItems[i].Admission.Tier = applyTierFloor(baseTier, workItems[i].Admission.TierFloor)
		}
	}
	plan.Ranked = append(plan.Ranked, scored...)
	plan.WorkItems = workItems
	return plan
}

func ScheduleCandidates(candidates []Candidate, policy Policy) []ScheduledCandidate {
	plan := BuildRefinementPlan(candidates, policy)
	if len(plan.Ranked) > 0 {
		return plan.Ranked
	}
	return plan.Selected
}

func refinementStrategy(policy Policy) (string, string) {
	intent := strings.ToLower(strings.TrimSpace(policy.Intent))
	profile := strings.ToLower(strings.TrimSpace(policy.Profile))
	switch {
	case strings.Contains(profile, "digital"):
		return "digital-hunting", "profile"
	case strings.Contains(profile, "archive"):
		return "archive-oriented", "profile"
	case strings.Contains(profile, "aggressive"):
		return "multi-resolution", "profile"
	case strings.Contains(intent, "digital") || strings.Contains(intent, "hunt") || strings.Contains(intent, "decode"):
		return "digital-hunting", "intent"
	case strings.Contains(intent, "archive") || strings.Contains(intent, "triage") || strings.Contains(policy.Mode, "archive"):
		return "archive-oriented", "intent"
	case strings.Contains(strings.ToLower(policy.SurveillanceStrategy), "multi"):
		return "multi-resolution", "surveillance-strategy"
	default:
		return "single-resolution", "default"
	}
}

func applyStrategyWeights(strategy string, model RefinementScoreModel) RefinementScoreModel {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "digital-hunting":
		model.SNRWeight *= 1.4
		model.BandwidthWeight *= 0.75
		model.PeakWeight *= 1.2
	case "archive-oriented":
		model.SNRWeight *= 1.1
		model.BandwidthWeight *= 1.6
		model.PeakWeight *= 1.05
	case "multi-resolution", "multi", "multi-res", "multi_res":
		model.SNRWeight *= 1.15
		model.BandwidthWeight *= 1.1
		model.PeakWeight *= 1.15
	case "single-resolution":
		model.SNRWeight *= 1.1
		model.BandwidthWeight *= 1.0
		model.PeakWeight *= 1.0
	}
	return model
}

func candidateEvidenceScore(candidate Candidate, strategy string) (float64, EvidenceScoreDetails) {
	state := CandidateEvidenceStateFor(candidate)
	details := EvidenceScoreDetails{
		DetectionLevels:     state.DetectionLevelCount,
		PrimaryLevels:       state.PrimaryLevelCount,
		DerivedLevels:       state.DerivedLevelCount,
		SupportLevels:       state.SupportLevelCount,
		ProvenanceCount:     len(state.Provenance),
		DerivedOnly:         state.DerivedOnly,
		MultiLevelConfirmed: state.MultiLevelConfirmed,
	}
	score := 0.0
	if state.MultiLevelConfirmed && state.DetectionLevelCount > 1 {
		bonus := 0.85 * float64(state.DetectionLevelCount-1)
		score += bonus
		details.MultiLevelBonus = bonus
	}
	if len(state.Provenance) > 1 {
		bonus := 0.15 * float64(len(state.Provenance)-1)
		score += bonus
		details.ProvenanceBonus = bonus
	}
	if state.DerivedOnly {
		penalty := 0.35
		score -= penalty
		details.DerivedPenalty = -penalty
	}
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "multi-resolution", "multi", "multi-res", "multi_res":
		if state.DerivedOnly {
			bias := 0.2
			score += bias
			details.StrategyBias = bias
		} else if state.MultiLevelConfirmed {
			bias := 0.1
			score += bias
			details.StrategyBias = bias
		}
	case "digital-hunting":
		if state.DerivedOnly {
			bias := -0.15
			score += bias
			details.StrategyBias = bias
		} else if state.MultiLevelConfirmed {
			bias := 0.05
			score += bias
			details.StrategyBias = bias
		}
	case "archive-oriented":
		if state.DerivedOnly {
			bias := -0.1
			score += bias
			details.StrategyBias = bias
		}
	case "single-resolution":
		if state.MultiLevelConfirmed {
			bias := 0.05
			score += bias
			details.StrategyBias = bias
		}
	}
	return score, details
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

type monitorStatUpdate int

const (
	monitorStatCandidates monitorStatUpdate = iota
	monitorStatPlanned
	monitorStatDropped
)

func buildMonitorWindowStats(windows []MonitorWindow) []MonitorWindowStats {
	if len(windows) == 0 {
		return nil
	}
	stats := make([]MonitorWindowStats, 0, len(windows))
	for _, win := range windows {
		stats = append(stats, MonitorWindowStats{
			Index:        win.Index,
			Label:        win.Label,
			Source:       win.Source,
			StartHz:      win.StartHz,
			EndHz:        win.EndHz,
			CenterHz:     win.CenterHz,
			SpanHz:       win.SpanHz,
			Priority:     win.Priority,
			PriorityBias: win.PriorityBias,
		})
	}
	return stats
}

func updateMonitorWindowStats(stats []MonitorWindowStats, matches []MonitorWindowMatch, update monitorStatUpdate) {
	if len(stats) == 0 || len(matches) == 0 {
		return
	}
	index := make(map[int]int, len(stats))
	for i := range stats {
		index[stats[i].Index] = i
	}
	for _, match := range matches {
		i, ok := index[match.Index]
		if !ok {
			continue
		}
		switch update {
		case monitorStatCandidates:
			stats[i].Candidates++
		case monitorStatPlanned:
			stats[i].Planned++
		case monitorStatDropped:
			stats[i].Dropped++
		}
	}
}

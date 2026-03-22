package pipeline

import "strings"

type BudgetRebalance struct {
	Mode         string                     `json:"mode,omitempty"`
	MaxShift     int                        `json:"max_shift,omitempty"`
	Active       bool                       `json:"active,omitempty"`
	Protect      []string                   `json:"protect,omitempty"`
	Favor        []string                   `json:"favor,omitempty"`
	Reasons      []string                   `json:"reasons,omitempty"`
	Adjustments  BudgetRebalanceAdjustments `json:"adjustments,omitempty"`
	favorWeights map[string]float64         `json:"-"`
	protectMap   map[string]bool            `json:"-"`
}

type BudgetRebalanceAdjustments struct {
	Refinement int `json:"refinement,omitempty"`
	Record     int `json:"record,omitempty"`
	Decode     int `json:"decode,omitempty"`
}

type rebalanceQueue struct {
	name     string
	baseMax  int
	max      int
	pressure BudgetPressure
	protect  bool
	favor    float64
}

func ApplyBudgetRebalance(policy Policy, budget BudgetModel, pressure BudgetPressureSummary) BudgetModel {
	state := buildRebalanceState(policy)
	budget.Rebalance = state
	if state.MaxShift <= 0 {
		return budget
	}
	queues := []rebalanceQueue{
		{
			name:     "refinement",
			baseMax:  budget.Refinement.Max,
			max:      budget.Refinement.Max,
			pressure: pressure.Refinement,
			protect:  false,
			favor:    state.favorWeight("refinement"),
		},
		{
			name:     "record",
			baseMax:  budget.Record.Max,
			max:      budget.Record.Max,
			pressure: pressure.Record,
			protect:  state.protects("record"),
			favor:    state.favorWeight("record"),
		},
		{
			name:     "decode",
			baseMax:  budget.Decode.Max,
			max:      budget.Decode.Max,
			pressure: pressure.Decode,
			protect:  state.protects("decode"),
			favor:    state.favorWeight("decode"),
		},
	}

	for i := 0; i < state.MaxShift; i++ {
		recvIdx := pickRebalanceReceiver(queues)
		donorIdx := pickRebalanceDonor(queues)
		if recvIdx < 0 || donorIdx < 0 || recvIdx == donorIdx {
			break
		}
		if queues[donorIdx].max <= 1 {
			break
		}
		queues[donorIdx].max--
		queues[recvIdx].max++
		state.Active = true
	}

	applyRebalanceQueue(&budget.Refinement, queues[0])
	applyRebalanceQueue(&budget.Record, queues[1])
	applyRebalanceQueue(&budget.Decode, queues[2])

	if state.Active {
		state.Adjustments = BudgetRebalanceAdjustments{
			Refinement: budget.Refinement.RebalanceDelta,
			Record:     budget.Record.RebalanceDelta,
			Decode:     budget.Decode.RebalanceDelta,
		}
		budget.Rebalance = state
	}

	return budget
}

func applyRebalanceQueue(queue *BudgetQueue, state rebalanceQueue) {
	if queue == nil {
		return
	}
	delta := state.max - state.baseMax
	queue.RebalanceDelta = delta
	if delta != 0 {
		queue.RebalancedMax = state.max
	} else {
		queue.RebalancedMax = 0
	}
	queue.EffectiveMax = effectiveBudget(budgetQueueLimit(*queue), queue.Preference)
}

func buildRebalanceState(policy Policy) BudgetRebalance {
	state := BudgetRebalance{
		Mode:     "conservative",
		MaxShift: 1,
	}
	profile := strings.ToLower(strings.TrimSpace(policy.Profile))
	intent := strings.ToLower(strings.TrimSpace(policy.Intent))
	strategy := strings.ToLower(strings.TrimSpace(policy.RefinementStrategy))

	protect := map[string]bool{}
	favor := map[string]float64{
		"refinement": 1.0,
		"record":     1.0,
		"decode":     1.0,
	}
	reasons := make([]string, 0, 6)
	addReason := func(tag string) {
		if tag == "" {
			return
		}
		for _, r := range reasons {
			if r == tag {
				return
			}
		}
		reasons = append(reasons, tag)
	}
	legacy := strings.Contains(profile, "legacy")
	if legacy {
		state.MaxShift = 0
		addReason("profile:legacy")
	}
	if strings.Contains(profile, "archive") {
		protect["record"] = true
		favor["record"] += 0.3
		addReason("profile:archive")
		addReason("protect:record")
	}
	if strings.Contains(profile, "digital") {
		protect["decode"] = true
		favor["decode"] += 0.3
		addReason("profile:digital")
		addReason("protect:decode")
	}
	if strings.Contains(profile, "aggressive") {
		favor["refinement"] += 0.35
		if !legacy {
			state.MaxShift = maxInt(state.MaxShift, 2)
		}
		addReason("profile:aggressive")
		addReason("favor:refinement")
	}

	if strings.Contains(intent, "wideband") || strings.Contains(intent, "surveillance") {
		favor["refinement"] += 0.25
		if !legacy {
			state.MaxShift = maxInt(state.MaxShift, 2)
		}
		addReason("intent:wideband")
		addReason("favor:refinement")
	}
	if strings.Contains(intent, "archive") || strings.Contains(intent, "record") {
		protect["record"] = true
		addReason("intent:archive")
		addReason("protect:record")
	}
	if strings.Contains(intent, "decode") || strings.Contains(intent, "digital") || strings.Contains(intent, "hunt") {
		protect["decode"] = true
		addReason("intent:decode")
		addReason("protect:decode")
	}

	if strings.Contains(strategy, "archive") {
		protect["record"] = true
		addReason("strategy:archive")
		addReason("protect:record")
	}
	if strings.Contains(strategy, "digital") {
		protect["decode"] = true
		addReason("strategy:digital")
		addReason("protect:decode")
	}
	if strings.Contains(strategy, "multi") {
		favor["refinement"] += 0.2
		addReason("strategy:multi-resolution")
		addReason("favor:refinement")
	}

	state.Protect = mapKeysSorted(protect)
	state.Favor = favorKeysSorted(favor)
	state.Reasons = reasons
	state.favorWeights = favor
	state.protectMap = protect
	return state
}

func pickRebalanceReceiver(queues []rebalanceQueue) int {
	best := -1
	bestScore := 0.0
	for i := range queues {
		q := &queues[i]
		if q.baseMax <= 0 || q.max <= 0 {
			continue
		}
		if !pressureIsReceiver(q.pressure) {
			continue
		}
		score := pressureScore(q.pressure) * q.favor
		if best == -1 || score > bestScore {
			best = i
			bestScore = score
		}
	}
	return best
}

func pickRebalanceDonor(queues []rebalanceQueue) int {
	best := -1
	bestScore := 0.0
	for i := range queues {
		q := &queues[i]
		if q.baseMax <= 1 || q.max <= 1 {
			continue
		}
		if q.protect {
			continue
		}
		if !pressureIsDonor(q.pressure) {
			continue
		}
		score := pressureScore(q.pressure)
		if best == -1 || score < bestScore {
			best = i
			bestScore = score
		}
	}
	return best
}

func pressureIsReceiver(pressure BudgetPressure) bool {
	if pressure.Pressure >= 1.15 {
		return true
	}
	switch pressure.Level {
	case "high", "critical":
		return true
	default:
		return false
	}
}

func pressureIsDonor(pressure BudgetPressure) bool {
	if pressure.Level == "blocked" {
		return false
	}
	if pressure.Pressure == 0 && pressure.Demand == 0 {
		return true
	}
	if pressure.Pressure > 0 && pressure.Pressure <= 0.85 {
		return true
	}
	switch pressure.Level {
	case "steady", "idle":
		return true
	default:
		return false
	}
}

func pressureScore(pressure BudgetPressure) float64 {
	if pressure.Pressure > 0 {
		return pressure.Pressure
	}
	switch pressure.Level {
	case "critical":
		return 1.6
	case "high":
		return 1.2
	case "elevated":
		return 0.9
	case "steady":
		return 0.6
	case "idle":
		return 0.0
	default:
		return 0.0
	}
}

func mapKeysSorted(values map[string]bool) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for k, ok := range values {
		if ok {
			keys = append(keys, k)
		}
	}
	sortStrings(keys)
	return keys
}

func favorKeysSorted(weights map[string]float64) []string {
	keys := make([]string, 0, len(weights))
	for k, v := range weights {
		if v > 1.01 {
			keys = append(keys, k)
		}
	}
	sortStrings(keys)
	return keys
}

func sortStrings(values []string) {
	if len(values) <= 1 {
		return
	}
	for i := 0; i < len(values)-1; i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

func (r *BudgetRebalance) favorWeight(queue string) float64 {
	if r == nil {
		return 1.0
	}
	if r.favorWeights != nil {
		if v, ok := r.favorWeights[queue]; ok {
			return v
		}
	}
	return 1.0
}

func (r *BudgetRebalance) protects(queue string) bool {
	if r == nil {
		return false
	}
	if r.protectMap != nil {
		if v, ok := r.protectMap[queue]; ok {
			return v
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

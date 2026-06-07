package pipeline

import "math"

type BudgetPressure struct {
	Max        int     `json:"max"`
	Effective  float64 `json:"effective,omitempty"`
	Preference float64 `json:"preference,omitempty"`
	Demand     int     `json:"demand"`
	Queued     int     `json:"queued,omitempty"`
	Selected   int     `json:"selected,omitempty"`
	Active     int     `json:"active,omitempty"`
	Pressure   float64 `json:"pressure,omitempty"`
	Level      string  `json:"level,omitempty"`
}

type BudgetPressureSummary struct {
	Refinement BudgetPressure `json:"refinement"`
	Record     BudgetPressure `json:"record"`
	Decode     BudgetPressure `json:"decode"`
}

func BuildBudgetPressureSummary(budget BudgetModel, admission RefinementAdmission, queue DecisionQueueStats) BudgetPressureSummary {
	return BudgetPressureSummary{
		Refinement: buildRefinementPressure(budget, admission),
		Record:     buildQueuePressure(budget.Record, queue.RecordQueued, queue.RecordSelected, queue.RecordActive),
		Decode:     buildQueuePressure(budget.Decode, queue.DecodeQueued, queue.DecodeSelected, queue.DecodeActive),
	}
}

func buildRefinementPressure(budget BudgetModel, admission RefinementAdmission) BudgetPressure {
	demand := admission.Planned
	selected := admission.Admitted
	return buildPressure(budget.Refinement, demand, 0, selected, 0)
}

func buildQueuePressure(queue BudgetQueue, queued, selected, active int) BudgetPressure {
	demand := queued
	if demand < selected {
		demand = selected
	}
	return buildPressure(queue, demand, queued, selected, active)
}

func buildPressure(queue BudgetQueue, demand int, queued int, selected int, active int) BudgetPressure {
	maxBudget := budgetQueueLimit(queue)
	effective := queue.EffectiveMax
	preference := queue.Preference
	if effective <= 0 && maxBudget > 0 {
		if preference <= 0 {
			preference = 1.0
		}
		effective = float64(maxBudget) * preference
	}
	pressure := 0.0
	level := ""
	switch {
	case demand == 0:
		level = "idle"
	case maxBudget <= 0:
		level = "blocked"
	case effective > 0:
		pressure = float64(demand) / effective
		level = pressureLevel(pressure)
	}
	return BudgetPressure{
		Max:        maxBudget,
		Effective:  roundFloat(pressureEffectiveMax(effective)),
		Preference: preference,
		Demand:     demand,
		Queued:     queued,
		Selected:   selected,
		Active:     active,
		Pressure:   roundFloat(pressure),
		Level:      level,
	}
}

func pressureLevel(pressure float64) string {
	switch {
	case pressure >= 1.5:
		return "critical"
	case pressure >= 1.15:
		return "high"
	case pressure >= 0.85:
		return "elevated"
	default:
		return "steady"
	}
}

func pressureReasonTag(pressure BudgetPressure) string {
	// Return interned constants for the known levels (set by pressureLevel) so the
	// per-candidate-per-frame call does not allocate a fresh "pressure:"+level
	// string every time (#21). Output is identical to the previous concat.
	switch pressure.Level {
	case "", "idle":
		return ""
	case "steady":
		return "pressure:steady"
	case "elevated":
		return "pressure:elevated"
	case "high":
		return "pressure:high"
	case "critical":
		return "pressure:critical"
	default:
		return "pressure:" + pressure.Level
	}
}

func pressureEffectiveMax(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}

func roundFloat(value float64) float64 {
	if value == 0 {
		return 0
	}
	return math.Round(value*100) / 100
}

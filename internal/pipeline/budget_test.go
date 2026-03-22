package pipeline

import "testing"

func TestBudgetModelRefinementSource(t *testing.T) {
	t.Run("uses refinement max concurrent when tighter", func(t *testing.T) {
		policy := Policy{MaxRefinementJobs: 12, RefinementMaxConcurrent: 4}
		budget := BudgetModelFromPolicy(policy)
		if budget.Refinement.Max != 4 {
			t.Fatalf("expected refinement max 4, got %d", budget.Refinement.Max)
		}
		if budget.Refinement.Source != "refinement.max_concurrent" {
			t.Fatalf("expected refinement source refinement.max_concurrent, got %s", budget.Refinement.Source)
		}
	})
	t.Run("keeps resources budget when smaller", func(t *testing.T) {
		policy := Policy{MaxRefinementJobs: 3, RefinementMaxConcurrent: 8}
		budget := BudgetModelFromPolicy(policy)
		if budget.Refinement.Max != 3 {
			t.Fatalf("expected refinement max 3, got %d", budget.Refinement.Max)
		}
		if budget.Refinement.Source != "resources.max_refinement_jobs" {
			t.Fatalf("expected refinement source resources.max_refinement_jobs, got %s", budget.Refinement.Source)
		}
	})
}

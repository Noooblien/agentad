package tests

import (
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/pacing"
)

func TestPacing_RegistrationAndInitialState(t *testing.T) {
	ctrl := pacing.NewController(pacing.DefaultPIDGains())
	ctrl.RegisterCampaign("cmp_1", 100.0, pacing.PacingEven)

	state, ok := ctrl.GetState("cmp_1")
	if !ok {
		t.Fatalf("expected campaign to be registered")
	}
	if state.DailyBudgetUSD != 100.0 {
		t.Fatalf("expected 100.0 budget, got %.2f", state.DailyBudgetUSD)
	}
	if state.TotalSpentUSD != 0.0 {
		t.Fatalf("expected 0 spent, got %.2f", state.TotalSpentUSD)
	}
	if state.RemainingBudgetUSD != 100.0 {
		t.Fatalf("expected 100 remaining, got %.2f", state.RemainingBudgetUSD)
	}
	if state.Status != pacing.StatusNormal {
		t.Fatalf("expected StatusNormal, got %s", state.Status)
	}
}

func TestPacing_EvenPacingTrajectoryAndPID(t *testing.T) {
	ctrl := pacing.NewController(pacing.DefaultPIDGains())
	ctrl.RegisterCampaign("cmp_even", 24.0, pacing.PacingEven) // $1.00 per hour ideal

	// At hour 0, minute 30: target spend = $0.50
	dec := ctrl.EvaluateBid("cmp_even", 0, 30, 0.10)
	if !dec.AllowBid {
		t.Fatalf("expected bid to be allowed under normal pace")
	}

	// Record $0.50 spend at hour 0
	ctrl.RecordSpend("cmp_even", 0.50, 0)
	state, _ := ctrl.GetState("cmp_even")
	if state.TotalSpentUSD != 0.50 {
		t.Fatalf("expected 0.50 spent, got %.2f", state.TotalSpentUSD)
	}

	// At hour 1, minute 0: target spend = $1.00. Current spend = $0.50 -> behind pace -> multiplier increases
	dec2 := ctrl.EvaluateBid("cmp_even", 1, 0, 0.40)
	if !dec2.AllowBid {
		t.Fatalf("expected bid allowed when behind pace")
	}
	if dec2.PacingProbability < 0.50 {
		t.Fatalf("expected probability >= 0.50 when behind pace, got %.2f", dec2.PacingProbability)
	}
}

func TestPacing_AcceleratedGreedyAllocation(t *testing.T) {
	ctrl := pacing.NewController(pacing.DefaultPIDGains())
	ctrl.RegisterCampaign("cmp_acc", 5.0, pacing.PacingAccelerated)

	// Accelerated always allows 100% of bids until budget runs out
	dec := ctrl.EvaluateBid("cmp_acc", 0, 10, 0.99)
	if !dec.AllowBid {
		t.Fatalf("accelerated pacing should allow 100%% of bids")
	}
	if dec.PacingProbability != 1.0 {
		t.Fatalf("expected probability 1.0, got %.2f", dec.PacingProbability)
	}

	// Exhaust budget
	ctrl.RecordSpend("cmp_acc", 5.0, 0)
	decExhausted := ctrl.EvaluateBid("cmp_acc", 0, 15, 0.01)
	if decExhausted.AllowBid {
		t.Fatalf("expected bid rejected when budget is exhausted")
	}
}

func TestPacing_SpendVelocityCircuitBreaker(t *testing.T) {
	ctrl := pacing.NewController(pacing.DefaultPIDGains())
	ctrl.RegisterCampaign("cmp_surge", 24.0, pacing.PacingEven) // Avg $1.00 / hour. 2.0x limit = $2.00 / hour

	// Spend $2.50 in hour 1 (breaches 2.0x velocity limit)
	ctrl.RecordSpend("cmp_surge", 2.50, 1)

	state, _ := ctrl.GetState("cmp_surge")
	if !state.CircuitBreakerTripped {
		t.Fatalf("expected circuit breaker to be tripped after $2.50 spend in 1 hour")
	}
	if state.Status != pacing.StatusCircuitBreaker {
		t.Fatalf("expected StatusCircuitBreaker, got %s", state.Status)
	}

	// Next bid evaluation must be rejected by circuit breaker
	dec := ctrl.EvaluateBid("cmp_surge", 1, 30, 0.01)
	if dec.AllowBid {
		t.Fatalf("circuit breaker must block all bids")
	}
	if !strings.Contains(dec.Reason, "circuit breaker") {
		t.Fatalf("expected circuit breaker in reason, got %s", dec.Reason)
	}

	// Reset circuit breaker
	ctrl.ResetCircuitBreaker("cmp_surge")
	stateReset, _ := ctrl.GetState("cmp_surge")
	if stateReset.CircuitBreakerTripped {
		t.Fatalf("expected circuit breaker cleared after reset")
	}
}

func TestPacing_ManualCircuitBreakerTripAndReset(t *testing.T) {
	ctrl := pacing.NewController(pacing.DefaultPIDGains())
	ctrl.RegisterCampaign("cmp_manual", 50.0, pacing.PacingEven)

	ctrl.TripCircuitBreaker("cmp_manual")
	state, _ := ctrl.GetState("cmp_manual")
	if !state.CircuitBreakerTripped {
		t.Fatalf("expected manual trip")
	}

	dec := ctrl.EvaluateBid("cmp_manual", 5, 0, 0.01)
	if dec.AllowBid {
		t.Fatalf("expected bid denied while tripped")
	}

	ctrl.ResetCircuitBreaker("cmp_manual")
	decAfter := ctrl.EvaluateBid("cmp_manual", 5, 1, 0.01)
	if !decAfter.AllowBid {
		t.Fatalf("expected bid allowed after reset")
	}
}

func TestPacing_SimulationExecutionAndRendering(t *testing.T) {
	results := pacing.RunSimulation()
	if len(results) != 4 {
		t.Fatalf("expected 4 simulation scenarios, got %d", len(results))
	}

	passedCount := 0
	for _, r := range results {
		if r.Passed {
			passedCount++
		}
	}
	if passedCount != 4 {
		t.Fatalf("expected all 4 scenarios to pass governance standards, got %d", passedCount)
	}

	// Test ANSI rendering
	ansi := pacing.RenderSuiteScorecard(results)
	if !strings.Contains(ansi, "DYNAMIC BUDGET PACING") {
		t.Fatalf("missing header in ANSI scorecard")
	}
	if !strings.Contains(ansi, "PASS / OPTIMAL") {
		t.Fatalf("missing PASS / OPTIMAL in ANSI scorecard")
	}

	// Test Markdown rendering
	md := pacing.RenderMarkdown(results)
	if !strings.Contains(md, "Even Pacing (PID Controller)") {
		t.Fatalf("missing scenario name in markdown")
	}
	if !strings.Contains(md, "Pacing RMSE") {
		t.Fatalf("missing Pacing RMSE column in markdown")
	}

	// Test JSON rendering
	jsonStr, err := pacing.RenderJSON(results)
	if err != nil {
		t.Fatalf("failed to render JSON: %v", err)
	}
	if !strings.Contains(jsonStr, "\"pacing_error_rmse\"") {
		t.Fatalf("missing pacing_error_rmse in JSON")
	}
}

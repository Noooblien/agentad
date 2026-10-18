package tests

import (
	"testing"

	"github.com/Noooblien/agentad/pkg/pricing"
)

func TestIntegration_PricingEngine_EndToEnd(t *testing.T) {
	pe := pricing.NewPricingEvaluator(nil, nil)
	scenarios := pricing.GetDemoScenarios()

	if len(scenarios) != 4 {
		t.Fatalf("Expected 4 production demo scenarios, got %d", len(scenarios))
	}

	var totalYieldUSD float64
	var totalBuyerSavingsUSD float64
	clearedCount := 0

	for _, sc := range scenarios {
		pe.FloorOpt.SetPublisherMetrics(sc.PublisherID, sc.Category, sc.FillRate, sc.AvgClearing)
		res := pe.EvaluateAuction(sc.PublisherID, sc.Category, sc.Candidates, sc.ApplyShading)

		if res.Cleared {
			clearedCount++
			totalYieldUSD += res.PublisherYieldUSD
			totalBuyerSavingsUSD += res.BuyerSavingsUSD

			if res.Winner == nil {
				t.Errorf("Scenario '%s' cleared but winner is nil", sc.Name)
			}
			if res.ClearingPriceCPM < res.DynamicFloor.FinalFloorCPM {
				t.Errorf("Scenario '%s': Clearing price $%.3f dropped below floor $%.3f",
					sc.Name, res.ClearingPriceCPM, res.DynamicFloor.FinalFloorCPM)
			}
		}

		// Verify execution speed SLA (sub-5ms)
		if res.ExecutionDurationUs > 50000 {
			t.Errorf("Scenario '%s' took %d µs, exceeding 50ms SLA", sc.Name, res.ExecutionDurationUs)
		}
	}

	if clearedCount < 3 {
		t.Errorf("Expected at least 3/4 scenarios to clear, got %d", clearedCount)
	}

	if totalYieldUSD <= 0 {
		t.Errorf("Expected positive aggregate publisher yield, got $%.6f", totalYieldUSD)
	}

	if totalBuyerSavingsUSD <= 0 {
		t.Errorf("Expected positive aggregate buyer surplus savings, got $%.6f", totalBuyerSavingsUSD)
	}
}

func TestIntegration_DynamicFloor_FillRateAdaptation(t *testing.T) {
	opt := pricing.NewFloorOptimizer(pricing.DefaultDynamicFloorConfig())

	// Low fill rate simulation (e.g. 0.25 fill rate)
	opt.SetPublisherMetrics("pub-soften", "ai-infra", 0.25, 2.50)
	floorLow := opt.CalculateFloor("pub-soften", "ai-infra", 2, 0.85)

	// High fill rate simulation (e.g. 0.95 fill rate)
	opt.SetPublisherMetrics("pub-stiffen", "ai-infra", 0.95, 4.50)
	floorHigh := opt.CalculateFloor("pub-stiffen", "ai-infra", 2, 0.85)

	if floorLow.FinalFloorCPM >= floorHigh.FinalFloorCPM {
		t.Errorf("Floor under low fill ($%.3f) should be strictly lower than under high fill ($%.3f)",
			floorLow.FinalFloorCPM, floorHigh.FinalFloorCPM)
	}

	// Dynamic floor should have adjusted by at least 20%
	diff := (floorHigh.FinalFloorCPM - floorLow.FinalFloorCPM) / floorLow.FinalFloorCPM
	if diff < 0.20 {
		t.Errorf("Expected at least 20%% floor elasticity, got %.1f%%", diff*100)
	}
}

package placement

import (
	"time"
)

// RunPlacementDemo executes 4 enterprise agent scenarios demonstrating placement arbitration.
func RunPlacementDemo() *PlacementSummary {
	cfg := DefaultPlacementConfig()
	engine := NewEngine(cfg)

	// Scenario 1: Clean Exploration Phase Tool Definition (Approved)
	s1 := &SessionState{
		SessionID:          "sess_prod_001_exploratory",
		CurrentTurn:        1,
		TotalContextTokens: 4096,
		RemainingTurnsEst:  4,
		CurrentStage:       StageExploration,
		Impressions:        []ImpressionRecord{},
	}
	c1 := PlacementCandidate{
		AdID:            "ad_pinecone_v2",
		Advertiser:      "Pinecone Systems",
		Category:        "vector_database",
		BidCPM:          4.50,
		FactsheetTokens: 120,
		SupportedSlots:  []SlotArchetype{SlotToolDefinition, SlotToolResponse},
		Factsheet: map[string]interface{}{
			"service_name":        "Pinecone Serverless",
			"read_p95_ms":         14.2,
			"pricing_per_100k":    "$0.08",
			"direct_action_url":   "https://pinecone.io/agent",
		},
	}
	ev1 := engine.Evaluate(s1, c1)

	// Scenario 2: Deep Reasoning Quiet Zone Suppression (Blocked)
	s2 := &SessionState{
		SessionID:          "sess_prod_002_code_refactor",
		CurrentTurn:        8,
		TotalContextTokens: 16384,
		RemainingTurnsEst:  6,
		CurrentStage:       StageReasoning,
		Impressions:        []ImpressionRecord{},
	}
	c2 := PlacementCandidate{
		AdID:            "ad_datadog_apm",
		Advertiser:      "Datadog Inc",
		Category:        "observability",
		BidCPM:          8.00,
		FactsheetTokens: 150,
		SupportedSlots:  []SlotArchetype{SlotToolResponse, SlotScratchpad},
		Factsheet: map[string]interface{}{
			"service_name": "Datadog APM",
			"apm_trace_ms": 2.1,
		},
	}
	ev2 := engine.Evaluate(s2, c2)

	// Scenario 3: Advertiser Turn Cooldown Enforcement (Blocked)
	s3 := &SessionState{
		SessionID:          "sess_prod_003_frequent_user",
		CurrentTurn:        4,
		TotalContextTokens: 6144,
		RemainingTurnsEst:  3,
		CurrentStage:       StageExploration,
		Impressions: []ImpressionRecord{
			{
				Turn:       3,
				AdID:       "ad_pinecone_v2",
				Advertiser: "Pinecone Systems",
				Slot:       SlotToolDefinition,
				TokenCount: 120,
				BidCPM:     4.50,
				Timestamp:  time.Now().Add(-1 * time.Minute),
			},
		},
	}
	c3 := PlacementCandidate{
		AdID:            "ad_pinecone_v2",
		Advertiser:      "Pinecone Systems",
		Category:        "vector_database",
		BidCPM:          5.00,
		FactsheetTokens: 120,
		SupportedSlots:  []SlotArchetype{SlotToolDefinition, SlotToolResponse},
		Factsheet: map[string]interface{}{
			"service_name": "Pinecone Serverless",
		},
	}
	ev3 := engine.Evaluate(s3, c3)

	// Scenario 4: Subeconomic ROI & Bloated Factsheet Overload (Blocked)
	s4 := &SessionState{
		SessionID:          "sess_prod_004_decision_synthesis",
		CurrentTurn:        6,
		TotalContextTokens: 4096,
		RemainingTurnsEst:  3,
		CurrentStage:       StageDecision,
		Impressions:        []ImpressionRecord{},
	}
	c4 := PlacementCandidate{
		AdID:            "ad_bloat_storage",
		Advertiser:      "Legacy Cloud Storage",
		Category:        "object_storage",
		BidCPM:          0.50, // Low bid
		FactsheetTokens: 650,  // Bloated tokens (650 / 4096 = 15.8% > 5%)
		SupportedSlots:  []SlotArchetype{SlotSynthesisEnvelope},
		Factsheet: map[string]interface{}{
			"notes": "Large multi-paragraph unstructured brochure dump...",
		},
	}
	ev4 := engine.Evaluate(s4, c4)

	evals := []*PlacementEvaluation{ev1, ev2, ev3, ev4}
	approved := 0
	suppressed := 0
	var totalFatigue float64

	for _, ev := range evals {
		if ev.Approved {
			approved++
		} else {
			suppressed++
		}
		totalFatigue += ev.CurrentFatigue
	}

	return &PlacementSummary{
		SessionID:       "sess_aap_placement_matrix_suite",
		TotalEvaluated:  len(evals),
		ApprovedCount:   approved,
		SuppressedCount: suppressed,
		AverageFatigue:  totalFatigue / float64(len(evals)),
		Evaluations:     evals,
	}
}

package placement

import (
	"testing"
	"time"
)

func TestPlacement_ExplorationApproved(t *testing.T) {
	cfg := DefaultPlacementConfig()
	engine := NewEngine(cfg)

	session := &SessionState{
		SessionID:          "test_sess_01",
		CurrentTurn:        1,
		TotalContextTokens: 4096,
		RemainingTurnsEst:  3,
		CurrentStage:       StageExploration,
		Impressions:        []ImpressionRecord{},
	}

	candidate := PlacementCandidate{
		AdID:            "ad_pinecone_01",
		Advertiser:      "Pinecone Systems",
		Category:        "vector_database",
		BidCPM:          5.00,
		FactsheetTokens: 100,
		SupportedSlots:  []SlotArchetype{SlotToolDefinition, SlotToolResponse},
		Factsheet: map[string]interface{}{
			"service_name": "Pinecone Serverless",
			"read_p95_ms":  12.5,
		},
	}

	eval := engine.Evaluate(session, candidate)
	if !eval.Approved {
		t.Fatalf("expected placement to be approved, got blocked: %s", eval.Reason)
	}
	if eval.SelectedSlot != SlotToolDefinition {
		t.Fatalf("expected SelectedSlot to be SlotToolDefinition, got %s", eval.SelectedSlot)
	}
	if eval.ROIRatio < 1.25 {
		t.Fatalf("expected ROIRatio >= 1.25, got %.2f", eval.ROIRatio)
	}
	if eval.FormattedSlotPayload == "" {
		t.Fatalf("expected non-empty formatted slot payload")
	}
}

func TestPlacement_ReasoningSuppressed(t *testing.T) {
	cfg := DefaultPlacementConfig()
	engine := NewEngine(cfg)

	session := &SessionState{
		SessionID:          "test_sess_reasoning",
		CurrentTurn:        5,
		TotalContextTokens: 8192,
		RemainingTurnsEst:  5,
		CurrentStage:       StageReasoning,
		Impressions:        []ImpressionRecord{},
	}

	candidate := PlacementCandidate{
		AdID:            "ad_high_bidder",
		Advertiser:      "HighBidder Corp",
		Category:        "developer_tools",
		BidCPM:          15.00,
		FactsheetTokens: 100,
		SupportedSlots:  []SlotArchetype{SlotToolResponse},
	}

	eval := engine.Evaluate(session, candidate)
	if eval.Approved {
		t.Fatalf("expected placement in StageReasoning to be suppressed, but was approved")
	}
	if eval.Decision != DecisionSuppressedStage {
		t.Fatalf("expected DecisionSuppressedStage, got %s", eval.Decision)
	}
}

func TestPlacement_ExecutionSuppressed(t *testing.T) {
	cfg := DefaultPlacementConfig()
	engine := NewEngine(cfg)

	session := &SessionState{
		SessionID:          "test_sess_exec",
		CurrentTurn:        10,
		TotalContextTokens: 8192,
		RemainingTurnsEst:  1,
		CurrentStage:       StageExecution,
		Impressions:        []ImpressionRecord{},
	}

	candidate := PlacementCandidate{
		AdID:            "ad_checkout_upsell",
		Advertiser:      "Upsell Corp",
		BidCPM:          10.00,
		FactsheetTokens: 80,
		SupportedSlots:  []SlotArchetype{SlotSynthesisEnvelope},
	}

	eval := engine.Evaluate(session, candidate)
	if eval.Approved {
		t.Fatalf("expected placement in StageExecution to be suppressed")
	}
	if eval.Decision != DecisionSuppressedStage {
		t.Fatalf("expected DecisionSuppressedStage, got %s", eval.Decision)
	}
}

func TestPlacement_FatigueDecayAndCeiling(t *testing.T) {
	cfg := DefaultPlacementConfig()
	engine := NewEngine(cfg)

	// Session with 2 very recent impressions -> fatigue should saturate
	session := &SessionState{
		SessionID:          "test_fatigue",
		CurrentTurn:        3,
		TotalContextTokens: 4096,
		RemainingTurnsEst:  3,
		CurrentStage:       StageExploration,
		Impressions: []ImpressionRecord{
			{Turn: 1, Advertiser: "Adv A", BidCPM: 3.0, TokenCount: 100, Timestamp: time.Now().Add(-2 * time.Minute)},
			{Turn: 2, Advertiser: "Adv B", BidCPM: 3.0, TokenCount: 100, Timestamp: time.Now().Add(-1 * time.Minute)},
		},
	}

	fatigue := engine.CalculateFatigue(session)
	if fatigue < 0.60 {
		t.Fatalf("expected high fatigue for back-to-back impressions, got %.3f", fatigue)
	}

	// 3 impressions hits the hard session limit
	session.Impressions = append(session.Impressions, ImpressionRecord{
		Turn: 3, Advertiser: "Adv C", BidCPM: 3.0, TokenCount: 100, Timestamp: time.Now(),
	})
	session.CurrentTurn = 4

	candidate := PlacementCandidate{
		AdID:            "ad_fresh",
		Advertiser:      "Adv D",
		BidCPM:          5.00,
		FactsheetTokens: 80,
		SupportedSlots:  []SlotArchetype{SlotToolDefinition},
	}

	eval := engine.Evaluate(session, candidate)
	if eval.Approved {
		t.Fatalf("expected placement to be blocked due to session limit")
	}
	if eval.Decision != DecisionCappedFatigue {
		t.Fatalf("expected DecisionCappedFatigue, got %s", eval.Decision)
	}
}

func TestPlacement_AdvertiserCooldown(t *testing.T) {
	cfg := DefaultPlacementConfig()
	engine := NewEngine(cfg)

	session := &SessionState{
		SessionID:          "test_cooldown",
		CurrentTurn:        4,
		TotalContextTokens: 4096,
		RemainingTurnsEst:  4,
		CurrentStage:       StageExploration,
		Impressions: []ImpressionRecord{
			{Turn: 2, Advertiser: "Pinecone Systems", BidCPM: 4.0, TokenCount: 100, Timestamp: time.Now()},
		},
	}

	// Pinecone bids again at Turn 4 (spacing = 2 turns < AdvertiserCooldownTurns = 5)
	candidate := PlacementCandidate{
		AdID:            "ad_pinecone_v2",
		Advertiser:      "Pinecone Systems",
		BidCPM:          5.00,
		FactsheetTokens: 100,
		SupportedSlots:  []SlotArchetype{SlotToolDefinition},
	}

	eval := engine.Evaluate(session, candidate)
	if eval.Approved {
		t.Fatalf("expected advertiser cooldown to block placement")
	}
	if eval.Decision != DecisionCooldownActive {
		t.Fatalf("expected DecisionCooldownActive, got %s", eval.Decision)
	}
}

func TestPlacement_TokenRatioExcess(t *testing.T) {
	cfg := DefaultPlacementConfig()
	engine := NewEngine(cfg)

	session := &SessionState{
		SessionID:          "test_token_ratio",
		CurrentTurn:        1,
		TotalContextTokens: 2000,
		RemainingTurnsEst:  3,
		CurrentStage:       StageExploration,
		Impressions:        []ImpressionRecord{},
	}

	// 250 tokens in 2000 context = 12.5% (> 5.0% limit)
	candidate := PlacementCandidate{
		AdID:            "ad_bloated",
		Advertiser:      "Bloat Inc",
		BidCPM:          10.00,
		FactsheetTokens: 250,
		SupportedSlots:  []SlotArchetype{SlotToolDefinition},
	}

	eval := engine.Evaluate(session, candidate)
	if eval.Approved {
		t.Fatalf("expected excessive token ratio to be blocked")
	}
	if eval.Decision != DecisionExceededTokenRatio {
		t.Fatalf("expected DecisionExceededTokenRatio, got %s", eval.Decision)
	}
}

func TestPlacement_SubeconomicROI(t *testing.T) {
	cfg := DefaultPlacementConfig()
	engine := NewEngine(cfg)

	session := &SessionState{
		SessionID:          "test_roi",
		CurrentTurn:        1,
		TotalContextTokens: 8192,
		RemainingTurnsEst:  5,
		CurrentStage:       StageExploration,
		Impressions:        []ImpressionRecord{},
	}

	// Low bid ($0.20 CPM) with 150 tokens over 5 turns -> token cost = (150/1000) * 0.003 * 5 = $0.00225
	// Revenue = $0.20 / 1000 = $0.0002. ROI = 0.0002 / 0.00225 = 0.088x (< 1.25x)
	candidate := PlacementCandidate{
		AdID:            "ad_penny_bidder",
		Advertiser:      "Penny Bidder",
		BidCPM:          0.20,
		FactsheetTokens: 150,
		SupportedSlots:  []SlotArchetype{SlotToolDefinition},
	}

	eval := engine.Evaluate(session, candidate)
	if eval.Approved {
		t.Fatalf("expected subeconomic ROI to be blocked")
	}
	if eval.Decision != DecisionSubeconomicROI {
		t.Fatalf("expected DecisionSubeconomicROI, got %s", eval.Decision)
	}
}

func TestPlacement_FormatPayloads(t *testing.T) {
	engine := NewEngine(DefaultPlacementConfig())
	candidate := PlacementCandidate{
		AdID:       "ad_format_test",
		Advertiser: "Format Corp",
		Category:   "cloud",
		Factsheet: map[string]interface{}{
			"tier": "enterprise",
		},
	}

	slots := []SlotArchetype{SlotToolDefinition, SlotToolResponse, SlotScratchpad, SlotSynthesisEnvelope}
	for _, s := range slots {
		payload := engine.FormatSlotPayload(s, candidate)
		if len(payload) == 0 {
			t.Fatalf("expected non-empty payload for slot %s", s)
		}
	}
}

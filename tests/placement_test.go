package tests

import (
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/placement"
)

func TestPlacement_EndToEndDemoSuite(t *testing.T) {
	summary := placement.RunPlacementDemo()

	if summary.TotalEvaluated != 4 {
		t.Fatalf("expected 4 evaluated scenarios, got %d", summary.TotalEvaluated)
	}

	if summary.ApprovedCount != 1 {
		t.Errorf("expected 1 approved candidate, got %d", summary.ApprovedCount)
	}
	if summary.SuppressedCount != 3 {
		t.Errorf("expected 3 suppressed candidates, got %d", summary.SuppressedCount)
	}

	terminal := placement.RenderPlacementScorecard(summary)
	if !strings.Contains(terminal, "AGENT SLOT PLACEMENT, FREQUENCY CAPPING & ATTENTION BUDGET ARBITER") {
		t.Errorf("expected scorecard header in terminal output")
	}

	mermaid := placement.RenderPlacementMermaid(summary)
	if !strings.Contains(mermaid, "flowchart TD") {
		t.Errorf("expected flowchart TD in mermaid output")
	}

	md := placement.RenderPlacementMarkdown(summary)
	if !strings.Contains(md, "AgentAd (AAP) Placement Arbitration Report") {
		t.Errorf("expected markdown report heading")
	}
}

func TestPlacement_DecisionStageSynthesisSlot(t *testing.T) {
	engine := placement.NewEngine(placement.DefaultPlacementConfig())

	session := &placement.SessionState{
		SessionID:          "test_sess_decision",
		CurrentTurn:        4,
		TotalContextTokens: 6144,
		RemainingTurnsEst:  2,
		CurrentStage:       placement.StageDecision,
		Impressions:        []placement.ImpressionRecord{},
	}

	candidate := placement.PlacementCandidate{
		AdID:            "ad_cloud_db",
		Advertiser:      "CloudDB Global",
		Category:        "database",
		BidCPM:          6.00,
		FactsheetTokens: 120,
		SupportedSlots:  []placement.SlotArchetype{placement.SlotSynthesisEnvelope, placement.SlotToolResponse},
		Factsheet: map[string]interface{}{
			"service_name": "CloudDB Serverless",
			"latency_p95":  10.0,
		},
	}

	eval := engine.Evaluate(session, candidate)
	if !eval.Approved {
		t.Fatalf("expected decision-stage candidate to be approved, got: %s", eval.Reason)
	}
	if eval.SelectedSlot != placement.SlotSynthesisEnvelope {
		t.Fatalf("expected SlotSynthesisEnvelope, got %s", eval.SelectedSlot)
	}
	if !strings.Contains(eval.FormattedSlotPayload, "Verified Alternative") {
		t.Errorf("expected Verified Alternative in payload, got %s", eval.FormattedSlotPayload)
	}
}

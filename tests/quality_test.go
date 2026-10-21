package tests

import (
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/quality"
)

func TestQuality_EndToEndDemoSuite(t *testing.T) {
	summary := quality.RunQualityDemo()

	if summary.TotalEvaluated != 4 {
		t.Fatalf("expected 4 evaluated candidates, got %d", summary.TotalEvaluated)
	}

	if summary.EligibleCount != 3 {
		t.Errorf("expected 3 eligible candidates, got %d", summary.EligibleCount)
	}
	if summary.DisqualifiedCount != 1 {
		t.Errorf("expected 1 disqualified candidate, got %d", summary.DisqualifiedCount)
	}

	terminal := quality.RenderTerminalScorecard(summary)
	if !strings.Contains(terminal, "AGENT AD QUALITY SCORE") {
		t.Errorf("expected scorecard header in terminal output")
	}

	mermaid := quality.RenderQualityMermaid(summary)
	if !strings.Contains(mermaid, "flowchart TD") {
		t.Errorf("expected flowchart TD in mermaid output")
	}

	md := quality.RenderQualityMarkdown(summary)
	if !strings.Contains(md, "Agent Ad Quality Score (AQS) & Factsheet Grounding Audit") {
		t.Errorf("expected markdown report heading")
	}
}

func TestQuality_AuctionClearingAdjustment(t *testing.T) {
	engine := quality.NewEngine(quality.DefaultQualityConfig())

	// Top tier candidate should earn clearing discount
	topCandidate := quality.QualityCandidate{
		AdID:            "ad_top",
		Advertiser:      "TopProvider",
		Category:        "ai-infra",
		BidCPM:          4.00,
		Keywords:        []string{"gpu", "cluster", "h100"},
		HistoricalCTR:   0.09,
		ToolCallSuccess: 0.99,
		Factsheet: map[string]interface{}{
			"service_name":   "TopProvider GPU Clusters",
			"api_endpoint":   "https://api.topprovider.com/v1",
			"authentication": "Bearer token",
			"pricing":        "$2.50 per GPU-hr",
		},
	}

	eval := engine.EvaluateCandidate("rent h100 gpu cluster", topCandidate)
	if !eval.Eligible {
		t.Fatalf("expected eligible candidate, got flags: %v", eval.FlaggedIssues)
	}
	if eval.Tier != quality.TierExcellent && eval.Tier != quality.TierGood {
		t.Errorf("expected Excellent or Good tier, got %s", eval.Tier)
	}
	if eval.EffectiveCostCPM >= eval.RawBidCPM {
		t.Errorf("expected EffectiveCostCPM ($%.2f) < RawBidCPM ($%.2f) due to quality discount",
			eval.EffectiveCostCPM, eval.RawBidCPM)
	}
}

package pricing

import (
	"strings"
	"testing"
)

func TestDynamicFloorOptimizer(t *testing.T) {
	cfg := DefaultDynamicFloorConfig()
	opt := NewFloorOptimizer(cfg)

	// 1. Baseline category test
	res := opt.CalculateFloor("pub-1", "ai-infra", 2, 0.85)
	if res.BaseFloorCPM != 3.50 {
		t.Errorf("Expected base floor 3.50, got %.2f", res.BaseFloorCPM)
	}
	if res.FinalFloorCPM <= 0 {
		t.Errorf("Expected positive floor CPM, got %.2f", res.FinalFloorCPM)
	}

	// 2. High fill rate test (inventory in high demand -> higher floor)
	opt.SetPublisherMetrics("pub-high-fill", "ai-infra", 0.95, 4.00)
	resHigh := opt.CalculateFloor("pub-high-fill", "ai-infra", 2, 0.85)

	// 3. Low fill rate test (inventory undersold -> lower floor to capture clearance)
	opt.SetPublisherMetrics("pub-low-fill", "ai-infra", 0.30, 2.00)
	resLow := opt.CalculateFloor("pub-low-fill", "ai-infra", 2, 0.85)

	if resHigh.FinalFloorCPM <= resLow.FinalFloorCPM {
		t.Errorf("High fill floor ($%.2f) must exceed low fill floor ($%.2f)",
			resHigh.FinalFloorCPM, resLow.FinalFloorCPM)
	}

	// 4. High competition boost (e.g. 8 bidders vs 2)
	resComp := opt.CalculateFloor("pub-1", "ai-infra", 8, 0.85)
	if resComp.CompetitionFactor <= 1.0 {
		t.Errorf("Expected competition factor > 1.0 for 8 bidders, got %.2f", resComp.CompetitionFactor)
	}

	// 5. Hard guardrails clamping
	opt.Config.MaxFloorCPM = 4.00
	resClamped := opt.CalculateFloor("pub-high-fill", "ai-infra", 10, 0.50)
	if resClamped.FinalFloorCPM > 4.00 {
		t.Errorf("Expected floor clamped to max 4.00, got %.2f", resClamped.FinalFloorCPM)
	}
	if !resClamped.IsClamped {
		t.Errorf("Expected IsClamped to be true")
	}

	// 6. Record auction outcome updates state
	opt.RecordAuctionOutcome("pub-test", "cloud-compute", true, 3.20)
	st := opt.GetPublisherState("pub-test")
	if st == nil || st.TotalAuctions != 1 || st.FilledAuctions != 1 {
		t.Fatalf("Expected recorded state with 1 filled auction")
	}
}

func TestBidShader(t *testing.T) {
	cfg := DefaultBidShadingConfig()
	shader := NewBidShader(cfg)

	// 1. Floor-aware hybrid shading
	res := shader.ShadeBid(5.00, "ai-infra", 2.00, 4)
	if res.ShadedBidCPM >= res.OriginalBidCPM {
		t.Errorf("Expected shaded bid to be less than original bid 5.00, got %.2f", res.ShadedBidCPM)
	}
	if res.ShadedBidCPM < (res.EstimatedFloorCPM + cfg.ReserveSafetyMarginCPM) {
		t.Errorf("Shaded bid %.2f dropped below estimated floor + margin (2.10)", res.ShadedBidCPM)
	}
	if res.SurplusSavedCPM <= 0 {
		t.Errorf("Expected positive surplus saved, got %.2f", res.SurplusSavedCPM)
	}

	// 2. Vickrey emulator algorithm
	shader.Config.Algorithm = AlgorithmVickreyEmulator
	resVickrey := shader.ShadeBid(10.00, "cloud-compute", 1.00, 2)
	if resVickrey.ShadingRatio > 0.80 {
		t.Errorf("Expected significant shading ratio for 2 bidders in Vickrey emulator, got %.2f", resVickrey.ShadingRatio)
	}

	// 3. Disabled shading
	shader.Config.Enabled = false
	resDisabled := shader.ShadeBid(10.00, "cloud-compute", 1.00, 5)
	if resDisabled.ShadedBidCPM != 10.00 || resDisabled.ShadingRatio != 1.0 {
		t.Errorf("Expected disabled shader to preserve raw bid 10.00, got %.2f", resDisabled.ShadedBidCPM)
	}
}

func TestPricingEvaluator(t *testing.T) {
	pe := NewPricingEvaluator(nil, nil)

	candidates := []CandidateBid{
		{
			CampaignID:     "cmp_1",
			AdvertiserName: "Alpha Vector",
			AgentID:        "ag_alpha",
			RawBidCPM:      5.50,
			RelevanceScore: 0.92,
			TrustScore:     0.95,
		},
		{
			CampaignID:     "cmp_2",
			AdvertiserName: "Beta Cache",
			AgentID:        "ag_beta",
			RawBidCPM:      4.20,
			RelevanceScore: 0.85,
			TrustScore:     0.90,
		},
		{
			CampaignID:     "cmp_low_trust",
			AdvertiserName: "Rogue Spammer",
			AgentID:        "ag_rogue",
			RawBidCPM:      9.00,
			RelevanceScore: 0.80,
			TrustScore:     0.15, // Below 0.30 trust threshold
		},
		{
			CampaignID:     "cmp_low_rel",
			AdvertiserName: "Irrelevant Ad",
			AgentID:        "ag_irr",
			RawBidCPM:      3.50,
			RelevanceScore: 0.20, // Below 0.35 relevance threshold
			TrustScore:     0.88,
		},
	}

	res := pe.EvaluateAuction("pub-main", "ai-infra", candidates, true)

	if !res.Cleared {
		t.Fatalf("Expected auction to clear")
	}
	if res.Winner == nil {
		t.Fatalf("Expected non-nil winner")
	}
	if res.Winner.CampaignID != "cmp_1" {
		t.Errorf("Expected cmp_1 to win, got %s", res.Winner.CampaignID)
	}

	// Verify rejected candidates
	if len(res.RejectedCandidates) < 2 {
		t.Errorf("Expected at least 2 rejections (low trust & low rel), got %d", len(res.RejectedCandidates))
	}

	hasLowTrust := false
	hasLowRel := false
	for _, rej := range res.RejectedCandidates {
		if rej.Reason == RejectLowTrust {
			hasLowTrust = true
		}
		if rej.Reason == RejectLowRelevance {
			hasLowRel = true
		}
	}
	if !hasLowTrust || !hasLowRel {
		t.Errorf("Expected low trust and low relevance rejection codes to be present")
	}

	// Verify clearing price is at or above dynamic floor
	if res.ClearingPriceCPM < res.DynamicFloor.FinalFloorCPM {
		t.Errorf("Clearing price $%.2f must be >= dynamic floor $%.2f",
			res.ClearingPriceCPM, res.DynamicFloor.FinalFloorCPM)
	}
}

func TestScorecardAndReports(t *testing.T) {
	pe := NewPricingEvaluator(nil, nil)
	scenarios := GetDemoScenarios()

	var results []*AuctionEvaluationResult
	for _, sc := range scenarios {
		pe.FloorOpt.SetPublisherMetrics(sc.PublisherID, sc.Category, sc.FillRate, sc.AvgClearing)
		res := pe.EvaluateAuction(sc.PublisherID, sc.Category, sc.Candidates, sc.ApplyShading)
		results = append(results, res)

		scorecard := RenderPricingScorecard(res)
		if !strings.Contains(scorecard, "DYNAMIC PUBLISHER FLOOR") {
			t.Errorf("Scorecard missing expected header")
		}

		mermaid := GenerateMermaidChart(res)
		if !strings.Contains(mermaid, "flowchart TD") {
			t.Errorf("Mermaid chart missing expected flowchart definition")
		}
	}

	summary := RenderSimulationSummary(results)
	if !strings.Contains(summary, "SIMULATION BENCHMARK SUMMARY") {
		t.Errorf("Simulation summary missing expected header")
	}

	markdown := GenerateMarkdownReport(results)
	if !strings.Contains(markdown, "Executive Summary") {
		t.Errorf("Markdown report missing expected header")
	}
}

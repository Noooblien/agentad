package quality

import (
	"strings"
	"testing"
)

func TestEvaluateCandidate_ExcellentFactsheet(t *testing.T) {
	engine := NewEngine(DefaultQualityConfig())
	query := "Need serverless vector database for RAG retrieval"

	candidate := QualityCandidate{
		AdID:            "ad_pinecone",
		Advertiser:      "Pinecone",
		Category:        "ai-infra",
		BidCPM:          3.50,
		Keywords:        []string{"vector", "database", "serverless", "rag"},
		HistoricalCTR:   0.09,
		ToolCallSuccess: 0.98,
		Factsheet: map[string]interface{}{
			"service_name":   "Pinecone Serverless",
			"api_endpoint":   "https://api.pinecone.io/v1/vectors",
			"authentication": "Bearer token",
			"pricing":        "$0.008 per 100k RUs",
			"latency_sla":    "12ms",
		},
	}

	eval := engine.EvaluateCandidate(query, candidate)

	if !eval.Eligible {
		t.Fatalf("Expected eligible, got ineligible with flags: %v", eval.FlaggedIssues)
	}
	if eval.QualityScore < 8.5 {
		t.Errorf("Expected AQS >= 8.5, got %.1f", eval.QualityScore)
	}
	if eval.ClearingMultiplier > 0.90 {
		t.Errorf("Expected clearing multiplier <= 0.90 (discount), got %.2f", eval.ClearingMultiplier)
	}
	if eval.EffectiveAdRank <= 0 {
		t.Errorf("Expected positive effective AdRank, got %.2f", eval.EffectiveAdRank)
	}
}

func TestEvaluateCandidate_HallucinationAndHype(t *testing.T) {
	engine := NewEngine(DefaultQualityConfig())
	query := "Need vector database"

	candidate := QualityCandidate{
		AdID:            "ad_shady",
		Advertiser:      "ShadyAI",
		Category:        "ai-infra",
		BidCPM:          10.00,
		Keywords:        []string{"vector", "database"},
		HistoricalCTR:   0.001,
		ToolCallSuccess: 0.10,
		Factsheet: map[string]interface{}{
			"service_name": "Magic 100% Free Forever",
			"api_endpoint": "not-a-valid-url",
			"pricing":      "unlimited free forever with guaranteed 1000x profit",
			"latency_sla":  "0ms zero latency infinite scale",
			"hype":         "best in the world revolutionary magic ai",
		},
	}

	eval := engine.EvaluateCandidate(query, candidate)

	if eval.Eligible {
		t.Fatalf("Expected candidate to be disqualified, but was marked eligible with AQS: %.1f", eval.QualityScore)
	}
	if eval.Tier != TierDisqualified {
		t.Errorf("Expected TierDisqualified, got %s", eval.Tier)
	}
	if eval.EffectiveAdRank != 0.0 {
		t.Errorf("Expected EffectiveAdRank 0.0 for disqualified ad, got %.2f", eval.EffectiveAdRank)
	}
	if len(eval.FlaggedIssues) == 0 {
		t.Errorf("Expected multiple flagged issues for hype and invalid endpoint")
	}
}

func TestEvaluateCandidate_MissingEndpointAndAuth(t *testing.T) {
	engine := NewEngine(DefaultQualityConfig())
	query := "Need analytics tool"

	candidate := QualityCandidate{
		AdID:       "ad_incomplete",
		Advertiser: "IncompleteInc",
		Category:   "analytics",
		BidCPM:     2.00,
		Keywords:   []string{"analytics"},
		Factsheet: map[string]interface{}{
			"service_name": "Simple Analytics",
		},
	}

	eval := engine.EvaluateCandidate(query, candidate)
	if eval.GroundingScore > 0.60 {
		t.Errorf("Expected low grounding score due to missing endpoint/auth, got %.2f", eval.GroundingScore)
	}
}

func TestRelevanceScoring_HighVsLow(t *testing.T) {
	engine := NewEngine(DefaultQualityConfig())

	highCandidate := QualityCandidate{
		Category: "ai-infra",
		Keywords: []string{"vector", "embeddings", "rag", "database"},
	}
	lowCandidate := QualityCandidate{
		Category: "furniture",
		Keywords: []string{"wooden", "chair", "table", "office"},
	}

	query := "Need vector database for rag embeddings"
	highRel := engine.computeRelevance(query, highCandidate)
	lowRel := engine.computeRelevance(query, lowCandidate)

	if highRel <= lowRel {
		t.Errorf("Expected high relevance (%.2f) > low relevance (%.2f)", highRel, lowRel)
	}
}

func TestAdoptionScoring_PriorVsHistory(t *testing.T) {
	engine := NewEngine(DefaultQualityConfig())

	coldStart := QualityCandidate{}
	scoreCold := engine.computeAdoptionScore(coldStart)
	if scoreCold != 0.50 {
		t.Errorf("Expected neutral 0.50 prior for cold-start candidate, got %.2f", scoreCold)
	}

	provenCandidate := QualityCandidate{
		HistoricalCTR:   0.09,
		ToolCallSuccess: 0.99,
	}
	scoreProven := engine.computeAdoptionScore(provenCandidate)
	if scoreProven <= 0.80 {
		t.Errorf("Expected proven candidate score > 0.80, got %.2f", scoreProven)
	}
}

func TestQualityArbitrage_RankSurpassesHigherBid(t *testing.T) {
	engine := NewEngine(DefaultQualityConfig())
	query := "Need high-performance serverless vector database"

	highQualityLowBid := QualityCandidate{
		AdID:            "ad_quality",
		Advertiser:      "QualityCloud",
		Category:        "ai-infra",
		BidCPM:          3.50,
		Keywords:        []string{"vector", "database", "serverless", "high-performance"},
		HistoricalCTR:   0.09,
		ToolCallSuccess: 0.95,
		Factsheet: map[string]interface{}{
			"service_name":   "QualityCloud Vector DB",
			"api_endpoint":   "https://api.qualitycloud.io",
			"authentication": "Bearer token",
			"pricing":        "$0.01 per 10k requests",
		},
	}

	lowQualityHighBid := QualityCandidate{
		AdID:            "ad_high_bid",
		Advertiser:      "SpamBigBid",
		Category:        "general",
		BidCPM:          5.00, // Higher bid
		Keywords:        []string{"cheap", "hosting"},
		HistoricalCTR:   0.01,
		ToolCallSuccess: 0.50,
		Factsheet: map[string]interface{}{
			"service_name":   "Spam Hosting",
			"api_endpoint":   "https://spam.example.com",
			"authentication": "api_key",
			"pricing":        "contact us",
		},
	}

	evalQuality := engine.EvaluateCandidate(query, highQualityLowBid)
	evalSpam := engine.EvaluateCandidate(query, lowQualityHighBid)

	if evalQuality.EffectiveAdRank <= evalSpam.EffectiveAdRank {
		t.Fatalf("Quality arbitrage failed: Quality ad rank (%.2f) should surpass high-bid spam rank (%.2f)",
			evalQuality.EffectiveAdRank, evalSpam.EffectiveAdRank)
	}
}

func TestEvaluateBatch_Sorting(t *testing.T) {
	summary := RunQualityDemo()

	if summary.TotalEvaluated != 4 {
		t.Fatalf("Expected 4 evaluated candidates, got %d", summary.TotalEvaluated)
	}
	if summary.EligibleCount != 3 {
		t.Errorf("Expected 3 eligible candidates, got %d", summary.EligibleCount)
	}
	if summary.DisqualifiedCount != 1 {
		t.Errorf("Expected 1 disqualified candidate, got %d", summary.DisqualifiedCount)
	}

	// Verify sorting descending by EffectiveAdRank
	for i := 1; i < len(summary.Evaluations); i++ {
		if summary.Evaluations[i-1].EffectiveAdRank < summary.Evaluations[i].EffectiveAdRank {
			t.Errorf("Evaluations not sorted descending: [%d]=%.2f < [%d]=%.2f",
				i-1, summary.Evaluations[i-1].EffectiveAdRank, i, summary.Evaluations[i].EffectiveAdRank)
		}
	}
}

func TestScorecardAndMermaid(t *testing.T) {
	summary := RunQualityDemo()

	terminalOut := RenderTerminalScorecard(summary)
	if !strings.Contains(terminalOut, "AGENT AD QUALITY SCORE") {
		t.Errorf("Terminal output missing header")
	}

	mermaidOut := RenderQualityMermaid(summary)
	if !strings.Contains(mermaidOut, "flowchart TD") {
		t.Errorf("Mermaid output missing flowchart directive")
	}

	markdownOut := RenderQualityMarkdown(summary)
	if !strings.Contains(markdownOut, "| Advertiser |") {
		t.Errorf("Markdown output missing table headers")
	}
}

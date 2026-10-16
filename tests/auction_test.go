package tests

import (
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/auction"
	"github.com/Noooblien/agentad/pkg/spec"
)

func TestAuction_Sub50msExecution(t *testing.T) {
	engine := auction.NewEngine(auction.DefaultAuctionConfig())
	engine.SeedDemoCampaigns()

	res, err := engine.RunAuction("find the fastest serverless vector database for rag embeddings", "")
	if err != nil {
		t.Fatalf("RunAuction failed: %v", err)
	}

	if res.ExecutionTimeMs > 50.0 {
		t.Errorf("Execution latency exceeded 50ms SLA: took %.2f ms", res.ExecutionTimeMs)
	}

	if !res.AuctionCleared {
		t.Errorf("Expected auction to clear for relevant vector query")
	}

	if res.WinningCampaign == nil || !strings.Contains(res.WinningCampaign.AdvertiserName, "Pinecone") {
		t.Errorf("Expected Pinecone to win vector database auction")
	}

	if res.ClearingPriceCPM <= 0 {
		t.Errorf("Expected positive clearing price, got %.2f", res.ClearingPriceCPM)
	}
}

func TestAuction_SecondPriceClearingLogic(t *testing.T) {
	engine := auction.NewEngine(auction.DefaultAuctionConfig())

	// Bidder A: High Bid ($10.00), Relevance 1.0, Trust 1.0 -> Score = 10.0
	_ = engine.RegisterCampaign(&auction.Campaign{
		CampaignID:     "cmp_a",
		AdvertiserName: "High Bidder",
		Category:       "cloud",
		TargetKeywords: []string{"kubernetes", "cloud"},
		MaxBidCPM:      10.00,
		TotalBudgetUSD: 100.0,
		TrustScore:     1.0,
		Factsheet: spec.Factsheet{
			ID:              "f_a",
			ProductName:     "Cloud A",
			DirectActionURL: "https://a.com",
		},
	})

	// Bidder B: Lower Bid ($4.00), Relevance 1.0, Trust 1.0 -> Score = 4.0
	_ = engine.RegisterCampaign(&auction.Campaign{
		CampaignID:     "cmp_b",
		AdvertiserName: "Medium Bidder",
		Category:       "cloud",
		TargetKeywords: []string{"kubernetes", "cloud"},
		MaxBidCPM:      4.00,
		TotalBudgetUSD: 100.0,
		TrustScore:     1.0,
		Factsheet: spec.Factsheet{
			ID:              "f_b",
			ProductName:     "Cloud B",
			DirectActionURL: "https://b.com",
		},
	})

	res, err := engine.RunAuction("kubernetes cloud deployment", "cloud")
	if err != nil {
		t.Fatalf("RunAuction failed: %v", err)
	}

	if !res.AuctionCleared || res.WinningCampaign.CampaignID != "cmp_a" {
		t.Fatalf("Expected Bidder A to win, got %+v", res.WinningCampaign)
	}

	// In second price auction with identical relevance/trust:
	// Clearing price should be around 4.00 + 0.01 = 4.01 CPM, well below Bidder A's $10.00 max bid!
	if res.ClearingPriceCPM > 5.00 || res.ClearingPriceCPM < 4.00 {
		t.Errorf("Expected second-price clearing between 4.00 and 5.00 CPM, got %.2f", res.ClearingPriceCPM)
	}
}

func TestAuction_BudgetDepletion(t *testing.T) {
	engine := auction.NewEngine(auction.DefaultAuctionConfig())

	_ = engine.RegisterCampaign(&auction.Campaign{
		CampaignID:     "cmp_tiny_budget",
		AdvertiserName: "Tiny Budget",
		TargetKeywords: []string{"serverless"},
		MaxBidCPM:      5.00,
		TotalBudgetUSD: 0.001, // Exactly 1 impression at reserve price $1.00 CPM ($0.001)
		TrustScore:     1.0,
		Factsheet: spec.Factsheet{
			ID:              "f_tiny",
			ProductName:     "Tiny",
			DirectActionURL: "https://tiny.com",
		},
	})

	// First run should clear
	res1, err := engine.RunAuction("serverless compute", "")
	if err != nil || !res1.AuctionCleared {
		t.Fatalf("First auction should clear: %v", err)
	}

	// Second run should fail to clear because budget is exhausted
	res2, err := engine.RunAuction("serverless compute", "")
	if err != nil {
		t.Fatalf("Second auction errored: %v", err)
	}

	if res2.AuctionCleared {
		t.Errorf("Expected auction to not clear after budget depletion")
	}
}

package bidder

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
	"github.com/Noooblien/agentad/pkg/placement"
)

func TestBidderAgent_Registration(t *testing.T) {
	agent := NewBidderAgent("adv_test", "Test Advertiser", nil)

	err := agent.RegisterCampaign(CampaignConfig{
		CampaignID:     "cmp_test_01",
		AdvertiserID:   "adv_test",
		AdvertiserName: "Test Advertiser",
		MaxBidCPM:      4.50,
	})
	if err != nil {
		t.Fatalf("Failed registering valid campaign: %v", err)
	}

	// Invalid campaign without ID
	errInvalid := agent.RegisterCampaign(CampaignConfig{
		MaxBidCPM: 4.50,
	})
	if errInvalid == nil {
		t.Fatalf("Expected error for campaign with empty ID")
	}
}

func TestBidderAgent_BudgetExhaustion(t *testing.T) {
	agent := NewBidderAgent("adv_test", "Test Advertiser", nil)

	_ = agent.RegisterCampaign(CampaignConfig{
		CampaignID:     "cmp_exhausted",
		AdvertiserID:   "adv_test",
		AdvertiserName: "Test Advertiser",
		Category:       "vector_database",
		MaxBidCPM:      5.00,
		DailyBudgetUSD: 10.00,
		TotalSpentUSD:  10.00, // Budget 100% spent
	})

	req := BidRequest{
		RequestID:       "req_test_01",
		ScrubbedIntent:  "need vector database",
		Category:        "vector_database",
		SlotType:        placement.SlotToolResponse,
		ReserveFloorCPM: 1.00,
		Timestamp:       time.Now(),
	}

	resp := agent.EvaluateBidRequest(req)
	if resp.Submitted {
		t.Fatalf("Expected no-bid for exhausted campaign, but bid was submitted")
	}
	if !strings.Contains(resp.NoBidReason, "budget") {
		t.Errorf("Expected reason to mention budget, got: %s", resp.NoBidReason)
	}
}

func TestBidderAgent_NegativeKeywords(t *testing.T) {
	agent := NewBidderAgent("adv_test", "Test Advertiser", nil)

	_ = agent.RegisterCampaign(CampaignConfig{
		CampaignID:       "cmp_negative_test",
		AdvertiserID:     "adv_test",
		AdvertiserName:   "Enterprise Brand",
		Category:         "vector_database",
		NegativeKeywords: []string{"cracked", "freeware", "pirated"},
		MaxBidCPM:        5.00,
	})

	req := BidRequest{
		RequestID:       "req_test_02",
		ScrubbedIntent:  "download cracked pirated vector database",
		Category:        "vector_database",
		SlotType:        placement.SlotToolResponse,
		ReserveFloorCPM: 1.00,
		Timestamp:       time.Now(),
	}

	resp := agent.EvaluateBidRequest(req)
	if resp.Submitted {
		t.Fatalf("Expected no-bid on query containing negative keywords")
	}
}

func TestBidderAgent_CryptographicSignature(t *testing.T) {
	key, _ := attest.GenerateKeyPair()
	agent := NewBidderAgent("adv_crypto", "Crypto Advertiser", key)

	_ = agent.RegisterCampaign(CampaignConfig{
		CampaignID:       "cmp_crypto_01",
		AdvertiserID:     "adv_crypto",
		AdvertiserName:   "Crypto Advertiser",
		Category:         "vector_database",
		TargetKeywords:   []string{"vector", "database"},
		MaxBidCPM:        4.50,
		MinBidCPM:        1.00,
		Strategy:         StrategyExpectedValue,
		ShadingDiscount:  0.10,
		SigningKey:       key,
	})

	req := BidRequest{
		RequestID:       "req_crypto_101",
		ScrubbedIntent:  "vector database indexing for rag",
		Category:        "vector_database",
		SlotType:        placement.SlotToolResponse,
		ReserveFloorCPM: 1.00,
		Timestamp:       time.Now(),
	}

	resp := agent.EvaluateBidRequest(req)
	if !resp.Submitted {
		t.Fatalf("Expected bid to be submitted, got no-bid: %s", resp.NoBidReason)
	}
	if resp.SignatureHex == "" {
		t.Fatalf("Expected SignatureHex on submitted bid")
	}

	// Verify Ed25519 signature
	sigPayload := fmt.Sprintf("%s:%s:%s:%.4f:%d",
		req.RequestID, resp.CampaignID, resp.AdvertiserID, resp.ShadedBidCPM, req.Timestamp.Unix())
	sigHash := sha256.Sum256([]byte(sigPayload))

	sigBytes, _ := hex.DecodeString(resp.SignatureHex)
	pubKeyBytes, _ := hex.DecodeString(key.PubKeyHex)

	if !ed25519.Verify(pubKeyBytes, sigHash[:], sigBytes) {
		t.Fatalf("Ed25519 signature verification failed for submitted bid")
	}

	// Verify shading
	if resp.ShadedBidCPM > resp.GrossBidCPM {
		t.Errorf("Expected ShadedBidCPM (%f) <= GrossBidCPM (%f)", resp.ShadedBidCPM, resp.GrossBidCPM)
	}
}

func TestBidderAgent_AuctionOutcomeTracking(t *testing.T) {
	agent := NewBidderAgent("adv_outcome", "Outcome Advertiser", nil)

	_ = agent.RegisterCampaign(CampaignConfig{
		CampaignID:     "cmp_outcome_01",
		AdvertiserID:   "adv_outcome",
		AdvertiserName: "Outcome Advertiser",
		DailyBudgetUSD: 100.00,
		MaxBidCPM:      5.00,
	})

	req := BidRequest{
		RequestID:       "req_outcome_01",
		ScrubbedIntent:  "general query",
		Category:        "general",
		SlotType:        placement.SlotToolResponse,
		ReserveFloorCPM: 0.50,
		Timestamp:       time.Now(),
	}

	resp := agent.EvaluateBidRequest(req)
	if resp.Submitted {
		agent.RecordAuctionOutcome(resp.CampaignID, true, 2.50)
	}

	summary := agent.GetSummary()
	if summary.TotalAuctionsWon != 1 {
		t.Fatalf("Expected 1 won auction, got %d", summary.TotalAuctionsWon)
	}
	if summary.OverallWinRatePercent != 100.0 {
		t.Fatalf("Expected 100%% win rate, got %f", summary.OverallWinRatePercent)
	}
}

func TestBidder_SimulationSuite(t *testing.T) {
	summary := RunBidderSimulation()

	if summary.TotalRequestsEvaluated == 0 {
		t.Fatalf("Expected requests evaluated in simulation")
	}
	if summary.TotalBidsSubmitted == 0 {
		t.Fatalf("Expected bids submitted in simulation")
	}
	if summary.TotalNoBids == 0 {
		t.Fatalf("Expected at least 1 no-bid for out-of-context query")
	}
	if summary.TotalAuctionsWon == 0 {
		t.Fatalf("Expected at least 1 won auction in simulation")
	}
	if summary.TotalShadingSavingsUSD <= 0 {
		t.Fatalf("Expected buyer shading savings to be positive")
	}

	scorecard := RenderBidderScorecard(summary)
	if !strings.Contains(scorecard, "AUTONOMOUS BIDDER (DSP) ENGINE") {
		t.Fatalf("Scorecard missing expected title")
	}

	md := RenderBidderMarkdown(summary)
	if !strings.Contains(md, "Autonomous DSP Bidder Engine Report") {
		t.Fatalf("Markdown missing expected title")
	}

	mermaid := RenderBidderMermaid(summary)
	if !strings.Contains(mermaid, "sequenceDiagram") {
		t.Fatalf("Mermaid missing sequenceDiagram")
	}
}

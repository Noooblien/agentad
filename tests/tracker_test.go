package tests

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
	"github.com/Noooblien/agentad/pkg/tracker"
)

func TestTracker_EndToEndFlow(t *testing.T) {
	cfg := tracker.DefaultTrackerConfig()
	tr := tracker.NewTracker(cfg)

	kp, err := attest.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}

	now := time.Now().UTC()
	sessionID := "sess_e2e_integration_01"

	// 1. Record 3 touches across different publishers and agent hosts
	touches := []tracker.TouchPoint{
		{
			ImpressionID: "imp_e2e_01",
			SessionID:    sessionID,
			CampaignID:   "cmp_datadog",
			AdvertiserID: "adv_datadog",
			PublisherID:  "pub_monitoring_hub",
			AgentID:      "agent_host_alpha",
			CostUSD:      0.003,
			Timestamp:    now.Add(-2 * time.Hour),
		},
		{
			ImpressionID: "imp_e2e_02",
			SessionID:    sessionID,
			CampaignID:   "cmp_datadog",
			AdvertiserID: "adv_datadog",
			PublisherID:  "pub_devops_catalog",
			AgentID:      "agent_host_beta",
			CostUSD:      0.005,
			Timestamp:    now.Add(-1 * time.Hour),
		},
		{
			ImpressionID: "imp_e2e_03",
			SessionID:    sessionID,
			CampaignID:   "cmp_datadog",
			AdvertiserID: "adv_datadog",
			PublisherID:  "pub_checkout_gateway",
			AgentID:      "agent_host_gamma",
			CostUSD:      0.008,
			Timestamp:    now.Add(-5 * time.Minute),
		},
	}

	for _, touch := range touches {
		if err := tr.RecordTouch(touch); err != nil {
			t.Fatalf("failed to record touch: %v", err)
		}
	}

	// 2. Prepare and sign conversion
	conv := tracker.Conversion{
		ConversionID:    "conv_e2e_01",
		SessionID:       sessionID,
		AdvertiserID:    "adv_datadog",
		CampaignID:      "cmp_datadog",
		ActionType:      tracker.ActionCheckout,
		RevenueUSD:      300.00,
		PayoutBountyUSD: 30.00, // $30 CPA bounty
		Timestamp:       now,
		Nonce:           "nonce_e2e_unique_5541",
	}

	if err := tracker.SignConversion(&conv, kp); err != nil {
		t.Fatalf("failed to sign conversion: %v", err)
	}

	// 3. Process conversion attribution
	res, err := tr.RecordConversion(conv, tracker.ModelPositionBased)
	if err != nil {
		t.Fatalf("conversion attribution failed: %v", err)
	}

	if res.Status != tracker.StatusAttributed {
		t.Fatalf("expected StatusAttributed, got %s", res.Status)
	}
	if !res.AttestationValid {
		t.Fatalf("expected cryptographic attestation to be verified")
	}
	if res.TouchesCount != 3 {
		t.Fatalf("expected 3 touches evaluated, got %d", res.TouchesCount)
	}

	// Position-based with 3 touches: First 40%, Middle 20%, Last 40%
	if math.Abs(res.Credits["imp_e2e_01"]-0.40) > 0.001 {
		t.Fatalf("expected imp_01 to have 40%% credit, got %f", res.Credits["imp_e2e_01"])
	}
	if math.Abs(res.Credits["imp_e2e_02"]-0.20) > 0.001 {
		t.Fatalf("expected imp_02 to have 20%% credit, got %f", res.Credits["imp_e2e_02"])
	}
	if math.Abs(res.Credits["imp_e2e_03"]-0.40) > 0.001 {
		t.Fatalf("expected imp_03 to have 40%% credit, got %f", res.Credits["imp_e2e_03"])
	}

	// Total bounty: $30. 70% to publishers ($21), 20% to hosts ($6), 10% to protocol ($3)
	var pubSum float64
	for _, p := range res.PublisherPayouts {
		pubSum += p
	}
	var hostSum float64
	for _, h := range res.HostPayouts {
		hostSum += h
	}

	if math.Abs(pubSum-21.00) > 0.001 {
		t.Fatalf("expected publisher payouts to total $21.00, got $%.4f", pubSum)
	}
	if math.Abs(hostSum-6.00) > 0.001 {
		t.Fatalf("expected host payouts to total $6.00, got $%.4f", hostSum)
	}
	if math.Abs(res.ProtocolFeeUSD-3.00) > 0.001 {
		t.Fatalf("expected protocol fee $3.00, got $%.4f", res.ProtocolFeeUSD)
	}

	// Verify stats
	stats := tr.GetStats()
	if stats.TotalTouches != 3 {
		t.Fatalf("expected 3 total touches recorded, got %d", stats.TotalTouches)
	}
	if stats.TotalConversions != 1 {
		t.Fatalf("expected 1 total conversion, got %d", stats.TotalConversions)
	}
	if stats.TotalRevenueUSD != 300.00 {
		t.Fatalf("expected $300 revenue, got %f", stats.TotalRevenueUSD)
	}
}

func TestTracker_MultiModelComparison(t *testing.T) {
	scenarios := tracker.GetDemoScenarios()
	if len(scenarios) == 0 {
		t.Fatalf("expected demo scenarios to be populated")
	}

	for _, sc := range scenarios {
		results := tracker.RunMultiModelComparison(sc)
		if len(results) != 5 {
			t.Fatalf("scenario %s expected 5 model results, got %d", sc.Name, len(results))
		}

		for _, r := range results {
			if r.Status != tracker.StatusAttributed {
				t.Fatalf("scenario %s model %s expected StatusAttributed, got %s", sc.Name, r.Model, r.Status)
			}

			// Ensure conservation of credits: sum of credits == 1.0
			var sum float64
			for _, credit := range r.Credits {
				sum += credit
			}
			if math.Abs(sum-1.0) > 0.001 {
				t.Fatalf("scenario %s model %s credits sum != 1.0 (got %f)", sc.Name, r.Model, sum)
			}
		}
	}
}

func TestTracker_Renderers(t *testing.T) {
	scenarios := tracker.GetDemoScenarios()
	results := tracker.RunMultiModelComparison(scenarios[0])
	res := results[0]

	// 1. Terminal Scorecard
	scorecard := tracker.RenderTrackerScorecard(res)
	if !strings.Contains(scorecard, "AGENTAD (AAP) // MULTI-TOUCH CONVERSION ATTRIBUTION") {
		t.Fatalf("scorecard missing header: %s", scorecard)
	}
	if !strings.Contains(scorecard, "TOUCHPOINT ATTRIBUTION BREAKDOWN") {
		t.Fatalf("scorecard missing breakdown: %s", scorecard)
	}

	// 2. Mermaid Diagram
	mermaid := tracker.RenderMermaid(res)
	if !strings.Contains(mermaid, "sequenceDiagram") || !strings.Contains(mermaid, "Proof-of-Conversion") {
		t.Fatalf("mermaid diagram invalid: %s", mermaid)
	}

	// 3. Markdown Report
	md := tracker.RenderMarkdown(results)
	if !strings.Contains(md, "AAP Multi-Touch Conversion Attribution Matrix") {
		t.Fatalf("markdown report missing table title: %s", md)
	}
}

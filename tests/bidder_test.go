package tests

import (
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/bidder"
)

func TestBidder_EndToEndSimulationSuite(t *testing.T) {
	summary := bidder.RunBidderSimulation()
	if summary.TotalRequestsEvaluated != 5 {
		t.Fatalf("expected 5 total evaluated requests, got %d", summary.TotalRequestsEvaluated)
	}
	if summary.TotalBidsSubmitted == 0 {
		t.Fatalf("expected bids submitted in simulation")
	}
	if summary.TotalNoBids == 0 {
		t.Fatalf("expected no-bid on out-of-context culinary request")
	}
	if summary.TotalAuctionsWon == 0 {
		t.Fatalf("expected at least 1 won auction in simulation")
	}
	if summary.TotalShadingSavingsUSD <= 0 {
		t.Fatalf("expected positive buyer shading savings")
	}

	scorecard := bidder.RenderBidderScorecard(summary)
	if !strings.Contains(scorecard, "AUTONOMOUS BIDDER (DSP) ENGINE") {
		t.Fatalf("scorecard missing expected title")
	}

	md := bidder.RenderBidderMarkdown(summary)
	if !strings.Contains(md, "Autonomous DSP Bidder Engine Report") {
		t.Fatalf("markdown report missing expected header")
	}

	mermaid := bidder.RenderBidderMermaid(summary)
	if !strings.Contains(mermaid, "sequenceDiagram") {
		t.Fatalf("mermaid diagram missing sequenceDiagram")
	}
}

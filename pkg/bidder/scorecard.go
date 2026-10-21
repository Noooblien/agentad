package bidder

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// RenderBidderScorecard formats the DSP bidder performance into an ANSI terminal scorecard.
func RenderBidderScorecard(s *BidderSummary) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("========================================================================================\n")
	sb.WriteString("        AGENT ADVERTISING PROTOCOL (AAP) - AUTONOMOUS BIDDER (DSP) ENGINE               \n")
	sb.WriteString("========================================================================================\n\n")

	sb.WriteString(fmt.Sprintf("Timestamp:               %s\n", s.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Advertiser Network ID:   %s\n", s.AdvertiserID))
	sb.WriteString(fmt.Sprintf("Total RTB Requests:      %d\n", s.TotalRequestsEvaluated))
	sb.WriteString(fmt.Sprintf("Bids Submitted:          %d (%.1f%% participation)\n",
		s.TotalBidsSubmitted, (float64(s.TotalBidsSubmitted)/float64(mathMax(1, s.TotalRequestsEvaluated)))*100.0))
	sb.WriteString(fmt.Sprintf("No-Bids / Skipped:       %d\n", s.TotalNoBids))
	sb.WriteString(fmt.Sprintf("Auctions Won:            %d\n", s.TotalAuctionsWon))
	sb.WriteString(fmt.Sprintf("Overall Win Rate:        %.1f%%\n", s.OverallWinRatePercent))
	sb.WriteString("----------------------------------------------------------------------------------------\n")

	sb.WriteString("ECONOMIC & BID SHADING SAVINGS:\n")
	sb.WriteString(fmt.Sprintf("  Gross Valuation Bid:   $%.6f USD\n", s.TotalGrossSpendUSD))
	sb.WriteString(fmt.Sprintf("  Actual Shaded Spend:   $%.6f USD\n", s.TotalShadedSpendUSD))
	sb.WriteString(fmt.Sprintf("  Buyer Surplus Saved:   $%.6f USD (%.1f%% savings preserved)\n",
		s.TotalShadingSavingsUSD, (s.TotalShadingSavingsUSD/mathMaxFloat(0.0001, s.TotalGrossSpendUSD))*100.0))
	sb.WriteString("----------------------------------------------------------------------------------------\n")

	sb.WriteString("CAMPAIGN PERFORMANCE PORTFOLIO:\n")
	sb.WriteString(fmt.Sprintf("%-20s | %-16s | %-18s | %-5s | %-5s | %-8s | %-10s\n",
		"Campaign ID", "Advertiser", "Strategy", "Bids", "Wins", "WinRate", "Spent USD"))
	sb.WriteString("---------------------+------------------+--------------------+-------+-------+----------+-----------\n")

	keys := make([]string, 0, len(s.CampaignReports))
	for k := range s.CampaignReports {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		rep := s.CampaignReports[k]
		sb.WriteString(fmt.Sprintf("%-20s | %-16s | %-18s | %-5d | %-5d | %-7.1f%% | $%-9.6f\n",
			truncate(rep.CampaignID, 20),
			truncate(rep.AdvertiserName, 16),
			truncate(string(rep.Strategy), 18),
			rep.BidsSubmitted,
			rep.AuctionsWon,
			rep.WinRatePercent,
			rep.TotalSpentUSD,
		))
	}

	sb.WriteString("========================================================================================\n\n")
	return sb.String()
}

// RenderBidderMarkdown outputs a structured Markdown performance report.
func RenderBidderMarkdown(s *BidderSummary) string {
	var sb strings.Builder

	sb.WriteString("# 🤖 Autonomous DSP Bidder Engine Report\n\n")
	sb.WriteString(fmt.Sprintf("> **Generated:** `%s` | **Network:** `%s` | **Overall Win Rate:** `%.1f%%`\n\n",
		s.GeneratedAt.Format(time.RFC3339), s.AdvertiserID, s.OverallWinRatePercent))

	sb.WriteString("### 📊 Operational Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("| :--- | :--- |\n")
	sb.WriteString(fmt.Sprintf("| **Total Requests Evaluated** | `%d` |\n", s.TotalRequestsEvaluated))
	sb.WriteString(fmt.Sprintf("| **Bids Submitted** | `%d` |\n", s.TotalBidsSubmitted))
	sb.WriteString(fmt.Sprintf("| **No-Bids / Passes** | `%d` |\n", s.TotalNoBids))
	sb.WriteString(fmt.Sprintf("| **Auctions Won** | `%d` |\n", s.TotalAuctionsWon))
	sb.WriteString(fmt.Sprintf("| **Gross Valuation Bid** | `$%.6f USD` |\n", s.TotalGrossSpendUSD))
	sb.WriteString(fmt.Sprintf("| **Shaded Spend** | `$%.6f USD` |\n", s.TotalShadedSpendUSD))
	sb.WriteString(fmt.Sprintf("| **Buyer Shading Savings** | `$%.6f USD` |\n", s.TotalShadingSavingsUSD))
	sb.WriteString("\n")

	sb.WriteString("### 🎯 Campaign Breakdown\n\n")
	sb.WriteString("| Campaign ID | Advertiser | Bidding Strategy | Bids | Wins | Win Rate | Total Spend |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")

	keys := make([]string, 0, len(s.CampaignReports))
	for k := range s.CampaignReports {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		rep := s.CampaignReports[k]
		sb.WriteString(fmt.Sprintf("| `%s` | %s | `%s` | %d | %d | `%.1f%%` | `$%.6f` |\n",
			rep.CampaignID, rep.AdvertiserName, rep.Strategy, rep.BidsSubmitted, rep.AuctionsWon, rep.WinRatePercent, rep.TotalSpentUSD))
	}
	sb.WriteString("\n")

	return sb.String()
}

// RenderBidderMermaid generates a sequence diagram illustrating the autonomous bidding lifecycle.
func RenderBidderMermaid(s *BidderSummary) string {
	var sb strings.Builder

	sb.WriteString("```mermaid\n")
	sb.WriteString("sequenceDiagram\n")
	sb.WriteString("    autonumber\n")
	sb.WriteString("    participant Exchange as AAP RTB Exchange\n")
	sb.WriteString("    participant Bidder as DSP Bidder Agent\n")
	sb.WriteString("    participant Valuate as Valuation & pCTR Engine\n")
	sb.WriteString("    participant Shading as Bid Shading Optimizer\n")
	sb.WriteString("    participant Signer as Ed25519 Cryptographic Signer\n\n")

	sb.WriteString("    Exchange->>Bidder: 1. Broadcast BidRequest (Scrubbed Intent + Floor CPM)\n")
	sb.WriteString("    Bidder->>Valuate: 2. Evaluate Semantic Match & Budget Pacing\n")
	sb.WriteString("    Valuate-->>Shading: 3. Compute Gross Valuation ($eCPM = pCTR * pCVR * Target)\n")
	sb.WriteString("    Shading-->>Signer: 4. Apply Buyer Shading Discount (Preserve Consumer Surplus)\n")
	sb.WriteString("    Signer-->>Bidder: 5. Sign Bid Payload with Ed25519 Private Key\n")
	sb.WriteString("    Bidder-->>Exchange: 6. Submit Cryptographic BidResponse (<50ms SLA)\n")
	sb.WriteString("```\n")

	return sb.String()
}

func truncate(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen-1] + "…"
}

func mathMax(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func mathMaxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

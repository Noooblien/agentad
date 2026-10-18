package pricing

import (
	"fmt"
	"strings"
	"time"
)

// RenderPricingScorecard produces an ANSI-colored terminal scorecard for an auction evaluation.
func RenderPricingScorecard(res *AuctionEvaluationResult) string {
	var b strings.Builder

	clearColor := "\033[1;32m" // Green
	if !res.Cleared {
		clearColor = "\033[1;31m" // Red
	}

	b.WriteString("\033[1;35m========================================================================================\033[0m\n")
	b.WriteString("  \033[1;37mAGENTAD (AAP) // DYNAMIC PUBLISHER FLOOR & BID SHADING ENGINE\033[0m\n")
	b.WriteString("\033[1;35m========================================================================================\033[0m\n")
	b.WriteString(fmt.Sprintf("  \033[1;33mAuction ID:\033[0m           %s\n", res.AuctionID))
	b.WriteString(fmt.Sprintf("  \033[1;33mPublisher / Domain:\033[0m   %s\n", res.PublisherID))
	b.WriteString(fmt.Sprintf("  \033[1;33mQuery Category:\033[0m       \033[1;36m%s\033[0m\n", res.Category))
	b.WriteString(fmt.Sprintf("  \033[1;33mAuction Status:\033[0m       %s[%s]\033[0m (Evaluated in %d µs)\n",
		clearColor, map[bool]string{true: "CLEARED", false: "UNSOLD_DROPPED"}[res.Cleared], res.ExecutionDurationUs))

	// Floor Price Breakdown
	df := res.DynamicFloor
	b.WriteString("  \033[1;34m----------------------------------------------------------------------------------------\033[0m\n")
	b.WriteString("  \033[1;37mDYNAMIC RESERVE FLOOR DECOMPOSITION:\033[0m\n")
	b.WriteString(fmt.Sprintf("  Base Category Floor:   $%.3f CPM\n", df.BaseFloorCPM))
	b.WriteString(fmt.Sprintf("  Fill Rate Multiplier:  x%.3f (Adaptive inventory feedback)\n", df.FillRateFactor))
	b.WriteString(fmt.Sprintf("  Competition Boost:     x%.3f (%d active bidders)\n", df.CompetitionFactor, res.TotalBidsReceived))
	b.WriteString(fmt.Sprintf("  Reputation Adjustment: %+.1f%%\n", df.ReputationAdjustment*100))
	b.WriteString(fmt.Sprintf("  \033[1;32mFinal Dynamic Floor:\033[0m   \033[1;32m$%.3f CPM\033[0m (Static Baseline: $%.3f CPM)\n",
		df.FinalFloorCPM, res.StaticFloorCPM))
	if df.IsClamped {
		b.WriteString(fmt.Sprintf("  \033[1;33mGuardrail Alert:\033[0m       %s\n", df.ClampReason))
	}

	// Candidates Breakdown
	b.WriteString("  \033[1;34m----------------------------------------------------------------------------------------\033[0m\n")
	b.WriteString("  \033[1;37mCANDIDATE BID EVALUATION & SHADING:\033[0m\n")
	b.WriteString("  #   Advertiser         Raw Bid CPM   Shaded CPM   Rel.  Trust  Score   Status\n")
	b.WriteString("  ----------------------------------------------------------------------------------------\n")

	for i, c := range res.EvaluatedBids {
		status := "\033[1;32mQUALIFIED\033[0m"
		for _, rej := range res.RejectedCandidates {
			if rej.CampaignID == c.CampaignID {
				status = fmt.Sprintf("\033[1;31mREJ: %s\033[0m", rej.Reason)
				break
			}
		}
		if res.Winner != nil && res.Winner.CampaignID == c.CampaignID {
			status = "\033[1;33m★ WINNER\033[0m"
		}

		b.WriteString(fmt.Sprintf("  [%d] %-18s $%-10.3f $%-10.3f %-5.2f %-5.2f %-7.3f %s\n",
			i+1, truncate(c.AdvertiserName, 18), c.RawBidCPM, c.ShadedBidCPM, c.RelevanceScore, c.TrustScore, c.CompositeScore, status))
	}

	// Yield & Surplus Outcomes
	b.WriteString("  \033[1;34m----------------------------------------------------------------------------------------\033[0m\n")
	b.WriteString("  \033[1;37mFINANCIAL SETTLEMENT & YIELD COMPARISON:\033[0m\n")
	if res.Cleared {
		b.WriteString(fmt.Sprintf("  Winning Campaign:      \033[1;36m%s\033[0m (%s)\n", res.Winner.AdvertiserName, res.Winner.CampaignID))
		b.WriteString(fmt.Sprintf("  Clearing Price:        \033[1;32m$%.3f CPM\033[0m ($%.6f USD per impression)\n",
			res.ClearingPriceCPM, res.PublisherYieldUSD))
		b.WriteString(fmt.Sprintf("  Buyer Surplus Saved:   \033[1;33m$%.6f USD\033[0m (Eliminated winner's curse)\n", res.BuyerSavingsUSD))
	} else {
		b.WriteString("  No candidate satisfied the dynamic reserve floor; auction remained unfilled.\n")
	}

	upliftSign := "+"
	if res.YieldUpliftUSD < 0 {
		upliftSign = ""
	}
	b.WriteString(fmt.Sprintf("  Static Baseline Yield: $%.6f USD (Clearing: $%.3f CPM)\n",
		res.StaticPublisherYieldUSD, res.StaticClearingPriceCPM))
	b.WriteString(fmt.Sprintf("  \033[1;37mPublisher Yield Delta:\033[0m \033[1;32m%s$%.6f USD (%s%.1f%%)\033[0m\n",
		upliftSign, res.YieldUpliftUSD, upliftSign, res.YieldUpliftPercent))
	b.WriteString("\033[1;35m========================================================================================\033[0m\n")

	return b.String()
}

// RenderSimulationSummary prints an aggregate scorecard across a batch of scenarios.
func RenderSimulationSummary(results []*AuctionEvaluationResult) string {
	var b strings.Builder

	totalAuctions := len(results)
	clearedCount := 0
	staticClearedCount := 0
	totalDynYield := 0.0
	totalStaticYield := 0.0
	totalBuyerSavings := 0.0

	for _, r := range results {
		if r.Cleared {
			clearedCount++
			totalDynYield += r.PublisherYieldUSD
			totalBuyerSavings += r.BuyerSavingsUSD
		}
		if r.StaticWouldHaveCleared {
			staticClearedCount++
			totalStaticYield += r.StaticPublisherYieldUSD
		}
	}

	dynFillRate := float64(clearedCount) / float64(totalAuctions) * 100.0
	staticFillRate := float64(staticClearedCount) / float64(totalAuctions) * 100.0
	overallDeltaUSD := totalDynYield - totalStaticYield
	overallDeltaPct := 0.0
	if totalStaticYield > 0 {
		overallDeltaPct = (overallDeltaUSD / totalStaticYield) * 100.0
	}

	b.WriteString("\n\033[1;36m========================================================================================\033[0m\n")
	b.WriteString("  \033[1;37mAGENTAD PRICING SIMULATION BENCHMARK SUMMARY\033[0m\n")
	b.WriteString("\033[1;36m========================================================================================\033[0m\n")
	b.WriteString(fmt.Sprintf("  Total Scenarios Evaluated:     %d\n", totalAuctions))
	b.WriteString(fmt.Sprintf("  Dynamic Floor Fill Rate:       \033[1;32m%.1f%%\033[0m (%d/%d cleared)\n", dynFillRate, clearedCount, totalAuctions))
	b.WriteString(fmt.Sprintf("  Static Baseline Fill Rate:     %.1f%% (%d/%d cleared)\n", staticFillRate, staticClearedCount, totalAuctions))
	b.WriteString(fmt.Sprintf("  Dynamic Publisher Total Yield: \033[1;32m$%.6f USD\033[0m\n", totalDynYield))
	b.WriteString(fmt.Sprintf("  Static Baseline Total Yield:   $%.6f USD\n", totalStaticYield))
	sign := "+"
	if overallDeltaUSD < 0 {
		sign = ""
	}
	b.WriteString(fmt.Sprintf("  Net Publisher Revenue Uplift:  \033[1;32m%s$%.6f USD (%s%.1f%%)\033[0m\n", sign, overallDeltaUSD, sign, overallDeltaPct))
	b.WriteString(fmt.Sprintf("  Total Buyer Surplus Preserved: \033[1;33m$%.6f USD\033[0m (Via intelligent bid shading)\n", totalBuyerSavings))
	b.WriteString("\033[1;36m========================================================================================\033[0m\n")

	return b.String()
}

// GenerateMermaidChart creates a Mermaid flowchart illustrating auction bid distribution and reserve floor clearance.
func GenerateMermaidChart(res *AuctionEvaluationResult) string {
	var b strings.Builder
	b.WriteString("```mermaid\n")
	b.WriteString("flowchart TD\n")
	b.WriteString(fmt.Sprintf("    subgraph Auction [\"Auction: %s (Category: %s)\"]\n", res.AuctionID, res.Category))
	b.WriteString(fmt.Sprintf("        Floor[\"Dynamic Reserve Floor: $%.2f CPM\"]:::floor\n", res.DynamicFloor.FinalFloorCPM))

	for i, c := range res.EvaluatedBids {
		nodeID := fmt.Sprintf("Bid%d", i+1)
		cleanName := strings.ReplaceAll(c.AdvertiserName, "\"", "'")
		if res.Winner != nil && res.Winner.CampaignID == c.CampaignID {
			b.WriteString(fmt.Sprintf("        %s[\"★ Winner: %s<br/>Bid: $%.2f CPM (Cleared: $%.2f)\"]:::winner\n",
				nodeID, cleanName, c.ShadedBidCPM, res.ClearingPriceCPM))
		} else if c.ShadedBidCPM < res.DynamicFloor.FinalFloorCPM {
			b.WriteString(fmt.Sprintf("        %s[\"Rejected: %s<br/>Bid $%.2f < Floor $%.2f\"]:::rejected\n",
				nodeID, cleanName, c.ShadedBidCPM, res.DynamicFloor.FinalFloorCPM))
		} else {
			b.WriteString(fmt.Sprintf("        %s[\"Qualified: %s<br/>Bid: $%.2f CPM\"]:::qualified\n",
				nodeID, cleanName, c.ShadedBidCPM))
		}
	}

	b.WriteString("    end\n")
	b.WriteString("    classDef floor fill:#ff9800,stroke:#e65100,stroke-width:2px,color:#fff;\n")
	b.WriteString("    classDef winner fill:#4caf50,stroke:#1b5e20,stroke-width:3px,color:#fff;\n")
	b.WriteString("    classDef qualified fill:#2196f3,stroke:#0d47a1,stroke-width:1px,color:#fff;\n")
	b.WriteString("    classDef rejected fill:#f44336,stroke:#b71c1c,stroke-width:1px,color:#fff;\n")
	b.WriteString("```\n")

	return b.String()
}

// GenerateMarkdownReport produces a detailed GitHub-flavored Markdown report.
func GenerateMarkdownReport(results []*AuctionEvaluationResult) string {
	var b strings.Builder

	b.WriteString("# AgentAd (AAP) // Dynamic Floor Price & Bid Shading Engine Report\n\n")
	b.WriteString(fmt.Sprintf("**Report Generated:** %s  \n", time.Now().UTC().Format(time.RFC3339)))
	b.WriteString("**Protocol:** AgentAd RTB Discovery & Monetization (AAP)  \n\n")

	b.WriteString("## Executive Summary\n\n")
	b.WriteString("| Metric | Dynamic Reserve + Bid Shading | Static Baseline ($1.00 CPM) | Delta |\n")
	b.WriteString("| :--- | :--- | :--- | :--- |\n")

	totalAuctions := len(results)
	clearedCount := 0
	staticClearedCount := 0
	totalDynYield := 0.0
	totalStaticYield := 0.0
	totalBuyerSavings := 0.0

	for _, r := range results {
		if r.Cleared {
			clearedCount++
			totalDynYield += r.PublisherYieldUSD
			totalBuyerSavings += r.BuyerSavingsUSD
		}
		if r.StaticWouldHaveCleared {
			staticClearedCount++
			totalStaticYield += r.StaticPublisherYieldUSD
		}
	}

	deltaUSD := totalDynYield - totalStaticYield
	deltaPct := 0.0
	if totalStaticYield > 0 {
		deltaPct = (deltaUSD / totalStaticYield) * 100.0
	}

	b.WriteString(fmt.Sprintf("| **Fill Rate** | %.1f%% (%d/%d) | %.1f%% (%d/%d) | %+.1f%% |\n",
		float64(clearedCount)/float64(totalAuctions)*100, clearedCount, totalAuctions,
		float64(staticClearedCount)/float64(totalAuctions)*100, staticClearedCount, totalAuctions,
		(float64(clearedCount-staticClearedCount)/float64(totalAuctions))*100))
	sign := "+"
	if deltaUSD < 0 {
		sign = ""
	}
	b.WriteString(fmt.Sprintf("| **Publisher Total Yield** | **$%.6f USD** | $%.6f USD | **%s$%.6f USD (%+.1f%%)** |\n",
		totalDynYield, totalStaticYield, sign, deltaUSD, deltaPct))
	b.WriteString(fmt.Sprintf("| **Buyer Surplus Saved** | **$%.6f USD** | $0.00 USD | +$%.6f USD |\n\n",
		totalBuyerSavings, totalBuyerSavings))

	b.WriteString("## Evaluated Scenarios\n\n")
	b.WriteString("| Scenario / Auction | Category | Dynamic Floor | Winner | Clearing CPM | Uplift % | Status |\n")
	b.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")

	for _, r := range results {
		winnerName := "None"
		if r.Winner != nil {
			winnerName = r.Winner.AdvertiserName
		}
		status := "Dropped"
		if r.Cleared {
			status = "Cleared"
		}
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | $%.3f | %s | $%.3f | %+.1f%% | %s |\n",
			r.AuctionID, r.Category, r.DynamicFloor.FinalFloorCPM, winnerName, r.ClearingPriceCPM, r.YieldUpliftPercent, status))
	}

	b.WriteString("\n## Visual Flowcharts\n\n")
	if len(results) > 0 {
		b.WriteString(GenerateMermaidChart(results[0]))
	}

	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

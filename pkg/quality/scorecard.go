package quality

import (
	"fmt"
	"strings"
)

// ANSI color escape constants for terminal scorecard output.
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorGray   = "\033[90m"
)

// RenderTerminalScorecard outputs a high-contrast CLI scorecard summarizing quality audits.
func RenderTerminalScorecard(summary *QualitySummary) string {
	var sb strings.Builder

	sb.WriteString("\n" + colorBold + colorPurple + "========================================================================================================\n" + colorReset)
	sb.WriteString(colorBold + colorWhite + "  AGENTAD (AAP) // AGENT AD QUALITY SCORE (AQS) & FACTSHEET GROUNDING ENGINE\n" + colorReset)
	sb.WriteString(colorBold + colorPurple + "========================================================================================================\n" + colorReset)

	sb.WriteString(fmt.Sprintf("  %sEvaluated:%s %d ads  |  %sEligible:%s %d  |  %sDisqualified:%s %d  |  %sAverage AQS:%s %.1f/10.0  |  %sLatency:%s %s\n",
		colorBold, colorReset, summary.TotalEvaluated,
		colorGreen, colorReset, summary.EligibleCount,
		colorRed, colorReset, summary.DisqualifiedCount,
		colorCyan, colorReset, summary.AverageAQS,
		colorGray, colorReset, summary.Duration))
	sb.WriteString(colorGray + "--------------------------------------------------------------------------------------------------------\n" + colorReset)

	sb.WriteString(fmt.Sprintf("%-16s %-16s %-8s %-6s %-6s %-6s %-6s %-8s %-10s %s\n",
		"ADVERTISER", "AD ID", "RAW BID", "AQS", "GRND", "RELV", "ADPT", "RANK", "DISCOUNT", "TIER"))
	sb.WriteString(colorGray + "--------------------------------------------------------------------------------------------------------\n" + colorReset)

	for _, ev := range summary.Evaluations {
		tierColor := colorGreen
		tierIcon := "✓"
		discountStr := fmt.Sprintf("%+.0f%%", (ev.ClearingMultiplier-1.0)*100)
		if ev.ClearingMultiplier == 1.0 {
			discountStr = "0%"
		}

		switch ev.Tier {
		case TierExcellent:
			tierColor = colorGreen
			tierIcon = "★"
		case TierGood:
			tierColor = colorCyan
			tierIcon = "✓"
		case TierNeutral:
			tierColor = colorYellow
			tierIcon = "•"
		case TierSubstandard:
			tierColor = colorRed
			tierIcon = "▲"
		case TierDisqualified:
			tierColor = colorRed
			tierIcon = "✗"
			discountStr = "REJECT"
		}

		sb.WriteString(fmt.Sprintf("%-16s %-16s $%5.2f  %s%4.1f%s   %4.2f  %4.2f  %4.2f  %s$%5.2f%s   %-10s %s%s %-12s%s\n",
			truncate(ev.Advertiser, 15),
			truncate(ev.AdID, 15),
			ev.RawBidCPM,
			colorBold+tierColor, ev.QualityScore, colorReset,
			ev.GroundingScore,
			ev.RelevanceScore,
			ev.AdoptionScore,
			colorBold+colorWhite, ev.EffectiveAdRank, colorReset,
			discountStr,
			tierColor, tierIcon, ev.Tier, colorReset))

		if len(ev.FlaggedIssues) > 0 {
			for _, issue := range ev.FlaggedIssues {
				sb.WriteString(fmt.Sprintf("    %s↳ [FLAG]%s %s\n", colorRed, colorReset, issue))
			}
		}
	}

	sb.WriteString(colorBold + colorPurple + "========================================================================================================\n" + colorReset)
	sb.WriteString(colorGray + "  Formula: AQS = 1.0 + 9.0*(0.40*Grounding + 0.35*Relevance + 0.25*Adoption - Penalties)\n" + colorReset)
	sb.WriteString(colorGray + "  AdRank: EffectiveAdRank = RawBidCPM * (AQS / 10.0) | High AQS Earns Up To 20% Clearing Discount\n" + colorReset)

	return sb.String()
}

// RenderQualityMermaid generates a Mermaid flowchart mapping factsheet audits to AQS ad ranking.
func RenderQualityMermaid(summary *QualitySummary) string {
	var sb strings.Builder
	sb.WriteString("```mermaid\nflowchart TD\n")
	sb.WriteString("    subgraph Input [Agent Query & Candidates]\n")
	sb.WriteString("        Q[\"User Query Intent\"] --> EvalEngine[\"Quality Evaluation Engine\"]\n")
	sb.WriteString("        Ads[\"Ad Candidates & Factsheets\"] --> EvalEngine\n")
	sb.WriteString("    end\n\n")

	sb.WriteString("    subgraph Metrics [Audit Pillars]\n")
	sb.WriteString("        EvalEngine --> G[\"1. Factual Grounding (40%)\"]\n")
	sb.WriteString("        EvalEngine --> R[\"2. Semantic Relevance (35%)\"]\n")
	sb.WriteString("        EvalEngine --> A[\"3. Agent Adoption (25%)\"]\n")
	sb.WriteString("        EvalEngine --> P[\"4. Hallucination Penalties\"]\n")
	sb.WriteString("    end\n\n")

	sb.WriteString("    subgraph Synthesis [Composite Quality Score]\n")
	sb.WriteString("        G --> AQS[\"AQS Scale: 1.0 - 10.0\"]\n")
	sb.WriteString("        R --> AQS\n")
	sb.WriteString("        A --> AQS\n")
	sb.WriteString("        P --> AQS\n")
	sb.WriteString("    end\n\n")

	sb.WriteString("    subgraph AuctionGate [Auction Clearing & AdRank]\n")
	sb.WriteString("        AQS --> Rank[\"Effective AdRank = Bid * (AQS / 10.0)\"]\n")
	for i, ev := range summary.Evaluations {
		nodeID := fmt.Sprintf("Ad_%d", i+1)
		label := fmt.Sprintf("%s (AQS: %.1f | Rank: $%.2f)", ev.Advertiser, ev.QualityScore, ev.EffectiveAdRank)
		if ev.Eligible {
			sb.WriteString(fmt.Sprintf("        Rank --> %s[\"✅ %s\"]\n", nodeID, label))
		} else {
			sb.WriteString(fmt.Sprintf("        Rank --> %s[\"❌ %s [DISQUALIFIED]\"]\n", nodeID, label))
		}
	}
	sb.WriteString("    end\n```")
	return sb.String()
}

// RenderQualityMarkdown formats an empirical markdown table for PR documentation and reports.
func RenderQualityMarkdown(summary *QualitySummary) string {
	var sb strings.Builder

	sb.WriteString("### Agent Ad Quality Score (AQS) & Factsheet Grounding Audit\n\n")
	sb.WriteString(fmt.Sprintf("**Evaluated**: %d ads | **Eligible**: %d | **Disqualified**: %d | **Average AQS**: `%.1f/10.0` | **Latency**: `%s`\n\n",
		summary.TotalEvaluated, summary.EligibleCount, summary.DisqualifiedCount, summary.AverageAQS, summary.Duration))

	sb.WriteString("| Advertiser | Ad ID | Raw Bid CPM | Grounding | Relevance | Adoption | AQS | Effective AdRank | Discount | Tier |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")

	for _, ev := range summary.Evaluations {
		discountStr := fmt.Sprintf("%+.0f%%", (ev.ClearingMultiplier-1.0)*100)
		if ev.ClearingMultiplier == 1.0 {
			discountStr = "0%"
		}
		tierBadge := fmt.Sprintf("`%s`", ev.Tier)
		if ev.Tier == TierDisqualified {
			tierBadge = "**❌ DISQUALIFIED**"
			discountStr = "N/A"
		} else if ev.Tier == TierExcellent {
			tierBadge = "🌟 **EXCELLENT**"
		}

		sb.WriteString(fmt.Sprintf("| **%s** | `%s` | `$%.2f` | `%.2f` | `%.2f` | `%.2f` | `%.1f/10` | **`$%.2f`** | `%s` | %s |\n",
			ev.Advertiser, ev.AdID, ev.RawBidCPM, ev.GroundingScore, ev.RelevanceScore, ev.AdoptionScore, ev.QualityScore, ev.EffectiveAdRank, discountStr, tierBadge))
	}

	return sb.String()
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-2] + ".."
	}
	return s
}

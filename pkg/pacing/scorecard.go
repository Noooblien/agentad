package pacing

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[1;31m"
	colorGreen  = "\033[1;32m"
	colorYellow = "\033[1;33m"
	colorCyan   = "\033[1;36m"
	colorBold   = "\033[1m"
)

// RenderScorecard produces an ANSI dashboard for a pacing simulation result.
func RenderScorecard(res SimulationResult) string {
	var sb strings.Builder

	statusBadge := fmt.Sprintf("%s[PASS / OPTIMAL]%s", colorGreen, colorReset)
	if !res.Passed {
		statusBadge = fmt.Sprintf("%s[SUB-OPTIMAL]%s", colorRed, colorReset)
	}

	depletionStr := "Paced across 24h"
	if res.DepletionHour >= 0 {
		depletionStr = fmt.Sprintf("%sHour %d:00 UTC (Burnout)%s", colorRed, res.DepletionHour, colorReset)
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s╭────────────────────────────────────────────────────────────────────────────╮%s\n", colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s  🎯  %sAgentAd Dynamic Budget Pacing & PID Controller Scorecard%s          %s│%s\n", colorCyan, colorReset, colorBold, colorReset, colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────────┤%s\n", colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("│  %sStrategy:%s         %-28s  %sStatus:%s       %-22s │\n", colorBold, colorReset, truncate(res.Strategy, 28), colorBold, colorReset, statusBadge))
	sb.WriteString(fmt.Sprintf("│  %sDaily Budget:%s     %-28s  %sTotal Spent:%s  %-12s │\n", colorBold, colorReset, fmt.Sprintf("$%.2f USD", res.DailyBudgetUSD), colorBold, colorReset, fmt.Sprintf("$%.2f USD", res.TotalSpentUSD)))
	sb.WriteString(fmt.Sprintf("│  %sUtilization:%s      %-28s  %sPacing RMSE:%s  %-12s │\n", colorBold, colorReset, fmt.Sprintf("%.1f%%", res.BudgetUtilizationPct), colorBold, colorReset, fmt.Sprintf("$%.2f", res.PacingErrorRMSE)))
	sb.WriteString(fmt.Sprintf("│  %sDepletion Point:%s  %-40s  %sAuctions Won:%s %-5s │\n", colorBold, colorReset, depletionStr, colorBold, colorReset, fmt.Sprintf("%d", res.AuctionsWon)))
	sb.WriteString(fmt.Sprintf("│  %sAuctions Handled:%s %-28s  %sWin Rate:%s     %-12s │\n", colorBold, colorReset, fmt.Sprintf("%d / %d entered", res.AuctionsParticipated, res.AuctionsEligible), colorBold, colorReset, fmt.Sprintf("%.1f%%", (float64(res.AuctionsWon)/float64(res.AuctionsParticipated+1))*100)))
	sb.WriteString(fmt.Sprintf("│  %sSummary:%s          %-56s │\n", colorBold, colorReset, truncate(res.Summary, 56)))

	// Hourly spend mini-waterfall (00h, 06h, 12h, 18h, 23h samples)
	sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────────┤%s\n", colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("│  %sSpend Waterfall:%s 00h: $%.2f | 06h: $%.2f | 12h: $%.2f | 18h: $%.2f | 23h: $%.2f  │\n",
		colorBold, colorReset, res.HourlySpend[0], res.HourlySpend[6], res.HourlySpend[12], res.HourlySpend[18], res.HourlySpend[23]))
	sb.WriteString(fmt.Sprintf("%s╰────────────────────────────────────────────────────────────────────────────╯%s\n", colorCyan, colorReset))
	return sb.String()
}

// RenderSuiteScorecard produces an aggregated ANSI summary for the full pacing suite.
func RenderSuiteScorecard(results []SimulationResult) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s========================================================================================%s\n", colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("   %sAGENTAD (AAP) // DYNAMIC BUDGET PACING & SPEND PROTECTION BENCHMARK%s\n", colorBold, colorReset))
	sb.WriteString(fmt.Sprintf("%s========================================================================================%s\n", colorCyan, colorReset))

	passed := 0
	for i, r := range results {
		fmt.Fprintf(&sb, "\n%s[Scenario %d/%d]%s %s%s%s\n",
			colorYellow, i+1, len(results), colorReset, colorBold, r.Strategy, colorReset)
		sb.WriteString(RenderScorecard(r))
		if r.Passed {
			passed++
		}
	}

	fmt.Fprintf(&sb, "\n%s✓ Pacing Benchmark Complete.%s Verified %d/%d scenarios meet RTB budget governance standards.\n\n",
		colorGreen, colorReset, passed, len(results))
	return sb.String()
}

// RenderMarkdown creates a formatted Markdown report table for pull requests and CI/CD.
func RenderMarkdown(results []SimulationResult) string {
	var sb strings.Builder

	sb.WriteString("### 🎯 AgentAd Dynamic Budget Pacing & PID Controller Benchmark\n\n")
	sb.WriteString("| Strategy | Budget | Spent | Utilization % | Depletion Point | Pacing RMSE | Auctions Won | Verdict |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")

	for _, r := range results {
		depStr := "24h Paced"
		if r.DepletionHour >= 0 {
			depStr = fmt.Sprintf("Hour %d (Burnout)", r.DepletionHour)
		}
		verdict := "✅ PASS"
		if !r.Passed {
			verdict = "❌ FAIL"
		}

		sb.WriteString(fmt.Sprintf("| **%s** | `$%.2f` | `$%.2f` | `%.1f%%` | %s | `$%.2f` | %d | %s |\n",
			r.Strategy, r.DailyBudgetUSD, r.TotalSpentUSD, r.BudgetUtilizationPct, depStr, r.PacingErrorRMSE, r.AuctionsWon, verdict))
	}
	sb.WriteString("\n")
	return sb.String()
}

// RenderJSON serializes the simulation results to formatted JSON.
func RenderJSON(results []SimulationResult) (string, error) {
	bytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

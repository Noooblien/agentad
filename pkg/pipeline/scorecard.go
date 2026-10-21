package pipeline

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

// RenderPipelineScorecard generates a high-contrast ANSI terminal scorecard.
func RenderPipelineScorecard(summary *PipelineSummary) string {
	var sb strings.Builder

	sb.WriteString("\n" + colorBold + colorBlue + "========================================================================================================\n" + colorReset)
	sb.WriteString(colorBold + colorWhite + "  AGENTAD (AAP) // ZERO-JAILBREAK END-TO-END 12-STAGE ORCHESTRATION PIPELINE\n" + colorReset)
	sb.WriteString(colorBold + colorBlue + "========================================================================================================\n" + colorReset)

	sb.WriteString(fmt.Sprintf("  %sEvaluated:%s %d  |  %sCleared:%s %d  |  %sSuppressed:%s %d  |  %sRejected:%s %d  |  %sAvg Duration:%s %.2fms\n",
		colorBold, colorReset, summary.TotalEvaluated,
		colorGreen, colorReset, summary.ClearedCount,
		colorYellow, colorReset, summary.SuppressedCount,
		colorRed, colorReset, summary.RejectedCount,
		colorCyan, colorReset, summary.AverageDurationMs))
	sb.WriteString(colorGray + "--------------------------------------------------------------------------------------------------------\n" + colorReset)

	sb.WriteString(fmt.Sprintf("%-20s %-12s %-16s %-18s %-10s %-10s %s\n",
		"TRACE ID", "STATUS", "HALT STAGE", "WINNING SPONSOR", "CLEAR CPM", "LATENCY", "OUTCOME / AUDIT REASON"))
	sb.WriteString(colorGray + "--------------------------------------------------------------------------------------------------------\n" + colorReset)

	for _, r := range summary.Runs {
		statusColor := colorGreen
		if r.Status == StatusSuppressed {
			statusColor = colorYellow
		} else if r.Status == StatusRejected || r.Status == StatusNoBid {
			statusColor = colorRed
		}

		halt := string(r.HaltStage)
		if halt == "" {
			halt = "—"
		}

		sponsor := r.WinningAdvertiser
		if sponsor == "" {
			sponsor = "—"
		}

		cpmStr := "—"
		if r.ClearingPriceCPM > 0 {
			cpmStr = fmt.Sprintf("$%.2f", r.ClearingPriceCPM)
		}

		sb.WriteString(fmt.Sprintf("%-20s %s%-12s%s %-16s %-18s %-10s %-10s %s\n",
			truncate(r.TraceID, 19),
			statusColor, r.Status, colorReset,
			truncate(halt, 15),
			truncate(sponsor, 17),
			cpmStr,
			fmt.Sprintf("%.2fms", r.TotalDurationMs),
			truncate(r.Reason, 32)))
	}

	sb.WriteString(colorGray + "--------------------------------------------------------------------------------------------------------\n" + colorReset)
	return sb.String()
}

// RenderPipelineMarkdown formats the summary into a clean GitHub-flavored Markdown table.
func RenderPipelineMarkdown(summary *PipelineSummary) string {
	var sb strings.Builder

	sb.WriteString("## AgentAd (AAP) End-to-End Orchestration Pipeline Report\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Evaluated**: %d\n", summary.TotalEvaluated))
	sb.WriteString(fmt.Sprintf("- **Cleared Impressions**: %d\n", summary.ClearedCount))
	sb.WriteString(fmt.Sprintf("- **Suppressed (Quiet Zone / Fatigue)**: %d\n", summary.SuppressedCount))
	sb.WriteString(fmt.Sprintf("- **Rejected (Fraud / Safety / Injection)**: %d\n", summary.RejectedCount))
	sb.WriteString(fmt.Sprintf("- **Average Latency**: `%.2fms` (SLA Budget: `<50ms`)\n\n", summary.AverageDurationMs))

	sb.WriteString("| Trace ID | Status | Halt Stage | Winning Sponsor | Clearing CPM | Latency | Outcome Reason |\n")
	sb.WriteString("|:---|:---|:---|:---|:---|:---|:---|\n")

	for _, r := range summary.Runs {
		statusIcon := "✅ CLEARED"
		if r.Status == StatusSuppressed {
			statusIcon = "⚠️ SUPPRESSED"
		} else if r.Status == StatusRejected || r.Status == StatusNoBid {
			statusIcon = "❌ " + string(r.Status)
		}

		halt := string(r.HaltStage)
		if halt == "" {
			halt = "—"
		}

		sponsor := r.WinningAdvertiser
		if sponsor == "" {
			sponsor = "—"
		}

		cpmStr := "—"
		if r.ClearingPriceCPM > 0 {
			cpmStr = fmt.Sprintf("$%.2f", r.ClearingPriceCPM)
		}

		sb.WriteString(fmt.Sprintf("| `%s` | %s | `%s` | **%s** | %s | `%.2fms` | %s |\n",
			r.TraceID, statusIcon, halt, sponsor, cpmStr, r.TotalDurationMs, r.Reason))
	}

	return sb.String()
}

// RenderPipelineMermaid outputs a sequence diagram detailing the complete 12-stage lifecycle.
func RenderPipelineMermaid() string {
	var sb strings.Builder

	sb.WriteString("```mermaid\n")
	sb.WriteString("sequenceDiagram\n")
	sb.WriteString("    autonumber\n")
	sb.WriteString("    participant Agent as Autonomous Agent\n")
	sb.WriteString("    participant Fraud as Fraud &amp; Telemetry Gate\n")
	sb.WriteString("    participant Privacy as Zero-PII Scrubber\n")
	sb.WriteString("    participant Safety as Brand Safety &amp; Competitor Gate\n")
	sb.WriteString("    participant Floor as Dynamic Floor &amp; Bid Shading\n")
	sb.WriteString("    participant Auction as RTB Auction Engine\n")
	sb.WriteString("    participant Quality as AQS Quality &amp; Grounding\n")
	sb.WriteString("    participant Pacing as PID Budget Pacing\n")
	sb.WriteString("    participant Placement as Placement &amp; Quiet Zone Arbiter\n")
	sb.WriteString("    participant Guard as AST Factsheet Purifier\n")
	sb.WriteString("    participant Attest as Ed25519 PoI Attestor\n")
	sb.WriteString("    participant Escrow as Micro-Escrow (70/20/10 Split)\n\n")

	sb.WriteString("    Agent->>Fraud: 1. Send Query with Agent Telemetry\n")
	sb.WriteString("    Fraud-->>Privacy: 2. Entropy &amp; Velocity Verified (Clean Traffic)\n")
	sb.WriteString("    Privacy-->>Safety: 3. PII &amp; Credentials Scrubbed (Zero-PII Intent)\n")
	sb.WriteString("    Safety-->>Floor: 4. Brand Suitability &amp; Competitor Checks Pass\n")
	sb.WriteString("    Floor-->>Auction: 5. Dynamic Reserve Floor Injected\n")
	sb.WriteString("    Auction-->>Quality: 6. Second-Price Auction Cleared\n")
	sb.WriteString("    Quality-->>Pacing: 7. AQS Quality Score &amp; Clearing Discount Computed\n")
	sb.WriteString("    Pacing-->>Placement: 8. Spend Velocity &amp; Hourly Budget Cleared\n")
	sb.WriteString("    Placement-->>Guard: 9. Quiet Zone Clear &amp; Context Budget Verified\n")
	sb.WriteString("    Guard-->>Attest: 10. Factsheet Purified (No Imperative Instructions)\n")
	sb.WriteString("    Attest-->>Escrow: 11. Ed25519 Proof-of-Impression Receipt Signed\n")
	sb.WriteString("    Escrow-->>Agent: 12. Automated Micro-Settlement (70/20/10) &amp; Framed Factsheet Context\n")
	sb.WriteString("```\n")

	return sb.String()
}

func truncate(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen-1] + "…"
}

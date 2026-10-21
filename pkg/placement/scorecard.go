package placement

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

// RenderPlacementScorecard outputs a high-contrast CLI scorecard for placement evaluations.
func RenderPlacementScorecard(summary *PlacementSummary) string {
	var sb strings.Builder

	sb.WriteString("\n" + colorBold + colorPurple + "========================================================================================================\n" + colorReset)
	sb.WriteString(colorBold + colorWhite + "  AGENTAD (AAP) // AGENT SLOT PLACEMENT, FREQUENCY CAPPING & ATTENTION BUDGET ARBITER\n" + colorReset)
	sb.WriteString(colorBold + colorPurple + "========================================================================================================\n" + colorReset)

	sb.WriteString(fmt.Sprintf("  %sSession:%s %s  |  %sEvaluated:%s %d  |  %sApproved:%s %d  |  %sSuppressed:%s %d  |  %sAvg Fatigue:%s %.3f\n",
		colorBold, colorReset, summary.SessionID,
		colorBold, colorReset, summary.TotalEvaluated,
		colorGreen, colorReset, summary.ApprovedCount,
		colorRed, colorReset, summary.SuppressedCount,
		colorCyan, colorReset, summary.AverageFatigue))
	sb.WriteString(colorGray + "--------------------------------------------------------------------------------------------------------\n" + colorReset)

	sb.WriteString(fmt.Sprintf("%-16s %-14s %-12s %-14s %-8s %-9s %-8s %-12s %s\n",
		"ADVERTISER", "AD ID", "DECISION", "SELECTED SLOT", "FATIGUE", "CTX RATIO", "ROI", "TOKEN COST", "OUTCOME"))
	sb.WriteString(colorGray + "--------------------------------------------------------------------------------------------------------\n" + colorReset)

	for _, ev := range summary.Evaluations {
		statusColor := colorGreen
		statusIcon := "✓ APPROVED"
		if !ev.Approved {
			statusColor = colorRed
			statusIcon = "✗ BLOCKED"
		}

		slotStr := string(ev.SelectedSlot)
		if slotStr == "" || slotStr == "NONE" {
			slotStr = "—"
		}

		sb.WriteString(fmt.Sprintf("%-16s %-14s %s%-12s%s %-14s %-8.3f %-8.2f%% %-7.2fx $%-11.5f %s%s%s\n",
			truncate(ev.Candidate.Advertiser, 15),
			truncate(ev.Candidate.AdID, 13),
			statusColor, string(ev.Decision), colorReset,
			truncate(slotStr, 13),
			ev.CurrentFatigue,
			ev.ContextTokenRatio*100,
			ev.ROIRatio,
			ev.EstTokenCostUSD,
			statusColor, statusIcon, colorReset))

		sb.WriteString(fmt.Sprintf("  %s↳ Audit Note:%s %s\n", colorGray, colorReset, ev.Reason))
		if ev.Approved && ev.FormattedSlotPayload != "" {
			lines := strings.Split(ev.FormattedSlotPayload, "\n")
			snippet := lines[0]
			if len(lines) > 1 {
				snippet += fmt.Sprintf(" ... (+%d lines)", len(lines)-1)
			}
			sb.WriteString(fmt.Sprintf("  %s↳ Payload Container:%s %s\n", colorCyan, colorReset, snippet))
		}
	}
	sb.WriteString(colorBold + colorPurple + "========================================================================================================\n\n" + colorReset)

	return sb.String()
}

// RenderPlacementMermaid renders a sequence diagram or flowchart showing the placement decision pipeline.
func RenderPlacementMermaid(summary *PlacementSummary) string {
	var sb strings.Builder

	sb.WriteString("```mermaid\n")
	sb.WriteString("flowchart TD\n")
	sb.WriteString("    Start([Candidate Ad Enters Placement Pipeline]) --> FatigueCheck{Fatigue Check<br/>Session Decay &lt; Threshold?}\n")
	sb.WriteString("    FatigueCheck -- No (Fatigue High) --> BlockFatigue[Block: CAPPED_FATIGUE]\n")
	sb.WriteString("    FatigueCheck -- Yes --> CooldownCheck{Cooldown Check<br/>Turn Spacing &amp; Caps?}\n")
	sb.WriteString("    CooldownCheck -- Active --> BlockCooldown[Block: COOLDOWN_ACTIVE]\n")
	sb.WriteString("    CooldownCheck -- Clear --> StageGate{Lifecycle Stage Gate<br/>Agent Cognitive Phase?}\n")
	sb.WriteString("    StageGate -- Reasoning / Execution --> BlockStage[Block: SUPPRESSED_STAGE<br/>Quiet Zone Lock]\n")
	sb.WriteString("    StageGate -- Exploration / Decision --> EconGate{Token Economics<br/>Context Ratio &amp; ROI &gt; 1.25x?}\n")
	sb.WriteString("    EconGate -- Ratio &gt; 5% or ROI &lt; 1.25x --> BlockEcon[Block: SUBECONOMIC_ROI]\n")
	sb.WriteString("    EconGate -- Profitable &amp; Bound --> SlotResolve[Resolve Structural Slot<br/>TOOL_DEF / TOOL_RESP / SYNTHESIS]\n")
	sb.WriteString("    SlotResolve --> Approved([Approved: Safe Factsheet Injected])\n")
	sb.WriteString("```\n")

	return sb.String()
}

// RenderPlacementMarkdown renders a structured Markdown table summarizing placement arbitration.
func RenderPlacementMarkdown(summary *PlacementSummary) string {
	var sb strings.Builder

	sb.WriteString("## AgentAd (AAP) Placement Arbitration Report\n\n")
	sb.WriteString(fmt.Sprintf("- **Session ID**: `%s`\n", summary.SessionID))
	sb.WriteString(fmt.Sprintf("- **Total Evaluated**: %d\n", summary.TotalEvaluated))
	sb.WriteString(fmt.Sprintf("- **Approved**: %d\n", summary.ApprovedCount))
	sb.WriteString(fmt.Sprintf("- **Suppressed**: %d\n", summary.SuppressedCount))
	sb.WriteString(fmt.Sprintf("- **Average Cognitive Fatigue**: `%.3f`\n\n", summary.AverageFatigue))

	sb.WriteString("| Advertiser | Ad ID | Lifecycle Stage | Decision | Slot | Fatigue | Ctx Ratio | ROI | Status |\n")
	sb.WriteString("|:---|:---|:---|:---|:---|:---|:---|:---|:---|\n")

	for _, ev := range summary.Evaluations {
		status := "✅ APPROVED"
		if !ev.Approved {
			status = "❌ BLOCKED"
		}
		slotStr := string(ev.SelectedSlot)
		if slotStr == "" || slotStr == "NONE" {
			slotStr = "—"
		}

		sb.WriteString(fmt.Sprintf("| **%s** | `%s` | `%s` | `%s` | `%s` | `%.3f` | `%.2f%%` | `%.2fx` | %s |\n",
			ev.Candidate.Advertiser,
			ev.Candidate.AdID,
			ev.Reason,
			ev.Decision,
			slotStr,
			ev.CurrentFatigue,
			ev.ContextTokenRatio*100,
			ev.ROIRatio,
			status))
	}

	return sb.String()
}

func truncate(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen-1] + "…"
}

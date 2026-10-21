package safety

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ANSI color escapes for terminal rendering.
const (
	colorReset   = "\033[0m"
	colorBold    = "\033[1m"
	colorRed     = "\033[1;31m"
	colorGreen   = "\033[1;32m"
	colorYellow  = "\033[1;33m"
	colorBlue    = "\033[1;34m"
	colorMagenta = "\033[1;35m"
	colorCyan    = "\033[1;36m"
	colorGray    = "\033[0;90m"
)

// RenderSafetyScorecard generates an ANSI terminal scorecard for a single ad evaluation.
func RenderSafetyScorecard(eval *SafetyEvaluation) string {
	var sb strings.Builder

	statusColor := colorGreen
	statusBadge := "[ELIGIBLE FOR AUCTION]"
	if !eval.Allowed {
		statusColor = colorRed
		statusBadge = "[" + string(eval.Verdict) + "]"
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s┌────────────────────────────────────────────────────────────────────────┐%s\n", colorBlue, colorReset))
	sb.WriteString(fmt.Sprintf("%s│ %s%-70s%s%s │%s\n", colorBlue, colorBold, "  AGENTAD BRAND SAFETY & COMPETITOR EXCLUSION SCORECARD", colorReset, colorBlue, colorReset))
	sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────┤%s\n", colorBlue, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s Query Context:   %-52s %s│%s\n", colorBlue, colorReset, truncate(eval.Query, 52), colorBlue, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s Candidate Ad:    %-52s %s│%s\n", colorBlue, colorReset, fmt.Sprintf("%s (%s)", eval.Advertiser, eval.AdID), colorBlue, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s Safety Tier:     %-52s %s│%s\n", colorBlue, colorReset, eval.Tier, colorBlue, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s Safety Score:    %-52s %s│%s\n", colorBlue, colorReset, fmt.Sprintf("%.1f / 100.0", eval.SafetyScore), colorBlue, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s Alignment Score: %-52s %s│%s\n", colorBlue, colorReset, fmt.Sprintf("%.2f (Min threshold: 0.40)", eval.AlignmentScore), colorBlue, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s Verdict:         %s%-52s%s %s│%s\n", colorBlue, colorReset, statusColor, statusBadge, colorReset, colorBlue, colorReset))

	if eval.MatchedCompetitor != "" {
		sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────┤%s\n", colorBlue, colorReset))
		sb.WriteString(fmt.Sprintf("%s│%s %sCompetitor Conflict Detected:%s %-39s %s│%s\n", colorBlue, colorReset, colorYellow, colorReset, eval.MatchedCompetitor, colorBlue, colorReset))
	}

	if len(eval.Violations) > 0 {
		sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────┤%s\n", colorBlue, colorReset))
		sb.WriteString(fmt.Sprintf("%s│%s %sPolicy Violations:%s                                                   %s│%s\n", colorBlue, colorReset, colorRed, colorReset, colorBlue, colorReset))
		for _, v := range eval.Violations {
			sb.WriteString(fmt.Sprintf("%s│%s   • %-65s %s│%s\n", colorBlue, colorGray, truncate(v, 65), colorBlue, colorReset))
		}
	}

	sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────┤%s\n", colorBlue, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s Governance Note: %-52s %s│%s\n", colorBlue, colorReset, truncate(eval.Reason, 52), colorBlue, colorReset))
	sb.WriteString(fmt.Sprintf("%s└────────────────────────────────────────────────────────────────────────┘%s\n", colorBlue, colorReset))

	return sb.String()
}

// RenderSafetyMermaid generates a Mermaid decision flowchart tracing the safety gates.
func RenderSafetyMermaid(eval *SafetyEvaluation) string {
	verdictNode := "Eligible[✅ Eligible for RTB Auction]"
	if !eval.Allowed {
		verdictNode = fmt.Sprintf("Blocked[❌ %s: %s]", eval.Verdict, eval.Reason)
	}

	return fmt.Sprintf("```mermaid\ngraph TD\n"+
		"  Query[\"Agent Query: %s\"] --> Gate1{\"Gate 1: Brand Safety<br/>Score: %.1f/100\"}\n"+
		"  Gate1 -->|Tier: %s| Gate2{\"Gate 2: Category Blocklist\"}\n"+
		"  Gate2 --> Gate3{\"Gate 3: Competitor Exclusion\"}\n"+
		"  Gate3 --> Gate4{\"Gate 4: Semantic Alignment<br/>Score: %.2f\"}\n"+
		"  Gate4 --> %s\n```",
		escapeMermaid(truncate(eval.Query, 35)), eval.SafetyScore, eval.Tier, eval.AlignmentScore, verdictNode)
}

// RenderMarkdownReport generates a GitHub-Flavored Markdown summary report.
func RenderMarkdownReport(evals []*SafetyEvaluation, summary *SafetySummary) string {
	var sb strings.Builder

	sb.WriteString("# 🛡️ AgentAd Brand Safety & Competitor Exclusion Report\n\n")
	sb.WriteString(fmt.Sprintf("**Generated At:** `%s`  \n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Total Candidates Evaluated:** `%d`  \n", summary.TotalEvaluated))
	sb.WriteString(fmt.Sprintf("**Eligible for Auction:** `%d` (`%.1f%%`)  \n", summary.PassedCount, float64(summary.PassedCount)/float64(summary.TotalEvaluated)*100.0))
	sb.WriteString(fmt.Sprintf("**Unsafe Context Blocks:** `%d`  \n", summary.UnsafeBlocked))
	sb.WriteString(fmt.Sprintf("**Competitor Conflict Blocks:** `%d`  \n", summary.CompetitorBlocked))
	sb.WriteString(fmt.Sprintf("**Irrelevant / Low-Alignment Blocks:** `%d`  \n\n", summary.IrrelevantBlocked))

	sb.WriteString("## 📊 Brand Safety Evaluation Matrix\n\n")
	sb.WriteString("| Query Intent | Candidate Ad | Safety Score | Alignment | Tier | Verdict | Status |\n")
	sb.WriteString("|:---|:---|:---:|:---:|:---:|:---|:---:|\n")

	for _, e := range evals {
		statusBadge := "✅ ELIGIBLE"
		if !e.Allowed {
			statusBadge = "🛑 BLOCKED"
		}
		sb.WriteString(fmt.Sprintf("| %s | **%s** | `%.1f` | `%.2f` | `%s` | `%s` | %s |\n",
			truncate(e.Query, 32), e.Advertiser, e.SafetyScore, e.AlignmentScore, e.Tier, e.Verdict, statusBadge))
	}
	sb.WriteString("\n")

	sb.WriteString("## 🚨 Violations & Governance Log\n\n")
	for _, e := range evals {
		if !e.Allowed {
			sb.WriteString(fmt.Sprintf("### %s (`%s`)\n", e.Advertiser, e.AdID))
			sb.WriteString(fmt.Sprintf("- **Query:** %s\n", e.Query))
			sb.WriteString(fmt.Sprintf("- **Verdict:** `%s`\n", e.Verdict))
			sb.WriteString(fmt.Sprintf("- **Reason:** %s\n", e.Reason))
			if len(e.Violations) > 0 {
				sb.WriteString(fmt.Sprintf("- **Violations:** %s\n", strings.Join(e.Violations, "; ")))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// RenderJSON serializes evaluations to formatted JSON.
func RenderJSON(evals []*SafetyEvaluation) (string, error) {
	bytes, err := json.MarshalIndent(evals, "", "  ")
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

func escapeMermaid(s string) string {
	return strings.ReplaceAll(s, "\"", "'")
}

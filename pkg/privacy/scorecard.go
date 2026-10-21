package privacy

import (
	"fmt"
	"strings"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiBlue   = "\033[34m"
	ansiMagenta= "\033[35m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
)

// RenderPrivacyScorecard formats a single evaluation into an ANSI terminal scorecard.
func RenderPrivacyScorecard(eval *PrivacyEvaluationResult) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("%s%s=== AGENTAD (AAP) ZERO-PII & DIFFERENTIAL PRIVACY SCORECARD ===%s\n", ansiBold, ansiMagenta, ansiReset))
	b.WriteString(fmt.Sprintf("Session ID:       %s\n", eval.SessionID))
	b.WriteString(fmt.Sprintf("Privacy Score:    %s%.1f / 100.0%s\n", getScoreColor(eval.PrivacyScore), eval.PrivacyScore, ansiReset))

	statusColor := ansiGreen
	statusText := "ALLOWED / PRIVACY PRESERVED"
	if eval.BudgetExhausted {
		statusColor = ansiRed
		statusText = "BLOCKED / PRIVACY BUDGET EXHAUSTED"
	}
	b.WriteString(fmt.Sprintf("Status:           %s%s%s\n", statusColor, statusText, ansiReset))
	b.WriteString(fmt.Sprintf("Budget (Epsilon): Spent: %.2f | Cost: %.2f | Remaining: %.2f\n", eval.EpsilonCost, eval.EpsilonCost, eval.RemainingBudget))
	b.WriteString(fmt.Sprintf("Generalization:   %s[%s]%s (K-Anonymity Cluster)\n", ansiCyan, eval.GeneralizationCategory, ansiReset))
	b.WriteString(fmt.Sprintf("Audit Hash:       %s%s%s\n", ansiGray, eval.AuditHash[:16]+"...", ansiReset))
	b.WriteString("──────────────────────────────────────────────────────────────────────────────────\n")

	b.WriteString(fmt.Sprintf("Original Query:   %s%s%s\n", ansiGray, eval.OriginalQuery, ansiReset))
	b.WriteString(fmt.Sprintf("Sanitized Query:  %s%s%s\n", ansiBold, eval.SanitizedQuery, ansiReset))

	if len(eval.RedactedEntities) > 0 {
		b.WriteString(fmt.Sprintf("\n%sINTERCEPTED & SCRUBBED PII ENTITIES (%d):%s\n", ansiYellow, len(eval.RedactedEntities), ansiReset))
		for i, e := range eval.RedactedEntities {
			b.WriteString(fmt.Sprintf("  [%d] %s[%s]%s %s -> %s%s%s (%s)\n",
				i+1, ansiRed, e.Category, ansiReset, e.Description, ansiGreen, e.Mask, ansiReset, e.Severity))
		}
	} else {
		b.WriteString(fmt.Sprintf("\n%s✔ CLEAN INTENT: No raw PII or secret credentials detected.%s\n", ansiGreen, ansiReset))
	}

	b.WriteString("──────────────────────────────────────────────────────────────────────────────────\n\n")
	return b.String()
}

// RenderPrivacyMermaid outputs a Mermaid flowchart diagram visualizing the privacy pipeline.
func RenderPrivacyMermaid(eval *PrivacyEvaluationResult) string {
	var b strings.Builder
	b.WriteString("```mermaid\n")
	b.WriteString("flowchart TD\n")
	b.WriteString("    UserIntent[\"Raw User Prompt / Intent Query\"] --> Redactor[\"Zero-PII Regex & Luhn Redactor\"]\n")

	if len(eval.RedactedEntities) > 0 {
		b.WriteString(fmt.Sprintf("    Redactor -->|\"Scrubbed %d Entities\"| Sanitized[\"Sanitized Intent: %s\"]\n",
			len(eval.RedactedEntities), escapeMermaid(eval.SanitizedQuery)))
	} else {
		b.WriteString("    Redactor -->|\"Zero PII Detected\"| Sanitized[\"Clean Query\"]\n")
	}

	b.WriteString("    Sanitized --> KAnon[\"K-Anonymity Generalizer\"]\n")
	b.WriteString(fmt.Sprintf("    KAnon --> Taxon[\"Taxonomy Bucket: %s\"]\n", eval.GeneralizationCategory))
	b.WriteString("    Taxon --> DP[\"Laplace DP Noise Perturbation (ε=1.0)\"]\n")
	b.WriteString(fmt.Sprintf("    DP --> Auction[\"Safe RTB Auction Floor (Privacy Score: %.1f)\"]\n", eval.PrivacyScore))
	b.WriteString("```\n")
	return b.String()
}

// GenerateMarkdownReport creates a GitHub Flavored Markdown summary report for multiple evaluations.
func GenerateMarkdownReport(evals []*PrivacyEvaluationResult) string {
	var b strings.Builder

	b.WriteString("## 🛡️ AgentAd (AAP) Zero-PII & Differential Privacy Evaluation Report\n\n")
	b.WriteString("| Session ID | Privacy Score | PII Scrubbed | Generalized Category | Epsilon Remaining | Status |\n")
	b.WriteString("| :--- | :---: | :---: | :--- | :---: | :---: |\n")

	for _, ev := range evals {
		status := "✅ ALLOWED"
		if ev.BudgetExhausted {
			status = "❌ BUDGET_EXHAUSTED"
		}
		b.WriteString(fmt.Sprintf("| `%s` | `%.1f / 100` | `%d` | `%s` | `%.2f` | %s |\n",
			ev.SessionID, ev.PrivacyScore, ev.TotalPIIFound, ev.GeneralizationCategory, ev.RemainingBudget, status))
	}

	b.WriteString("\n### Intercepted PII & Secret Redaction Log\n\n")
	for _, ev := range evals {
		if len(ev.RedactedEntities) > 0 {
			b.WriteString(fmt.Sprintf("#### Session `%s`\n\n", ev.SessionID))
			b.WriteString("| Category | Severity | Description | Mask Token |\n")
			b.WriteString("| :--- | :---: | :--- | :--- |\n")
			for _, ent := range ev.RedactedEntities {
				b.WriteString(fmt.Sprintf("| `%s` | **%s** | %s | `%s` |\n",
					ent.Category, ent.Severity, ent.Description, ent.Mask))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func getScoreColor(score float64) string {
	if score >= 90.0 {
		return ansiGreen
	} else if score >= 70.0 {
		return ansiYellow
	}
	return ansiRed
}

func escapeMermaid(s string) string {
	clean := strings.ReplaceAll(s, "\"", "'")
	if len(clean) > 35 {
		return clean[:35] + "..."
	}
	return clean
}

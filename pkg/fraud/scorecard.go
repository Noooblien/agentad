package fraud

import (
	"fmt"
	"strings"
)

// ANSI terminal color constants
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiRed     = "\033[31m"
	ansiCyan    = "\033[36m"
	ansiMagenta = "\033[35m"
	ansiBgGreen = "\033[42m\033[30m"
	ansiBgRed   = "\033[41m\033[37m"
	ansiBgYellow= "\033[43m\033[30m"
)

// RenderFraudScorecard formats the traffic verdict into an ANSI terminal scorecard.
func RenderFraudScorecard(v *TrafficVerdict) string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%s%s┌─────────────────────────────────────────────────────────────────────────────┐%s\n", ansiBold, ansiMagenta, ansiReset))
	b.WriteString(fmt.Sprintf("%s%s│    AGENTAD (AAP) // TRAFFIC SCRUBBER & CLICK-FRAUD DEFENSE REPORT           │%s\n", ansiBold, ansiMagenta, ansiReset))
	b.WriteString(fmt.Sprintf("%s%s└─────────────────────────────────────────────────────────────────────────────┘%s\n", ansiBold, ansiMagenta, ansiReset))

	b.WriteString(fmt.Sprintf("  %sQuery Text:%s          \"%s\"\n", ansiBold, ansiReset, truncate(v.IntentQuery, 60)))
	b.WriteString(fmt.Sprintf("  %sAgent ID:%s            %s\n", ansiBold, ansiReset, v.AgentID))
	b.WriteString(fmt.Sprintf("  %sPublisher ID:%s        %s\n", ansiBold, ansiReset, v.PublisherID))
	b.WriteString(fmt.Sprintf("  %sEvaluation Latency:%s  %.3f ms\n", ansiBold, ansiReset, v.ExecutionLatencyMs))

	// Score & Risk Level
	var scoreColor, badgeColor string
	switch v.RiskLevel {
	case RiskClean:
		scoreColor = ansiGreen
		badgeColor = ansiBgGreen
	case RiskSuspicious:
		scoreColor = ansiYellow
		badgeColor = ansiBgYellow
	case RiskFraudulent:
		scoreColor = ansiRed
		badgeColor = ansiBgRed
	}

	b.WriteString(fmt.Sprintf("\n  %sOverall Fraud Score:%s %s%s%.2f / 1.00%s\n", ansiBold, ansiReset, ansiBold, scoreColor, v.OverallFraudScore, ansiReset))
	b.WriteString(fmt.Sprintf("  %sRisk Severity:%s       %s%s [%s] %s\n", ansiBold, ansiReset, badgeColor, ansiBold, v.RiskLevel, ansiReset))
	b.WriteString(fmt.Sprintf("  %sEnforced Action:%s     %s%s%s%s\n", ansiBold, ansiReset, ansiBold, scoreColor, v.ActionTaken, ansiReset))

	// Signal Metrics
	b.WriteString(fmt.Sprintf("\n  %sSignal Metrics:%s\n", ansiBold, ansiReset))
	b.WriteString(fmt.Sprintf("    • Shannon Entropy:     %.2f bits/char %s(Natural range: 3.5 - 4.8)%s\n", v.EntropyScore, ansiDim, ansiReset))
	b.WriteString(fmt.Sprintf("    • Burst Velocity:      %.1f requests/min\n", v.BurstVelocityRPM))
	b.WriteString(fmt.Sprintf("    • Repetition Ratio:    %.0f%% duplicate intents\n", v.RepetitionRatio*100))

	// Tripped Vectors
	b.WriteString(fmt.Sprintf("\n  %sTripped Fraud Vectors:%s", ansiBold, ansiReset))
	if len(v.TriggeredVectors) == 0 {
		b.WriteString(fmt.Sprintf(" %sNone (Clean)%s\n", ansiGreen, ansiReset))
	} else {
		b.WriteString("\n")
		for _, vec := range v.TriggeredVectors {
			b.WriteString(fmt.Sprintf("    %s⚠ %s%s\n", ansiRed, vec, ansiReset))
		}
	}

	// Explanatory Triage Notes
	b.WriteString(fmt.Sprintf("\n  %sTriage Rationale:%s\n", ansiBold, ansiReset))
	for _, r := range v.Reasons {
		b.WriteString(fmt.Sprintf("    • %s%s%s\n", ansiDim, r, ansiReset))
	}

	b.WriteString("\n")
	return b.String()
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// RenderFraudMermaid generates a Mermaid flowchart diagram visualizing the traffic scrubbing pipeline.
func RenderFraudMermaid(v *TrafficVerdict) string {
	var b strings.Builder
	b.WriteString("```mermaid\n")
	b.WriteString("flowchart TD\n")
	b.WriteString("  subgraph Ingestion[\"Incoming Agent Ad Traffic\"]\n")
	b.WriteString(fmt.Sprintf("    REQ[\"Query: %s<br/>Agent: %s\"]\n", truncate(v.IntentQuery, 30), v.AgentID))
	b.WriteString("  end\n\n")

	b.WriteString("  subgraph Analysis[\"Multi-Vector Fraud & Leakage Scrubber\"]\n")
	b.WriteString(fmt.Sprintf("    ENT[\"Entropy Analyzer<br/>%.2f bits/char\"]\n", v.EntropyScore))
	b.WriteString(fmt.Sprintf("    VEL[\"Velocity Limiter<br/>%.1f RPM\"]\n", v.BurstVelocityRPM))
	b.WriteString("    KEY[\"Keyword Stacking Detector\"]\n")
	b.WriteString("    LEAK[\"Prompt Leakage Probe Scanner\"]\n")
	b.WriteString("    CAN[\"Canary Token Trap\"]\n")
	b.WriteString("  end\n\n")

	b.WriteString("  REQ --> ENT\n")
	b.WriteString("  REQ --> VEL\n")
	b.WriteString("  REQ --> KEY\n")
	b.WriteString("  REQ --> LEAK\n")
	b.WriteString("  REQ --> CAN\n\n")

	b.WriteString("  subgraph Verdict[\"Traffic Gate & Escrow Arbiter\"]\n")
	b.WriteString(fmt.Sprintf("    SCORE[\"Composite Fraud Score: %.2f / 1.00<br/>Risk: %s\"]\n", v.OverallFraudScore, v.RiskLevel))
	b.WriteString(fmt.Sprintf("    ACT{\"Action: %s\"}\n", v.ActionTaken))
	b.WriteString("  end\n\n")

	b.WriteString("  ENT --> SCORE\n")
	b.WriteString("  VEL --> SCORE\n")
	b.WriteString("  KEY --> SCORE\n")
	b.WriteString("  LEAK --> SCORE\n")
	b.WriteString("  CAN --> SCORE\n")
	b.WriteString("  SCORE --> ACT\n\n")

	b.WriteString("  %% Styling\n")
	b.WriteString("  classDef clean fill:#e6f4ea,stroke:#34a853,stroke-width:2px;\n")
	b.WriteString("  classDef sus fill:#fef7e0,stroke:#f9ab00,stroke-width:2px;\n")
	b.WriteString("  classDef fraud fill:#fce8e6,stroke:#ea4335,stroke-width:3px;\n")

	switch v.RiskLevel {
	case RiskClean:
		b.WriteString("  class SCORE,ACT clean;\n")
	case RiskSuspicious:
		b.WriteString("  class SCORE,ACT sus;\n")
	case RiskFraudulent:
		b.WriteString("  class SCORE,ACT fraud;\n")
	}

	b.WriteString("```\n")
	return b.String()
}

package bandit

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiCyan   = "\033[36m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiBlue   = "\033[34m"
	ansiRed    = "\033[31m"
	ansiDim    = "\033[2m"
)

// RenderBanditScorecard prints an ANSI terminal dashboard summarizing bandit simulation performance.
func RenderBanditScorecard(summary *BatchSimulationSummary) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s%s=== BAYESIAN MULTI-ARMED BANDIT SIMULATION SCORECARD ===%s\n", ansiBold, ansiCyan, ansiReset))
	sb.WriteString(fmt.Sprintf("Scenario:        %s%s%s (Category: %s)\n", ansiBold, summary.ScenarioName, ansiReset, summary.Category))
	sb.WriteString(fmt.Sprintf("Total Rounds:    %d\n", summary.TotalRounds))
	sb.WriteString(fmt.Sprintf("Total Impr:      %d | Total Clicks: %s%d%s | Total Conversions: %s%d%s\n",
		summary.TotalImpressions, ansiGreen, summary.TotalClicks, ansiReset, ansiGreen, summary.TotalConversions, ansiReset))
	sb.WriteString(fmt.Sprintf("Average CTR:     %s%.2f%%%s\n", ansiGreen, summary.AverageCTR*100.0, ansiReset))
	sb.WriteString(fmt.Sprintf("Cumulative Regret: %s$%.2f CPM%s\n", ansiYellow, summary.CumulativeRegret, ansiReset))
	sb.WriteString(strings.Repeat("─", 84) + "\n")

	sb.WriteString(fmt.Sprintf("%-16s | %-18s | %6s | %6s | %7s | %7s | %6s | %7s\n",
		"CAMPAIGN ID", "ADVERTISER", "IMPR", "CLICKS", "EMP CTR", "CVR", "ALPHA", "BETA"))
	sb.WriteString(strings.Repeat("─", 84) + "\n")

	// Sort arms by impressions descending
	sortedArms := make([]*ArmState, len(summary.Arms))
	copy(sortedArms, summary.Arms)
	sort.Slice(sortedArms, func(i, j int) bool {
		return sortedArms[i].Impressions > sortedArms[j].Impressions
	})

	for _, arm := range sortedArms {
		statusColor := ansiReset
		if arm.InColdStart {
			statusColor = ansiDim
		} else if arm.EmpiricalCTR > 0.15 {
			statusColor = ansiGreen
		}

		sb.WriteString(fmt.Sprintf("%-16s | %-18s | %6d | %6d | %s%6.2f%%%s | %6.2f%% | %6.1f | %7.1f\n",
			arm.CampaignID,
			truncate(arm.AdvertiserName, 18),
			arm.Impressions,
			arm.Clicks,
			statusColor,
			arm.EmpiricalCTR*100.0,
			ansiReset,
			arm.EmpiricalCVR*100.0,
			arm.Alpha,
			arm.Beta,
		))
	}
	sb.WriteString(strings.Repeat("─", 84) + "\n")

	// Print sample rounds progression
	if len(summary.Rounds) > 0 {
		sb.WriteString(fmt.Sprintf("\n%s%sROUND SELECTION PROGRESSION:%s\n", ansiBold, ansiBlue, ansiReset))
		sb.WriteString(fmt.Sprintf("%-8s | %-16s | %-12s | %10s | %10s | %10s\n",
			"ROUND", "WINNER ARM", "MODE", "SAMPLE CTR", "EXPECT CTR", "eCPM"))
		sb.WriteString(strings.Repeat("─", 80) + "\n")
		for idx, r := range summary.Rounds {
			modeColor := ansiReset
			switch r.Mode {
			case ModeColdStart:
				modeColor = ansiDim
			case ModeExploration:
				modeColor = ansiYellow
			case ModeExploitation:
				modeColor = ansiGreen
			}

			sb.WriteString(fmt.Sprintf("Round %-2d | %-16s | %s%-12s%s | %9.2f%% | %9.2f%% | $%-9.2f\n",
				idx+1,
				r.SelectedArmID,
				modeColor,
				r.Mode,
				ansiReset,
				r.SampledScore*100.0,
				r.ExpectedCTR*100.0,
				r.EffectiveCPM,
			))
		}
	}

	return sb.String()
}

// GenerateBanditMermaid renders a Mermaid diagram visualizing the bandit decision loop and state updates.
func GenerateBanditMermaid(summary *BatchSimulationSummary) string {
	var sb strings.Builder

	sb.WriteString("```mermaid\n")
	sb.WriteString("flowchart TD\n")
	sb.WriteString("    subgraph Input [1. Candidate Intake]\n")
	sb.WriteString("        A[Agent Search Query & Context] --> B[Retrieve Eligible Sponsored Tools]\n")
	sb.WriteString("        B --> C[Candidate Arms Pool]\n")
	sb.WriteString("    end\n\n")

	sb.WriteString("    subgraph Bandit [2. Bayesian Exploration Engine]\n")
	sb.WriteString("        C --> D{Check Cold-Start Threshold}\n")
	sb.WriteString("        D -->|Impr < N| E[Mode: COLD_START Prior Sampling]\n")
	sb.WriteString("        D -->|Impr >= N| F{Algorithm Selection}\n")
	sb.WriteString("        F -->|Thompson Sampling| G[\"Sample θ ~ Beta(α, β)\"]\n")
	sb.WriteString("        F -->|UCB1| H[\"Score = μ + c √(2 ln N / n)\"]\n")
	sb.WriteString("        F -->|Epsilon-Greedy| I[\"Explore with prob ε, else exploit\"]\n")
	sb.WriteString("        E --> J[Rank by eCPM = Score × Bid × Relevance]\n")
	sb.WriteString("        G --> J\n")
	sb.WriteString("        H --> J\n")
	sb.WriteString("        I --> J\n")
	sb.WriteString("        J --> K[Select Top Ranked Winner Arm]\n")
	sb.WriteString("    end\n\n")

	sb.WriteString("    subgraph Execution [3. Delivery & Telemetry Feedback]\n")
	sb.WriteString("        K --> L[Inject Sponsored Tool Call into Agent Session]\n")
	sb.WriteString("        L --> M{Agent Interaction Outcome}\n")
	sb.WriteString("        M -->|Click / Tool Call Invocation| N[\"Update Posterior: α ← α + 1\"]\n")
	sb.WriteString("        M -->|Agent Goal Conversion| O[\"Update Posterior: α ← α + 2\"]\n")
	sb.WriteString("        M -->|Ignored / Discarded| P[\"Update Posterior: β ← β + 1\"]\n")
	sb.WriteString("        N --> Q[Apply Time-Decay Discounting]\n")
	sb.WriteString("        O --> Q\n")
	sb.WriteString("        P --> Q\n")
	sb.WriteString("        Q --> C\n")
	sb.WriteString("    end\n")
	sb.WriteString("```\n")

	return sb.String()
}

// GenerateMarkdownReport returns a comprehensive Markdown report comparing multiple bandit scenarios.
func GenerateMarkdownReport(summaries []*BatchSimulationSummary) string {
	var sb strings.Builder

	sb.WriteString("# Contextual Multi-Armed Bandit & Bayesian Exploration Engine Report\n\n")
	sb.WriteString("This report summarizes the performance of Bayesian Thompson Sampling and contextual bandit exploration across agent discovery scenarios.\n\n")

	sb.WriteString("## Scenario Comparison Summary\n\n")
	sb.WriteString("| Scenario | Category | Total Rounds | Total Clicks | Avg CTR | Cumulative Regret | Top Winning Arm |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")

	for _, s := range summaries {
		topArm := "N/A"
		var maxImpr int
		for _, arm := range s.Arms {
			if arm.Impressions > maxImpr {
				maxImpr = arm.Impressions
				topArm = fmt.Sprintf("%s (%d imp, %.1f%% CTR)", arm.AdvertiserName, arm.Impressions, arm.EmpiricalCTR*100.0)
			}
		}

		sb.WriteString(fmt.Sprintf("| **%s** | `%s` | %d | %d | %.2f%% | $%.2f CPM | %s |\n",
			s.ScenarioName,
			s.Category,
			s.TotalRounds,
			s.TotalClicks,
			s.AverageCTR*100.0,
			s.CumulativeRegret,
			topArm,
		))
	}

	sb.WriteString("\n## Arm Distribution Breakdown\n\n")
	for _, s := range summaries {
		sb.WriteString(fmt.Sprintf("### Scenario: %s (`%s`)\n\n", s.ScenarioName, s.Category))
		sb.WriteString("| Campaign ID | Advertiser | Impressions | Clicks | Conversions | Empirical CTR | Beta (α / β) |\n")
		sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")

		sortedArms := make([]*ArmState, len(s.Arms))
		copy(sortedArms, s.Arms)
		sort.Slice(sortedArms, func(i, j int) bool {
			return sortedArms[i].Impressions > sortedArms[j].Impressions
		})

		for _, arm := range sortedArms {
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %d | %d | %d | **%.2f%%** | α=%.1f, β=%.1f |\n",
				arm.CampaignID, arm.AdvertiserName, arm.Impressions, arm.Clicks, arm.Conversions, arm.EmpiricalCTR*100.0, arm.Alpha, arm.Beta))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

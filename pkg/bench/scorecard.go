package bench

import (
	"fmt"
	"strings"
	"time"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiGreen  = "\033[32m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
)

// RenderTerminalScorecard prints a formatted ANSI box displaying benchmark metrics.
func RenderTerminalScorecard(summary *SuiteSummary) string {
	var sb strings.Builder

	overallBadge := fmt.Sprintf("%s%s[ALL SUITES PASSED]%s", ansiBold, ansiGreen, ansiReset)
	if !summary.OverallPassed {
		overallBadge = fmt.Sprintf("%s%s[SLA BREACH / DEFENSE FAILURE]%s", ansiBold, ansiRed, ansiReset)
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s╭────────────────────────────────────────────────────────────────────────────────────────╮%s\n", ansiCyan, ansiReset))
	sb.WriteString(fmt.Sprintf("%s│  ⚡ AGENTAD (AAP) EMPIRICAL VERIFICATION & BENCHMARK SUITE                             │%s\n", ansiBold, ansiReset))
	sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────────────────────┤%s\n", ansiCyan, ansiReset))
	sb.WriteString(fmt.Sprintf("│  Timestamp:       %-35s Total Duration: %-12s │\n",
		summary.Timestamp.Format(time.RFC3339), summary.TotalDuration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("│  Suites Evaluated: %-2d (Passed: %-2d, Failed: %-2d)         Overall Status: %-18s │\n",
		summary.TotalSuites, summary.PassedSuites, summary.FailedSuites, overallBadge))
	sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────────────────────┤%s\n", ansiCyan, ansiReset))
	sb.WriteString(fmt.Sprintf("│  %-34s │ %-8s │ %-7s │ %-7s │ %-7s │ %-6s │\n",
		"Target Benchmark Suite", "Throughput", "P50", "P95", "P99", "Status"))
	sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────┼──────────┼─────────┼─────────┼─────────┼────────┤%s\n", ansiCyan, ansiReset))

	for _, r := range summary.Reports {
		status := fmt.Sprintf("%sPASS%s", ansiGreen, ansiReset)
		if !r.Passed {
			status = fmt.Sprintf("%sFAIL%s", ansiRed, ansiReset)
		}

		throughputStr := fmt.Sprintf("%.0f/s", r.ThroughputOpsSec)
		p50Str := fmt.Sprintf("%.3fms", r.Latency.P50)
		p95Str := fmt.Sprintf("%.3fms", r.Latency.P95)
		p99Str := fmt.Sprintf("%.3fms", r.Latency.P99)

		name := r.Name
		if len(name) > 34 {
			name = name[:31] + "..."
		}

		sb.WriteString(fmt.Sprintf("│  %-34s │ %-8s │ %-7s │ %-7s │ %-7s │  [%s]  │\n",
			name, throughputStr, p50Str, p95Str, p99Str, status))
	}

	sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────────────────────┤%s\n", ansiCyan, ansiReset))
	sb.WriteString("│  Empirical SLA & Defense Guarantees:                                                   │\n")
	for _, r := range summary.Reports {
		if r.Category == "security_defenses" {
			sb.WriteString(fmt.Sprintf("│  • %-30s Recall: %s%.2f%%%s (FP Rate: %.2f%%)                         │\n",
				r.Name, ansiGreen, r.RecallRate*100, ansiReset, r.FalsePositiveRate*100))
		} else {
			sb.WriteString(fmt.Sprintf("│  • %-30s SLA: <%-5.1fms (P99: %-7.3fms, Breaches: %-2d)               │\n",
				r.Name, r.SLABudgetMs, r.Latency.P99, r.SLAViolations))
		}
	}
	sb.WriteString(fmt.Sprintf("%s╰────────────────────────────────────────────────────────────────────────────────────────╯%s\n", ansiCyan, ansiReset))

	return sb.String()
}

// RenderMarkdown produces a GitHub-flavored Markdown table summarizing verified benchmarks.
func RenderMarkdown(summary *SuiteSummary) string {
	var sb strings.Builder

	sb.WriteString("### AgentAd (AAP) Empirical Benchmark Verification\n\n")
	sb.WriteString(fmt.Sprintf("Verified on `%s UTC` across **%d benchmark suites** in **%s**.\n\n",
		summary.Timestamp.Format("2006-01-02 15:04:05"), summary.TotalSuites, summary.TotalDuration.Round(time.Millisecond)))

	sb.WriteString("| Benchmark Suite | Total Ops | Throughput | P50 Latency | P95 Latency | P99 Latency | SLA / Recall | Status |\n")
	sb.WriteString("|:----------------|:---------:|:----------:|:-----------:|:-----------:|:-----------:|:------------:|:------:|\n")

	for _, r := range summary.Reports {
		status := "✅ PASS"
		if !r.Passed {
			status = "❌ FAIL"
		}

		slaOrRecall := fmt.Sprintf("< %.1f ms", r.SLABudgetMs)
		if r.Category == "security_defenses" {
			slaOrRecall = fmt.Sprintf("%.1f%% recall", r.RecallRate*100)
		}

		sb.WriteString(fmt.Sprintf("| **%s** | %d | %.0f ops/s | %.3f ms | %.3f ms | %.3f ms | %s | %s |\n",
			r.Name, r.TotalOperations, r.ThroughputOpsSec, r.Latency.P50, r.Latency.P95, r.Latency.P99, slaOrRecall, status))
	}

	sb.WriteString("\n*Zero external dependencies. Benchmarked natively on Go runtime.*\n")
	return sb.String()
}

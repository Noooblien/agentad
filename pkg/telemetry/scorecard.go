package telemetry

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// RenderTelemetryScorecard formats the telemetry summary into a clean ANSI terminal scorecard.
func RenderTelemetryScorecard(s *TelemetrySummary) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("========================================================================================\n")
	sb.WriteString("           AGENT ADVERTISING PROTOCOL (AAP) - REAL-TIME TELEMETRY ENGINE                 \n")
	sb.WriteString("========================================================================================\n\n")

	sb.WriteString(fmt.Sprintf("Timestamp:               %s\n", s.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Total Requests:          %d\n", s.TotalRequests))
	sb.WriteString(fmt.Sprintf("Cleared Requests:        %d (%.1f%%)\n", s.ClearedRequests, s.ClearedRatePercent))
	sb.WriteString(fmt.Sprintf("Suppressed (Quiet Zone): %d\n", s.SuppressedRequests))
	sb.WriteString(fmt.Sprintf("Rejected (Threat/Fraud): %d\n", s.RejectedRequests))
	sb.WriteString(fmt.Sprintf("SLA Compliance (<50ms):  %.1f%%  [Target: >99.0%%]\n", s.SLAPassRatePercent))
	sb.WriteString("----------------------------------------------------------------------------------------\n")

	sb.WriteString("FINANCIAL & REVENUE METRICS:\n")
	sb.WriteString(fmt.Sprintf("  Total Revenue:         $%.6f USD\n", s.TotalRevenueUSD))
	sb.WriteString(fmt.Sprintf("  Publisher Share (70%%): $%.6f USD\n", s.PublisherRevenueUSD))
	sb.WriteString(fmt.Sprintf("  Agent Host Share (20%%): $%.6f USD\n", s.AgentRevenueUSD))
	sb.WriteString(fmt.Sprintf("  Protocol Treasury (10%%):$%.6f USD\n", s.TreasuryRevenueUSD))
	sb.WriteString(fmt.Sprintf("  Avg Clearing CPM:      $%.2f USD\n", s.AverageClearingPriceCPM))
	sb.WriteString("----------------------------------------------------------------------------------------\n")

	sb.WriteString("SECURITY, PRIVACY & GOVERNANCE COUNTERS:\n")
	sb.WriteString(fmt.Sprintf("  Prompt Injections Deflected: %d\n", s.PromptInjectionsBlocked))
	sb.WriteString(fmt.Sprintf("  Synthetic / Fraud Probes:    %d\n", s.FraudDetections))
	sb.WriteString(fmt.Sprintf("  PII / Credential Redactions: %d\n", s.PIIEntitiesRedacted))
	sb.WriteString(fmt.Sprintf("  Cognitive Quiet-Zone Drops:  %d\n", s.QuietZoneSuppressed))
	sb.WriteString("----------------------------------------------------------------------------------------\n")

	sb.WriteString("END-TO-END LATENCY PERCENTILES (ms):\n")
	sb.WriteString(fmt.Sprintf("  Min: %.2f ms | Avg: %.2f ms | P50: %.2f ms | P90: %.2f ms | P95: %.2f ms | P99: %.2f ms | Max: %.2f ms\n",
		s.OverallLatencyMs.Min, s.OverallLatencyMs.Average, s.OverallLatencyMs.P50,
		s.OverallLatencyMs.P90, s.OverallLatencyMs.P95, s.OverallLatencyMs.P99, s.OverallLatencyMs.Max))
	sb.WriteString("----------------------------------------------------------------------------------------\n")

	sb.WriteString("STAGE-BY-STAGE LATENCY PROFILES:\n")
	sb.WriteString(fmt.Sprintf("%-26s | %-8s | %-10s | %-10s | %-10s\n", "Pipeline Stage", "Calls", "P50 (ms)", "P95 (ms)", "Max (ms)"))
	sb.WriteString("---------------------------+----------+------------+------------+-----------\n")

	stageNames := make([]string, 0, len(s.StageLatenciesMs))
	for name := range s.StageLatenciesMs {
		stageNames = append(stageNames, name)
	}
	sort.Strings(stageNames)

	for _, name := range stageNames {
		st := s.StageLatenciesMs[name]
		sb.WriteString(fmt.Sprintf("%-26s | %-8d | %-10.3f | %-10.3f | %-10.3f\n",
			truncate(name, 26), st.Count, st.P50, st.P95, st.Max))
	}

	sb.WriteString("========================================================================================\n\n")
	return sb.String()
}

// RenderTelemetryMarkdown formats the telemetry metrics into Markdown format.
func RenderTelemetryMarkdown(s *TelemetrySummary) string {
	var sb strings.Builder

	sb.WriteString("# 📊 Agent Advertising Protocol (AAP) - Telemetry & Metrics Report\n\n")
	sb.WriteString(fmt.Sprintf("> **Generated:** `%s` | **SLA Compliance (<50ms):** `%.1f%%`\n\n",
		s.GeneratedAt.Format(time.RFC3339), s.SLAPassRatePercent))

	sb.WriteString("### ⚡ Execution & Traffic Overview\n\n")
	sb.WriteString("| Metric | Value | Proportion |\n")
	sb.WriteString("| :--- | :--- | :--- |\n")
	sb.WriteString(fmt.Sprintf("| **Total Pipeline Requests** | `%d` | 100.0%% |\n", s.TotalRequests))
	sb.WriteString(fmt.Sprintf("| **Cleared (Injected & Settled)** | `%d` | %.1f%% |\n", s.ClearedRequests, s.ClearedRatePercent))
	sb.WriteString(fmt.Sprintf("| **Suppressed (Quiet Zone / Fatigue)** | `%d` | %.1f%% |\n", s.SuppressedRequests, (float64(s.SuppressedRequests)/float64(s.TotalRequests))*100.0))
	sb.WriteString(fmt.Sprintf("| **Rejected (Fraud / Threat)** | `%d` | %.1f%% |\n", s.RejectedRequests, (float64(s.RejectedRequests)/float64(s.TotalRequests))*100.0))
	sb.WriteString(fmt.Sprintf("| **Average Clearing Price (CPM)** | `%.2f USD` | — |\n", s.AverageClearingPriceCPM))
	sb.WriteString("\n")

	sb.WriteString("### 💰 Programmatic Escrow Revenue Distribution\n\n")
	sb.WriteString("| Participant | Split | Cumulative Total |\n")
	sb.WriteString("| :--- | :--- | :--- |\n")
	sb.WriteString(fmt.Sprintf("| **Publisher Domain** | 70%% | `$%.6f USD` |\n", s.PublisherRevenueUSD))
	sb.WriteString(fmt.Sprintf("| **Agent Host Runtime** | 20%% | `$%.6f USD` |\n", s.AgentRevenueUSD))
	sb.WriteString(fmt.Sprintf("| **AAP Protocol Treasury** | 10%% | `$%.6f USD` |\n", s.TreasuryRevenueUSD))
	sb.WriteString(fmt.Sprintf("| **Gross Revenue** | 100%% | `$%.6f USD` |\n", s.TotalRevenueUSD))
	sb.WriteString("\n")

	sb.WriteString("### ⏱️ Stage Latency Percentiles (ms)\n\n")
	sb.WriteString("| Pipeline Stage | Invocations | P50 (ms) | P95 (ms) | Max (ms) |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")

	stageNames := make([]string, 0, len(s.StageLatenciesMs))
	for name := range s.StageLatenciesMs {
		stageNames = append(stageNames, name)
	}
	sort.Strings(stageNames)

	for _, name := range stageNames {
		st := s.StageLatenciesMs[name]
		sb.WriteString(fmt.Sprintf("| `%s` | %d | `%.3f` | `%.3f` | `%.3f` |\n",
			name, st.Count, st.P50, st.P95, st.Max))
	}
	sb.WriteString("\n")

	return sb.String()
}

// RenderTelemetryMermaid generates a Mermaid flowchart depicting telemetry data flow.
func RenderTelemetryMermaid(s *TelemetrySummary) string {
	var sb strings.Builder

	sb.WriteString("```mermaid\n")
	sb.WriteString("flowchart TD\n")
	sb.WriteString("    subgraph Pipeline [12-Stage AAP Pipeline]\n")
	sb.WriteString("        R[Incoming Agent Request] --> P1[Stage 1-4: Traffic, PII & Safety]\n")
	sb.WriteString("        P1 --> P2[Stage 5-8: Floor, RTB Auction & Pacing]\n")
	sb.WriteString("        P2 --> P3[Stage 9-12: Placement, AST Purify & Settlement]\n")
	sb.WriteString("    end\n\n")

	sb.WriteString("    subgraph Telemetry [Observability & Metrics Collector]\n")
	sb.WriteString("        T_Reg[In-Memory Prometheus Registry]\n")
	sb.WriteString(fmt.Sprintf("        T_Req[Requests Total: %d]\n", s.TotalRequests))
	sb.WriteString(fmt.Sprintf("        T_SLA[SLA Compliance: %.1f%%]\n", s.SLAPassRatePercent))
	sb.WriteString(fmt.Sprintf("        T_Rev[Total Revenue: $%.4f]\n", s.TotalRevenueUSD))
	sb.WriteString("    end\n\n")

	sb.WriteString("    subgraph Exporters [Prometheus / OpenTelemetry Exporters]\n")
	sb.WriteString("        E_Prom[\"/metrics (Prometheus Text)\"]\n")
	sb.WriteString("        E_JSON[\"/telemetry (JSON Snapshot)\"]\n")
	sb.WriteString("        E_CLI[\"agentad telemetry (Scorecard)\"]\n")
	sb.WriteString("    end\n\n")

	sb.WriteString("    P3 -.->|Observe| T_Reg\n")
	sb.WriteString("    T_Reg --> T_Req\n")
	sb.WriteString("    T_Reg --> T_SLA\n")
	sb.WriteString("    T_Reg --> T_Rev\n")
	sb.WriteString("    T_Reg ==> E_Prom\n")
	sb.WriteString("    T_Reg ==> E_JSON\n")
	sb.WriteString("    T_Reg ==> E_CLI\n")
	sb.WriteString("```\n")

	return sb.String()
}

func truncate(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen-1] + "…"
}

package tests

import (
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/telemetry"
)

func TestTelemetry_EndToEndWorkloadSuite(t *testing.T) {
	collector, summary := telemetry.RunTelemetryDemo()
	if summary.TotalRequests != 22 {
		t.Fatalf("expected 22 total evaluated requests, got %d", summary.TotalRequests)
	}
	if summary.ClearedRequests == 0 {
		t.Fatalf("expected cleared requests in demo summary")
	}
	if summary.SuppressedRequests == 0 {
		t.Fatalf("expected quiet zone suppressions")
	}
	if summary.SLAPassRatePercent < 95.0 {
		t.Fatalf("expected >=95%% SLA pass rate, got %f", summary.SLAPassRatePercent)
	}
	if summary.OverallLatencyMs.P95 > 50.0 {
		t.Fatalf("expected P95 latency <= 50ms SLA, got %f ms", summary.OverallLatencyMs.P95)
	}

	scorecard := telemetry.RenderTelemetryScorecard(summary)
	if !strings.Contains(scorecard, "REAL-TIME TELEMETRY ENGINE") {
		t.Fatalf("scorecard missing expected title")
	}

	md := telemetry.RenderTelemetryMarkdown(summary)
	if !strings.Contains(md, "Telemetry & Metrics Report") {
		t.Fatalf("markdown report missing expected header")
	}

	mermaid := telemetry.RenderTelemetryMermaid(summary)
	if !strings.Contains(mermaid, "flowchart TD") {
		t.Fatalf("mermaid diagram missing flowchart TD")
	}

	prom := collector.Registry().RenderPrometheus()
	if !strings.Contains(prom, telemetry.MetricRequestsTotal) {
		t.Fatalf("prometheus exposition missing metric: %s", telemetry.MetricRequestsTotal)
	}
	if !strings.Contains(prom, telemetry.MetricPipelineDurationMs) {
		t.Fatalf("prometheus exposition missing metric: %s", telemetry.MetricPipelineDurationMs)
	}
}

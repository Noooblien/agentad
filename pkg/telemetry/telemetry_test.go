package telemetry

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Noooblien/agentad/pkg/pipeline"
)

func TestCounter(t *testing.T) {
	c := &Counter{}
	if c.Get() != 0 {
		t.Fatalf("expected 0, got %f", c.Get())
	}
	c.Inc()
	c.Add(4.5)
	c.Add(-1.0) // negative should be ignored
	if c.Get() != 5.5 {
		t.Fatalf("expected 5.5, got %f", c.Get())
	}

	// Concurrency test
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc()
		}()
	}
	wg.Wait()

	if c.Get() != 55.5 {
		t.Fatalf("expected 55.5, got %f", c.Get())
	}
}

func TestGauge(t *testing.T) {
	g := &Gauge{}
	g.Set(10.0)
	g.Inc()
	g.Dec()
	g.Add(2.5)
	if g.Get() != 12.5 {
		t.Fatalf("expected 12.5, got %f", g.Get())
	}
}

func TestHistogram(t *testing.T) {
	h := NewHistogram([]float64{1.0, 5.0, 10.0, 50.0})
	for i := 1; i <= 100; i++ {
		h.Observe(float64(i))
	}

	summary := h.GetSummary()
	if summary.Count != 100 {
		t.Fatalf("expected count 100, got %d", summary.Count)
	}
	if summary.Min != 1.0 || summary.Max != 100.0 {
		t.Fatalf("unexpected min/max: min=%f max=%f", summary.Min, summary.Max)
	}
	if summary.P50 != 50.0 {
		t.Fatalf("expected P50 ~50, got %f", summary.P50)
	}
	if summary.P95 != 95.0 {
		t.Fatalf("expected P95 ~95, got %f", summary.P95)
	}
}

func TestRegistry_PrometheusExposition(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterCounter("test_counter", "Test counter help", "status")
	reg.RegisterGauge("test_gauge", "Test gauge help")
	reg.RegisterHistogram("test_latency_ms", "Test latency help", []float64{1.0, 5.0, 10.0})

	reg.Counter("test_counter", map[string]string{"status": "OK"}).Add(10)
	reg.Gauge("test_gauge", nil).Set(42.5)
	reg.Histogram("test_latency_ms", nil).Observe(2.5)

	out := reg.RenderPrometheus()

	if !strings.Contains(out, "# HELP test_counter Test counter help") {
		t.Fatalf("missing counter help in output")
	}
	if !strings.Contains(out, "test_counter{status=\"OK\"} 10") {
		t.Fatalf("missing counter value in output: %s", out)
	}
	if !strings.Contains(out, "test_gauge 42.5") {
		t.Fatalf("missing gauge in output: %s", out)
	}
	if !strings.Contains(out, "test_latency_ms_bucket{le=\"5\"} 1") {
		t.Fatalf("missing histogram bucket in output: %s", out)
	}

	jsonBytes, err := reg.ExportJSON()
	if err != nil || len(jsonBytes) == 0 {
		t.Fatalf("failed ExportJSON: %v", err)
	}
}

func TestCollector_RecordAndSummary(t *testing.T) {
	c := NewCollector()

	req := &pipeline.PipelineRequest{
		TraceID:  "trace_test_01",
		RawQuery: "find postgres",
	}

	resp := &pipeline.PipelineResponse{
		TraceID:          "trace_test_01",
		Status:           pipeline.StatusCleared,
		ScrubbedQuery:    "find postgres",
		TotalDurationMs:  4.2,
		ClearingPriceCPM: 3.50,
		DebitUSD:         0.0035,
		StageLatencies: map[pipeline.PipelineStage]float64{
			pipeline.StageTrafficScrubbing: 0.12,
			pipeline.StageAuctionClearing:  0.85,
			pipeline.StageEscrowSettlement: 0.40,
		},
		Timestamp: time.Now(),
	}

	c.RecordPipelineExecution(req, resp)

	summary := c.GetSummary()
	if summary.TotalRequests != 1 {
		t.Fatalf("expected 1 request, got %d", summary.TotalRequests)
	}
	if summary.ClearedRequests != 1 {
		t.Fatalf("expected 1 cleared, got %d", summary.ClearedRequests)
	}
	if summary.SLAPassRatePercent != 100.0 {
		t.Fatalf("expected 100%% SLA compliance, got %f", summary.SLAPassRatePercent)
	}
	if summary.TotalRevenueUSD != 0.0035 {
		t.Fatalf("expected 0.0035 revenue, got %f", summary.TotalRevenueUSD)
	}
	if summary.PublisherRevenueUSD != 0.0035*0.70 {
		t.Fatalf("expected 70%% publisher revenue, got %f", summary.PublisherRevenueUSD)
	}
}

func TestRunTelemetryDemo(t *testing.T) {
	collector, summary := RunTelemetryDemo()

	if summary.TotalRequests == 0 {
		t.Fatalf("expected requests in demo summary")
	}
	if summary.ClearedRequests == 0 {
		t.Fatalf("expected cleared requests in demo summary")
	}
	if summary.SuppressedRequests == 0 {
		t.Fatalf("expected suppressed requests in quiet zone")
	}
	if summary.PromptInjectionsBlocked == 0 && summary.FraudDetections == 0 {
		t.Fatalf("expected threats or fraud to be recorded")
	}
	if summary.SLAPassRatePercent < 90.0 {
		t.Fatalf("expected >90%% SLA pass rate, got %f", summary.SLAPassRatePercent)
	}

	scorecard := RenderTelemetryScorecard(summary)
	if !strings.Contains(scorecard, "AGENT ADVERTISING PROTOCOL (AAP) - REAL-TIME TELEMETRY ENGINE") {
		t.Fatalf("scorecard missing header")
	}

	md := RenderTelemetryMarkdown(summary)
	if !strings.Contains(md, "Telemetry & Metrics Report") {
		t.Fatalf("markdown missing header")
	}

	mermaid := RenderTelemetryMermaid(summary)
	if !strings.Contains(mermaid, "flowchart TD") {
		t.Fatalf("mermaid missing flowchart TD")
	}

	prom := collector.Registry().RenderPrometheus()
	if !strings.Contains(prom, MetricRequestsTotal) {
		t.Fatalf("prometheus export missing %s", MetricRequestsTotal)
	}
}

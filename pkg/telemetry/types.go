package telemetry

import "time"

// MetricType classifies the Prometheus/OpenTelemetry metric variant.
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
)

// HistogramSummary encapsulates statistical percentiles and aggregates.
type HistogramSummary struct {
	Count   int64     `json:"count"`
	Sum     float64   `json:"sum"`
	Average float64   `json:"average"`
	Min     float64   `json:"min"`
	P50     float64   `json:"p50"`
	P90     float64   `json:"p90"`
	P95     float64   `json:"p95"`
	P99     float64   `json:"p99"`
	Max     float64   `json:"max"`
}

// TelemetrySummary represents the unified observability scorecard for AAP.
type TelemetrySummary struct {
	GeneratedAt              time.Time                   `json:"generated_at"`
	TotalRequests            int64                       `json:"total_requests"`
	ClearedRequests          int64                       `json:"cleared_requests"`
	SuppressedRequests       int64                       `json:"suppressed_requests"`
	RejectedRequests         int64                       `json:"rejected_requests"`
	NoBidRequests            int64                       `json:"no_bid_requests"`
	ClearedRatePercent       float64                     `json:"cleared_rate_percent"`
	SLAPassRatePercent       float64                     `json:"sla_pass_rate_percent"` // Under 50ms SLA
	TotalRevenueUSD          float64                     `json:"total_revenue_usd"`
	PublisherRevenueUSD      float64                     `json:"publisher_revenue_usd"` // 70%
	AgentRevenueUSD          float64                     `json:"agent_revenue_usd"`     // 20%
	TreasuryRevenueUSD       float64                     `json:"treasury_revenue_usd"`  // 10%
	AverageClearingPriceCPM  float64                     `json:"average_clearing_price_cpm"`
	PromptInjectionsBlocked  int64                       `json:"prompt_injections_blocked"`
	PIIEntitiesRedacted      int64                       `json:"pii_entities_redacted"`
	QuietZoneSuppressed      int64                       `json:"quiet_zone_suppressed"`
	FraudDetections          int64                       `json:"fraud_detections"`
	OverallLatencyMs         HistogramSummary            `json:"overall_latency_ms"`
	StageLatenciesMs         map[string]HistogramSummary `json:"stage_latencies_ms"`
}

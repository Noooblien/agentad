package telemetry

import (
	"strings"
	"sync"
	"time"

	"github.com/Noooblien/agentad/pkg/pipeline"
)

// Metric names following Prometheus naming conventions
const (
	MetricRequestsTotal            = "aap_pipeline_requests_total"
	MetricPipelineDurationMs       = "aap_pipeline_duration_ms"
	MetricStageDurationMs          = "aap_stage_duration_ms"
	MetricClearingPriceCPM         = "aap_clearing_price_cpm"
	MetricRevenueUSDTotal          = "aap_revenue_usd_total"
	MetricThreatsBlockedTotal      = "aap_security_threats_blocked_total"
	MetricSuppressionsTotal        = "aap_placement_suppressions_total"
	MetricPrivacyRedactionsTotal   = "aap_privacy_entities_redacted_total"
)

// Collector coordinates metric recording and statistical summary aggregation.
type Collector struct {
	mu           sync.RWMutex
	registry     *Registry
	history      []*pipeline.PipelineResponse
	stageHists   map[string]*Histogram
	overallHist  *Histogram
	cpmHist      *Histogram
	startedAt    time.Time
}

// NewCollector initializes and pre-registers standard AAP metrics.
func NewCollector() *Collector {
	reg := NewRegistry()

	reg.RegisterCounter(MetricRequestsTotal, "Total number of AAP pipeline executions evaluated", "status", "halt_stage")
	reg.RegisterCounter(MetricRevenueUSDTotal, "Cumulative revenue generated across participants in USD", "recipient")
	reg.RegisterCounter(MetricThreatsBlockedTotal, "Total security threats and injections neutralized", "engine", "threat_type")
	reg.RegisterCounter(MetricSuppressionsTotal, "Total ad candidates safely suppressed by quiet zones or fatigue", "reason")
	reg.RegisterCounter(MetricPrivacyRedactionsTotal, "Total sensitive PII entities redacted", "entity_type")

	reg.RegisterHistogram(MetricPipelineDurationMs, "End-to-end pipeline execution latency in milliseconds", DefaultLatencyBucketsMs)
	reg.RegisterHistogram(MetricStageDurationMs, "Per-stage execution latency in milliseconds", DefaultLatencyBucketsMs, "stage")
	reg.RegisterHistogram(MetricClearingPriceCPM, "Vickrey second-price auction clearing price distribution in USD CPM", DefaultCPMBuckets)

	return &Collector{
		registry:    reg,
		history:     make([]*pipeline.PipelineResponse, 0, 128),
		stageHists:  make(map[string]*Histogram),
		overallHist: NewHistogram(DefaultLatencyBucketsMs),
		cpmHist:     NewHistogram(DefaultCPMBuckets),
		startedAt:   time.Now().UTC(),
	}
}

// Registry returns the underlying Prometheus registry.
func (c *Collector) Registry() *Registry {
	return c.registry
}

// RecordPipelineExecution processes a completed pipeline request and response.
func (c *Collector) RecordPipelineExecution(req *pipeline.PipelineRequest, resp *pipeline.PipelineResponse) {
	if resp == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.history = append(c.history, resp)

	// 1. Requests Total Counter
	haltStageStr := string(resp.HaltStage)
	if haltStageStr == "" {
		haltStageStr = "NONE"
	}
	c.registry.Counter(MetricRequestsTotal, map[string]string{
		"status":     string(resp.Status),
		"halt_stage": haltStageStr,
	}).Inc()

	// 2. End-to-End Latency
	c.overallHist.Observe(resp.TotalDurationMs)
	c.registry.Histogram(MetricPipelineDurationMs, nil).Observe(resp.TotalDurationMs)

	// 3. Per-Stage Latencies
	for stage, lat := range resp.StageLatencies {
		stageStr := string(stage)
		h, exists := c.stageHists[stageStr]
		if !exists {
			h = NewHistogram(DefaultLatencyBucketsMs)
			c.stageHists[stageStr] = h
		}
		h.Observe(lat)
		c.registry.Histogram(MetricStageDurationMs, map[string]string{"stage": stageStr}).Observe(lat)
	}

	// 4. Financial & CPM Metrics
	if resp.Status == pipeline.StatusCleared && resp.ClearingPriceCPM > 0 {
		c.cpmHist.Observe(resp.ClearingPriceCPM)
		c.registry.Histogram(MetricClearingPriceCPM, nil).Observe(resp.ClearingPriceCPM)

		if resp.DebitUSD > 0 {
			// Programmatic split: 70% publisher, 20% agent, 10% treasury
			c.registry.Counter(MetricRevenueUSDTotal, map[string]string{"recipient": "publisher"}).Add(resp.DebitUSD * 0.70)
			c.registry.Counter(MetricRevenueUSDTotal, map[string]string{"recipient": "agent"}).Add(resp.DebitUSD * 0.20)
			c.registry.Counter(MetricRevenueUSDTotal, map[string]string{"recipient": "treasury"}).Add(resp.DebitUSD * 0.10)
		}
	}

	// 5. Security & Threat Neutralization
	if resp.Status == pipeline.StatusRejected {
		if resp.HaltStage == pipeline.StageTrafficScrubbing {
			c.registry.Counter(MetricThreatsBlockedTotal, map[string]string{
				"engine":      "fraud_detector",
				"threat_type": "synthetic_traffic_probe",
			}).Inc()
		} else if resp.HaltStage == pipeline.StageGuardrailPurify {
			c.registry.Counter(MetricThreatsBlockedTotal, map[string]string{
				"engine":      "ast_guardrail",
				"threat_type": "indirect_prompt_injection",
			}).Inc()
		} else if resp.HaltStage == pipeline.StageBrandSafety {
			c.registry.Counter(MetricThreatsBlockedTotal, map[string]string{
				"engine":      "brand_safety",
				"threat_type": "policy_violation",
			}).Inc()
		}
	}

	// 6. Placement & Quiet Zone Suppressions
	if resp.Status == pipeline.StatusSuppressed {
		reasonTag := "attention_budget"
		if strings.Contains(strings.ToLower(resp.Reason), "quiet") || strings.Contains(strings.ToLower(resp.Reason), "fatigue") {
			reasonTag = "cognitive_quiet_zone"
		} else if strings.Contains(strings.ToLower(resp.Reason), "pacing") {
			reasonTag = "budget_pacing"
		}
		c.registry.Counter(MetricSuppressionsTotal, map[string]string{
			"reason": reasonTag,
		}).Inc()
	}

	// 7. Privacy Entities Redacted
	if req != nil && req.RawQuery != resp.ScrubbedQuery && resp.ScrubbedQuery != "" {
		c.registry.Counter(MetricPrivacyRedactionsTotal, map[string]string{
			"entity_type": "sensitive_credential_or_pii",
		}).Inc()
	}
}

// GetSummary compiles the aggregated TelemetrySummary for display or reporting.
func (c *Collector) GetSummary() *TelemetrySummary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	summary := &TelemetrySummary{
		GeneratedAt:      time.Now().UTC(),
		TotalRequests:    int64(len(c.history)),
		OverallLatencyMs: c.overallHist.GetSummary(),
		StageLatenciesMs: make(map[string]HistogramSummary),
	}

	for stage, h := range c.stageHists {
		summary.StageLatenciesMs[stage] = h.GetSummary()
	}

	var slaUnder50Count int64
	var totalCPM float64
	var clearedCountWithCPM int64

	for _, r := range c.history {
		if r.TotalDurationMs <= 50.0 {
			slaUnder50Count++
		}

		switch r.Status {
		case pipeline.StatusCleared:
			summary.ClearedRequests++
			summary.TotalRevenueUSD += r.DebitUSD
			if r.ClearingPriceCPM > 0 {
				totalCPM += r.ClearingPriceCPM
				clearedCountWithCPM++
			}
		case pipeline.StatusSuppressed:
			summary.SuppressedRequests++
			summary.QuietZoneSuppressed++
		case pipeline.StatusRejected:
			summary.RejectedRequests++
			if r.HaltStage == pipeline.StageGuardrailPurify {
				summary.PromptInjectionsBlocked++
			}
			if r.HaltStage == pipeline.StageTrafficScrubbing {
				summary.FraudDetections++
			}
		case pipeline.StatusNoBid:
			summary.NoBidRequests++
		}
	}

	if summary.TotalRequests > 0 {
		summary.ClearedRatePercent = (float64(summary.ClearedRequests) / float64(summary.TotalRequests)) * 100.0
		summary.SLAPassRatePercent = (float64(slaUnder50Count) / float64(summary.TotalRequests)) * 100.0
	}

	if clearedCountWithCPM > 0 {
		summary.AverageClearingPriceCPM = totalCPM / float64(clearedCountWithCPM)
	}

	summary.PublisherRevenueUSD = summary.TotalRevenueUSD * 0.70
	summary.AgentRevenueUSD = summary.TotalRevenueUSD * 0.20
	summary.TreasuryRevenueUSD = summary.TotalRevenueUSD * 0.10

	// PII Redactions count from registry
	summary.PIIEntitiesRedacted = int64(c.registry.Counter(MetricPrivacyRedactionsTotal, map[string]string{
		"entity_type": "sensitive_credential_or_pii",
	}).Get())

	return summary
}

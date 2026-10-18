package bench

import (
	"time"
)

// BenchmarkType identifies the evaluation target.
type BenchmarkType string

const (
	BenchTypeInjection BenchmarkType = "injection"
	BenchTypeAuction   BenchmarkType = "auction"
	BenchTypeMatcher   BenchmarkType = "matcher"
	BenchTypeFraud     BenchmarkType = "fraud"
	BenchTypeAll       BenchmarkType = "all"
)

// BenchmarkConfig defines iteration bounds, latency thresholds, and concurrency.
type BenchmarkConfig struct {
	Iterations   int           `json:"iterations"`
	Concurrency  int           `json:"concurrency"`
	SLABudgetMs  float64       `json:"sla_budget_ms"`
	MinRecall    float64       `json:"min_recall"`
	StrictMode   bool          `json:"strict_mode"`
	TargetSuite  BenchmarkType `json:"target_suite"`
	OutputFormat string        `json:"output_format"` // "text", "markdown", "json"
}

// DefaultBenchmarkConfig returns standard calibration settings.
func DefaultBenchmarkConfig() BenchmarkConfig {
	return BenchmarkConfig{
		Iterations:   500,
		Concurrency:  1,
		SLABudgetMs:  50.0, // Sub-50ms RTB SLA
		MinRecall:    0.99, // 99% minimum injection detection
		StrictMode:   false,
		TargetSuite:  BenchTypeAll,
		OutputFormat: "text",
	}
}

// LatencyPercentiles tracks measured latency distributions in milliseconds.
type LatencyPercentiles struct {
	Min    float64 `json:"min_ms"`
	Max    float64 `json:"max_ms"`
	Mean   float64 `json:"mean_ms"`
	P50    float64 `json:"p50_ms"`
	P90    float64 `json:"p90_ms"`
	P95    float64 `json:"p95_ms"`
	P99    float64 `json:"p99_ms"`
}

// BenchmarkReport captures verified empirical results of a single benchmark suite.
type BenchmarkReport struct {
	Name               string             `json:"name"`
	Category           string             `json:"category"`
	TotalOperations    int                `json:"total_operations"`
	SuccessCount       int                `json:"success_count"`
	FailureCount       int                `json:"failure_count"`
	RecallRate         float64            `json:"recall_rate"`
	FalsePositiveRate  float64            `json:"false_positive_rate"`
	Latency            LatencyPercentiles `json:"latency"`
	ThroughputOpsSec   float64            `json:"throughput_ops_sec"`
	SLABudgetMs        float64            `json:"sla_budget_ms"`
	SLAViolations      int                `json:"sla_violations"`
	Passed             bool               `json:"passed"`
	Violations         []string           `json:"violations,omitempty"`
	Duration           time.Duration      `json:"duration"`
}

// SuiteSummary aggregates reports across all executed benchmark targets.
type SuiteSummary struct {
	Timestamp      time.Time         `json:"timestamp"`
	TotalSuites    int               `json:"total_suites"`
	PassedSuites   int               `json:"passed_suites"`
	FailedSuites   int               `json:"failed_suites"`
	OverallPassed  bool              `json:"overall_passed"`
	TotalDuration  time.Duration     `json:"total_duration"`
	Reports        []BenchmarkReport `json:"reports"`
}

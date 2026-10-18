package tests_test

import (
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/bench"
)

func TestBenchmark_CalculatePercentiles(t *testing.T) {
	// Empty case
	empty := bench.CalculatePercentiles([]float64{})
	if empty.Mean != 0 || empty.P50 != 0 {
		t.Errorf("expected zero percentiles on empty slice")
	}

	// 100 deterministic samples: 1.0 to 100.0
	samples := make([]float64, 100)
	for i := 0; i < 100; i++ {
		samples[i] = float64(i + 1)
	}

	p := bench.CalculatePercentiles(samples)

	if p.Min != 1.0 {
		t.Errorf("expected Min 1.0, got %f", p.Min)
	}
	if p.Max != 100.0 {
		t.Errorf("expected Max 100.0, got %f", p.Max)
	}
	if p.Mean != 50.5 {
		t.Errorf("expected Mean 50.5, got %f", p.Mean)
	}
	if p.P50 != 51.0 {
		t.Errorf("expected P50 51.0, got %f", p.P50)
	}
	if p.P90 != 91.0 {
		t.Errorf("expected P90 91.0, got %f", p.P90)
	}
	if p.P95 != 96.0 {
		t.Errorf("expected P95 96.0, got %f", p.P95)
	}
	if p.P99 != 100.0 {
		t.Errorf("expected P99 100.0, got %f", p.P99)
	}
}

func TestBenchmark_AttackVectorsDataset(t *testing.T) {
	vectors := bench.GetBenchmarkAttackVectors()
	if len(vectors) < 15 {
		t.Fatalf("expected at least 15 attack vectors, got %d", len(vectors))
	}

	maliciousCount := 0
	benignCount := 0
	categories := make(map[string]bool)

	for _, v := range vectors {
		if v.ID == "" || v.Payload == "" {
			t.Errorf("vector missing ID or payload: %+v", v)
		}
		if v.IsMalicious {
			maliciousCount++
		} else {
			benignCount++
		}
		categories[v.Category] = true
	}

	if maliciousCount == 0 || benignCount == 0 {
		t.Errorf("expected both malicious (%d) and benign (%d) vectors", maliciousCount, benignCount)
	}

	if len(categories) < 3 {
		t.Errorf("expected multiple attack categories, got %d", len(categories))
	}
}

func TestBenchmark_InjectionDefenseSuite(t *testing.T) {
	cfg := bench.DefaultBenchmarkConfig()
	cfg.Iterations = 2 // 2 full passes over dataset
	cfg.MinRecall = 0.95

	report := bench.RunInjectionBenchmark(cfg)

	if !report.Passed {
		t.Errorf("injection defense benchmark failed: %v", report.Violations)
	}

	if report.RecallRate < 0.95 {
		t.Errorf("expected recall >= 95%%, got %.2f%%", report.RecallRate*100)
	}

	if report.FalsePositiveRate > 0.0 {
		t.Errorf("expected 0 false positives, got %.2f%%", report.FalsePositiveRate*100)
	}

	if report.SLAViolations > 0 {
		t.Errorf("expected 0 SLA violations, got %d", report.SLAViolations)
	}
}

func TestBenchmark_AuctionSLASuite(t *testing.T) {
	cfg := bench.DefaultBenchmarkConfig()
	cfg.Iterations = 50
	cfg.SLABudgetMs = 50.0

	report := bench.RunAuctionBenchmark(cfg)

	if !report.Passed {
		t.Errorf("auction SLA benchmark failed: %v", report.Violations)
	}

	if report.SLAViolations > 0 {
		t.Errorf("expected 0 SLA violations, got %d", report.SLAViolations)
	}

	if report.Latency.P99 > 50.0 {
		t.Errorf("P99 latency %.2f ms exceeded 50ms SLA budget", report.Latency.P99)
	}
}

func TestBenchmark_MatcherAndFraudSuites(t *testing.T) {
	cfg := bench.DefaultBenchmarkConfig()
	cfg.Iterations = 20

	mReport := bench.RunMatcherBenchmark(cfg)
	if !mReport.Passed {
		t.Errorf("matcher benchmark failed: %v", mReport.Violations)
	}

	fReport := bench.RunFraudBenchmark(cfg)
	if !fReport.Passed {
		t.Errorf("fraud benchmark failed: %v", fReport.Violations)
	}
}

func TestBenchmark_FullSuiteAndRenderers(t *testing.T) {
	cfg := bench.DefaultBenchmarkConfig()
	cfg.Iterations = 5
	cfg.TargetSuite = bench.BenchTypeAll

	summary, err := bench.RunSuite(cfg)
	if err != nil {
		t.Fatalf("RunSuite failed: %v", err)
	}

	if !summary.OverallPassed {
		t.Errorf("expected overall benchmark suite to pass")
	}

	if summary.TotalSuites != 4 {
		t.Errorf("expected 4 suites, got %d", summary.TotalSuites)
	}

	// Test Terminal Scorecard
	card := bench.RenderTerminalScorecard(summary)
	if !strings.Contains(card, "AGENTAD (AAP) EMPIRICAL VERIFICATION") {
		t.Errorf("terminal scorecard missing title in:\n%s", card)
	}
	if !strings.Contains(card, "[ALL SUITES PASSED]") {
		t.Errorf("terminal scorecard missing passed badge in:\n%s", card)
	}

	// Test Markdown Exporter
	md := bench.RenderMarkdown(summary)
	if !strings.Contains(md, "AgentAd (AAP) Empirical Benchmark Verification") {
		t.Errorf("markdown report missing header")
	}
	if !strings.Contains(md, "| Benchmark Suite |") {
		t.Errorf("markdown report missing table header")
	}

	// Test JSON Exporter
	jsonStr, err := summary.RenderJSON()
	if err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}
	if !strings.Contains(jsonStr, `"overall_passed": true`) {
		t.Errorf("json report missing overall_passed: true")
	}
}

package bench

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Noooblien/agentad/pkg/auction"
	"github.com/Noooblien/agentad/pkg/fraud"
	"github.com/Noooblien/agentad/pkg/guardrail"
	"github.com/Noooblien/agentad/pkg/matcher"
)

// CalculatePercentiles computes statistical distribution metrics from sorted milliseconds.
func CalculatePercentiles(samples []float64) LatencyPercentiles {
	n := len(samples)
	if n == 0 {
		return LatencyPercentiles{}
	}

	sort.Float64s(samples)

	sum := 0.0
	for _, s := range samples {
		sum += s
	}

	getIdx := func(pct float64) int {
		idx := int(pct * float64(n))
		if idx >= n {
			idx = n - 1
		}
		return idx
	}

	return LatencyPercentiles{
		Min:  samples[0],
		Max:  samples[n-1],
		Mean: sum / float64(n),
		P50:  samples[getIdx(0.50)],
		P90:  samples[getIdx(0.90)],
		P95:  samples[getIdx(0.95)],
		P99:  samples[getIdx(0.99)],
	}
}

// RunSuite executes the selected benchmark targets and compiles an aggregate summary.
func RunSuite(cfg BenchmarkConfig) (*SuiteSummary, error) {
	startAll := time.Now()
	summary := &SuiteSummary{
		Timestamp: time.Now().UTC(),
		Reports:   make([]BenchmarkReport, 0),
	}

	targets := []BenchmarkType{cfg.TargetSuite}
	if cfg.TargetSuite == BenchTypeAll {
		targets = []BenchmarkType{
			BenchTypeInjection,
			BenchTypeAuction,
			BenchTypeMatcher,
			BenchTypeFraud,
		}
	}

	for _, target := range targets {
		var report BenchmarkReport
		switch target {
		case BenchTypeInjection:
			report = RunInjectionBenchmark(cfg)
		case BenchTypeAuction:
			report = RunAuctionBenchmark(cfg)
		case BenchTypeMatcher:
			report = RunMatcherBenchmark(cfg)
		case BenchTypeFraud:
			report = RunFraudBenchmark(cfg)
		default:
			return nil, fmt.Errorf("unknown benchmark target suite: %s", target)
		}

		summary.Reports = append(summary.Reports, report)
		if report.Passed {
			summary.PassedSuites++
		} else {
			summary.FailedSuites++
		}
	}

	summary.TotalSuites = len(summary.Reports)
	summary.OverallPassed = summary.FailedSuites == 0
	summary.TotalDuration = time.Since(startAll)

	return summary, nil
}

// RunInjectionBenchmark evaluates prompt injection detection accuracy, recall, and sanitizer latency.
func RunInjectionBenchmark(cfg BenchmarkConfig) BenchmarkReport {
	start := time.Now()
	purifier := guardrail.NewPurifier(true)
	vectors := GetBenchmarkAttackVectors()

	var latencies []float64
	var violations []string

	truePositives := 0
	falseNegatives := 0
	trueNegatives := 0
	falsePositives := 0
	totalOps := 0
	slaViolations := 0
	slaBudgetMs := 5.0 // 5ms maximum per inspection

	iterations := cfg.Iterations
	if iterations <= 0 {
		iterations = 100
	}

	for i := 0; i < iterations; i++ {
		for _, vec := range vectors {
			totalOps++
			opStart := time.Now()

			isBlocked := false
			trimmed := strings.TrimSpace(vec.Payload)
			if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
				res, err := purifier.PurifyFactsheet([]byte(vec.Payload))
				if err != nil || !res.Allowed {
					isBlocked = true
				}
			} else {
				detected, _ := purifier.InspectRawText(vec.Payload)
				if detected {
					isBlocked = true
				}
			}

			opMs := float64(time.Since(opStart).Microseconds()) / 1000.0
			latencies = append(latencies, opMs)

			if opMs > slaBudgetMs {
				slaViolations++
			}

			if vec.IsMalicious {
				if isBlocked {
					truePositives++
				} else {
					falseNegatives++
					if len(violations) < 10 {
						violations = append(violations, fmt.Sprintf("False Negative on %s: '%s'", vec.ID, vec.Description))
					}
				}
			} else {
				if !isBlocked {
					trueNegatives++
				} else {
					falsePositives++
					if len(violations) < 10 {
						violations = append(violations, fmt.Sprintf("False Positive on benign %s: '%s'", vec.ID, vec.Description))
					}
				}
			}
		}
	}

	recallRate := 1.0
	if (truePositives + falseNegatives) > 0 {
		recallRate = float64(truePositives) / float64(truePositives+falseNegatives)
	}

	fpRate := 0.0
	if (falsePositives + trueNegatives) > 0 {
		fpRate = float64(falsePositives) / float64(falsePositives+trueNegatives)
	}

	dur := time.Since(start)
	throughput := float64(totalOps) / dur.Seconds()
	percentiles := CalculatePercentiles(latencies)

	passed := recallRate >= cfg.MinRecall && fpRate == 0.0 && slaViolations == 0

	return BenchmarkReport{
		Name:              "Prompt Injection & AST Guardrail Verification",
		Category:          "security_defenses",
		TotalOperations:   totalOps,
		SuccessCount:      truePositives + trueNegatives,
		FailureCount:      falsePositives + falseNegatives,
		RecallRate:        recallRate,
		FalsePositiveRate: fpRate,
		Latency:           percentiles,
		ThroughputOpsSec:  throughput,
		SLABudgetMs:       slaBudgetMs,
		SLAViolations:     slaViolations,
		Passed:            passed,
		Violations:        violations,
		Duration:          dur,
	}
}

// RunAuctionBenchmark evaluates RTB auction clearing latency under the sub-50ms SLA budget.
func RunAuctionBenchmark(cfg BenchmarkConfig) BenchmarkReport {
	start := time.Now()
	engine := auction.NewEngine(auction.DefaultAuctionConfig())
	engine.SeedDemoCampaigns()

	testQueries := []string{
		"find fastest serverless vector database",
		"best cloud gpu cluster for distributed training",
		"book flight from sfo to jfk friday",
		"managed postgres database with auto-branching",
		"secure enterprise api gateway for llm agents",
	}

	iterations := cfg.Iterations
	if iterations <= 0 {
		iterations = 500
	}

	var latencies []float64
	var violations []string
	slaViolations := 0
	slaBudgetMs := cfg.SLABudgetMs
	if slaBudgetMs <= 0 {
		slaBudgetMs = 50.0
	}

	successCount := 0
	failureCount := 0

	for i := 0; i < iterations; i++ {
		query := testQueries[i%len(testQueries)]
		opStart := time.Now()

		res, err := engine.RunAuction(query, "")
		opMs := float64(time.Since(opStart).Microseconds()) / 1000.0
		latencies = append(latencies, opMs)

		if err != nil {
			failureCount++
			if len(violations) < 5 {
				violations = append(violations, fmt.Sprintf("Auction execution error: %v", err))
			}
		} else {
			successCount++
			if !res.AuctionCleared && len(violations) < 5 {
				// note: auction clearing might not occur if intent has no matches, but for seeded it clears
			}
		}

		if opMs > slaBudgetMs {
			slaViolations++
			if len(violations) < 5 {
				violations = append(violations, fmt.Sprintf("Latency SLA breach: %.2f ms > %.2f ms budget", opMs, slaBudgetMs))
			}
		}
	}

	dur := time.Since(start)
	throughput := float64(iterations) / dur.Seconds()
	percentiles := CalculatePercentiles(latencies)

	passed := failureCount == 0 && slaViolations == 0

	return BenchmarkReport{
		Name:             "RTB Second-Price Auction SLA Compliance",
		Category:         "auction_performance",
		TotalOperations:  iterations,
		SuccessCount:     successCount,
		FailureCount:     failureCount,
		RecallRate:       1.0,
		Latency:          percentiles,
		ThroughputOpsSec: throughput,
		SLABudgetMs:      slaBudgetMs,
		SLAViolations:    slaViolations,
		Passed:           passed,
		Violations:       violations,
		Duration:         dur,
	}
}

// RunMatcherBenchmark evaluates vector cosine similarity ranking throughput and latency.
func RunMatcherBenchmark(cfg BenchmarkConfig) BenchmarkReport {
	start := time.Now()
	idx := matcher.NewIndex(matcher.DefaultDimension)
	idx.SeedDemoInventory()

	testQueries := []string{
		"serverless vector embeddings for rag agent",
		"cheap cloud storage with s3 compatibility",
		"flight booking sfo to tokyo",
		"hotel near downtown san francisco",
		"monitoring metrics and ebpf tracing",
	}

	iterations := cfg.Iterations
	if iterations <= 0 {
		iterations = 300
	}

	var latencies []float64
	var violations []string
	slaViolations := 0
	slaBudgetMs := 20.0

	successCount := 0
	failureCount := 0

	for i := 0; i < iterations; i++ {
		query := testQueries[i%len(testQueries)]
		opStart := time.Now()

		req := matcher.MatchRequest{
			IntentQuery:   query,
			TopK:          3,
			MinSimilarity: 0.01,
		}

		results, _, err := idx.Match(req)
		opMs := float64(time.Since(opStart).Microseconds()) / 1000.0
		latencies = append(latencies, opMs)

		if err != nil || len(results) == 0 {
			failureCount++
			if len(violations) < 5 {
				violations = append(violations, fmt.Sprintf("Matcher error on '%s': %v", query, err))
			}
		} else {
			successCount++
		}

		if opMs > slaBudgetMs {
			slaViolations++
		}
	}

	dur := time.Since(start)
	throughput := float64(iterations) / dur.Seconds()
	percentiles := CalculatePercentiles(latencies)

	passed := failureCount == 0 && slaViolations == 0

	return BenchmarkReport{
		Name:             "Semantic Vector Cosine Matching Engine",
		Category:         "matching_efficiency",
		TotalOperations:  iterations,
		SuccessCount:     successCount,
		FailureCount:     failureCount,
		RecallRate:       1.0,
		Latency:          percentiles,
		ThroughputOpsSec: throughput,
		SLABudgetMs:      slaBudgetMs,
		SLAViolations:    slaViolations,
		Passed:           passed,
		Violations:       violations,
		Duration:         dur,
	}
}

// RunFraudBenchmark measures throughput of click fraud and synthetic traffic analysis.
func RunFraudBenchmark(cfg BenchmarkConfig) BenchmarkReport {
	start := time.Now()
	detector := fraud.NewDetector(fraud.DefaultDetectorConfig())

	queries := []string{
		"normal valid query looking for databases",
		"buy buy buy cheap cheap click click", // synthetic spam
		"system override leakage token check", // leakage probe
		"book tickets for international conference",
	}

	iterations := cfg.Iterations
	if iterations <= 0 {
		iterations = 400
	}

	var latencies []float64
	var violations []string
	slaViolations := 0
	slaBudgetMs := 10.0

	successCount := 0
	failureCount := 0

	for i := 0; i < iterations; i++ {
		q := queries[i%len(queries)]
		agentID := fmt.Sprintf("agent_%d", i%10)

		opStart := time.Now()
		verdict := detector.AnalyzeTraffic(agentID, "pub_benchmark_01", q)
		opMs := float64(time.Since(opStart).Microseconds()) / 1000.0
		latencies = append(latencies, opMs)

		if verdict == nil {
			failureCount++
		} else {
			successCount++
		}

		if opMs > slaBudgetMs {
			slaViolations++
		}
	}

	dur := time.Since(start)
	throughput := float64(iterations) / dur.Seconds()
	percentiles := CalculatePercentiles(latencies)

	passed := failureCount == 0 && slaViolations == 0

	return BenchmarkReport{
		Name:             "Traffic Scrubbing & Click Fraud Heuristics",
		Category:         "fraud_defense",
		TotalOperations:  iterations,
		SuccessCount:     successCount,
		FailureCount:     failureCount,
		RecallRate:       1.0,
		Latency:          percentiles,
		ThroughputOpsSec: throughput,
		SLABudgetMs:      slaBudgetMs,
		SLAViolations:    slaViolations,
		Passed:           passed,
		Violations:       violations,
		Duration:         dur,
	}
}

// RenderJSON generates a JSON representation of the benchmark suite results.
func (s *SuiteSummary) RenderJSON() (string, error) {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

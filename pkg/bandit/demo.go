package bandit

import (
	"fmt"
)

// RunDemoSuite executes all 4 production benchmark scenarios and returns their summaries.
func RunDemoSuite() ([]*BatchSimulationSummary, error) {
	summaries := make([]*BatchSimulationSummary, 0, 4)

	// Scenario 1: Cold-Start Newcomer vs Stale Incumbent (Thompson Sampling)
	s1, err := runColdStartNewcomerDemo()
	if err != nil {
		return nil, fmt.Errorf("scenario 1 failed: %w", err)
	}
	summaries = append(summaries, s1)

	// Scenario 2: Multi-Arm Thompson Sampling Convergence
	s2, err := runThompsonConvergenceDemo()
	if err != nil {
		return nil, fmt.Errorf("scenario 2 failed: %w", err)
	}
	summaries = append(summaries, s2)

	// Scenario 3: UCB1 Upper Confidence Bound Exploration
	s3, err := runUCB1Demo()
	if err != nil {
		return nil, fmt.Errorf("scenario 3 failed: %w", err)
	}
	summaries = append(summaries, s3)

	// Scenario 4: Epsilon-Greedy Exploration Comparison
	s4, err := runEpsilonGreedyDemo()
	if err != nil {
		return nil, fmt.Errorf("scenario 4 failed: %w", err)
	}
	summaries = append(summaries, s4)

	return summaries, nil
}

// Scenario 1: Newcomer cold-start discovery vs established incumbent
func runColdStartNewcomerDemo() (*BatchSimulationSummary, error) {
	cfg := DefaultBanditConfig()
	cfg.Algorithm = AlgorithmThompsonSampling
	cfg.ColdStartThreshold = 8
	engine := NewBanditEngine(cfg, NewSampler(42))

	category := "cloud_infra"

	// Pre-seed an incumbent arm with 100 historical impressions and 12% CTR
	incumbent := CandidateArm{
		CampaignID:     "cmp_incumbent_legacy",
		AdvertiserName: "LegacyCloud Pro",
		Category:       category,
		BidCPM:         15.0,
		RelevanceScore: 0.85,
	}
	arm := engine.GetOrCreateArm(category, incumbent)
	arm.Impressions = 100
	arm.Clicks = 12
	arm.Alpha = 13.0
	arm.Beta = 89.0
	arm.EmpiricalCTR = 0.12
	arm.InColdStart = false

	// A new high-utility competitor joins with true CTR 38%
	newcomer := CandidateArm{
		CampaignID:     "cmp_newcomer_fast",
		AdvertiserName: "HyperScale GPUs",
		Category:       category,
		BidCPM:         18.0,
		RelevanceScore: 0.95,
	}

	averagePlayer := CandidateArm{
		CampaignID:     "cmp_baseline_vps",
		AdvertiserName: "BasicNode VPS",
		Category:       category,
		BidCPM:         12.0,
		RelevanceScore: 0.70,
	}

	candidates := []CandidateArm{incumbent, newcomer, averagePlayer}
	groundTruths := map[string]float64{
		"cmp_incumbent_legacy": 0.12,
		"cmp_newcomer_fast":     0.38,
		"cmp_baseline_vps":      0.08,
	}

	return engine.SimulateBatch("Cold-Start Fair Discovery", category, candidates, 150, groundTruths)
}

// Scenario 2: Multi-arm Thompson Sampling convergence across 4 competitors
func runThompsonConvergenceDemo() (*BatchSimulationSummary, error) {
	cfg := DefaultBanditConfig()
	cfg.Algorithm = AlgorithmThompsonSampling
	engine := NewBanditEngine(cfg, NewSampler(1337))

	category := "developer_tools"
	candidates := []CandidateArm{
		{CampaignID: "cmp_neon_db", AdvertiserName: "Neon Serverless Postgres", Category: category, BidCPM: 22.0, RelevanceScore: 0.92},
		{CampaignID: "cmp_turso_db", AdvertiserName: "Turso Edge SQLite", Category: category, BidCPM: 20.0, RelevanceScore: 0.88},
		{CampaignID: "cmp_pinecone_vdb", AdvertiserName: "Pinecone Vector Index", Category: category, BidCPM: 25.0, RelevanceScore: 0.96},
		{CampaignID: "cmp_upstash_redis", AdvertiserName: "Upstash Serverless Redis", Category: category, BidCPM: 16.0, RelevanceScore: 0.80},
	}

	// Pinecone has 36% true CTR, Neon 22%, Turso 14%, Upstash 8%
	groundTruths := map[string]float64{
		"cmp_pinecone_vdb":  0.36,
		"cmp_neon_db":       0.22,
		"cmp_turso_db":      0.14,
		"cmp_upstash_redis": 0.08,
	}

	return engine.SimulateBatch("Thompson Sampling Convergence", category, candidates, 200, groundTruths)
}

// Scenario 3: UCB1 Upper Confidence Bound exploration
func runUCB1Demo() (*BatchSimulationSummary, error) {
	cfg := DefaultBanditConfig()
	cfg.Algorithm = AlgorithmUCB1
	cfg.UCBConfidenceScale = 1.2
	engine := NewBanditEngine(cfg, NewSampler(999))

	category := "security_auditing"
	candidates := []CandidateArm{
		{CampaignID: "cmp_snyk_scan", AdvertiserName: "Snyk Code Security", Category: category, BidCPM: 28.0, RelevanceScore: 0.94},
		{CampaignID: "cmp_trivy_audit", AdvertiserName: "Aqua Trivy Container Scan", Category: category, BidCPM: 24.0, RelevanceScore: 0.89},
		{CampaignID: "cmp_semgrep_sast", AdvertiserName: "Semgrep SAST Engine", Category: category, BidCPM: 26.0, RelevanceScore: 0.95},
	}

	groundTruths := map[string]float64{
		"cmp_semgrep_sast": 0.32,
		"cmp_snyk_scan":    0.20,
		"cmp_trivy_audit":  0.11,
	}

	return engine.SimulateBatch("UCB1 Upper Confidence Bound", category, candidates, 150, groundTruths)
}

// Scenario 4: Epsilon-Greedy exploration comparison
func runEpsilonGreedyDemo() (*BatchSimulationSummary, error) {
	cfg := DefaultBanditConfig()
	cfg.Algorithm = AlgorithmEpsilonGreedy
	cfg.Epsilon = 0.15
	engine := NewBanditEngine(cfg, NewSampler(777))

	category := "ai_observability"
	candidates := []CandidateArm{
		{CampaignID: "cmp_langfuse_obs", AdvertiserName: "Langfuse LLM Observability", Category: category, BidCPM: 30.0, RelevanceScore: 0.97},
		{CampaignID: "cmp_arize_eval", AdvertiserName: "Arize Phoenix Evaluation", Category: category, BidCPM: 27.0, RelevanceScore: 0.91},
		{CampaignID: "cmp_braintrust_log", AdvertiserName: "Braintrust Proxy Logs", Category: category, BidCPM: 24.0, RelevanceScore: 0.86},
	}

	groundTruths := map[string]float64{
		"cmp_langfuse_obs":  0.34,
		"cmp_arize_eval":    0.21,
		"cmp_braintrust_log": 0.13,
	}

	return engine.SimulateBatch("Epsilon-Greedy Exploration (ε=0.15)", category, candidates, 150, groundTruths)
}

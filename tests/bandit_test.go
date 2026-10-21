package tests

import (
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/bandit"
)

func TestIntegration_BanditDemoSuite(t *testing.T) {
	summaries, err := bandit.RunDemoSuite()
	if err != nil {
		t.Fatalf("failed to execute bandit demo suite: %v", err)
	}

	if len(summaries) != 4 {
		t.Fatalf("expected 4 simulation summaries, got %d", len(summaries))
	}

	for _, s := range summaries {
		if s.TotalRounds <= 0 {
			t.Errorf("scenario %s has invalid total rounds: %d", s.ScenarioName, s.TotalRounds)
		}
		if s.TotalImpressions <= 0 {
			t.Errorf("scenario %s has zero impressions", s.ScenarioName)
		}
		if s.TotalClicks <= 0 {
			t.Errorf("scenario %s has zero clicks", s.ScenarioName)
		}
		if s.AverageCTR <= 0 {
			t.Errorf("scenario %s has non-positive average CTR", s.ScenarioName)
		}
		if len(s.Arms) == 0 {
			t.Errorf("scenario %s returned no arms", s.ScenarioName)
		}

		scorecard := bandit.RenderBanditScorecard(s)
		if len(scorecard) == 0 || !strings.Contains(scorecard, "SCORECARD") {
			t.Errorf("scenario %s produced invalid scorecard", s.ScenarioName)
		}

		mermaid := bandit.GenerateBanditMermaid(s)
		if !strings.Contains(mermaid, "flowchart TD") {
			t.Errorf("scenario %s produced invalid mermaid diagram", s.ScenarioName)
		}
	}

	report := bandit.GenerateMarkdownReport(summaries)
	if !strings.Contains(report, "Contextual Multi-Armed Bandit") {
		t.Errorf("generated markdown report missing expected title")
	}
}

func TestIntegration_ThompsonColdStartDiscovery(t *testing.T) {
	cfg := bandit.DefaultBanditConfig()
	cfg.Algorithm = bandit.AlgorithmThompsonSampling
	cfg.ColdStartThreshold = 5
	engine := bandit.NewBanditEngine(cfg, bandit.NewSampler(42))

	category := "distributed_cache"
	incumbent := bandit.CandidateArm{
		CampaignID:     "cmp_old_cache",
		AdvertiserName: "OldCache Inc",
		Category:       category,
		BidCPM:         15.0,
		RelevanceScore: 0.85,
	}
	newcomer := bandit.CandidateArm{
		CampaignID:     "cmp_fast_cache",
		AdvertiserName: "NextGen FastCache",
		Category:       category,
		BidCPM:         18.0,
		RelevanceScore: 0.95,
	}

	// Incumbent has historical data with 10% CTR
	arm := engine.GetOrCreateArm(category, incumbent)
	arm.Impressions = 50
	arm.Clicks = 5
	arm.Alpha = 6.0
	arm.Beta = 46.0
	arm.EmpiricalCTR = 0.10
	arm.InColdStart = false

	groundTruths := map[string]float64{
		"cmp_old_cache":  0.10,
		"cmp_fast_cache": 0.35, // Superior newcomer
	}

	summary, err := engine.SimulateBatch("ColdStart Integration", category, []bandit.CandidateArm{incumbent, newcomer}, 120, groundTruths)
	if err != nil {
		t.Fatalf("simulation failed: %v", err)
	}

	newcomerArm, ok := engine.GetArm(category, newcomer.CampaignID)
	if !ok {
		t.Fatalf("newcomer arm not found")
	}

	// Under Thompson Sampling, the superior newcomer should be explored and eventually win substantial impressions
	if newcomerArm.Impressions < 30 {
		t.Errorf("expected superior newcomer to win at least 30 impressions, got %d", newcomerArm.Impressions)
	}

	if newcomerArm.Clicks <= 0 {
		t.Errorf("expected newcomer to register clicks, got %d", newcomerArm.Clicks)
	}

	if summary.AverageCTR < 0.15 {
		t.Errorf("expected average CTR to exceed 15%% due to newcomer adoption, got %.2f%%", summary.AverageCTR*100.0)
	}
}

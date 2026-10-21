package bandit

import (
	"math"
	"sync"
	"testing"
)

func TestSamplerBetaDistribution(t *testing.T) {
	sampler := NewSampler(12345)

	// Test Marsaglia-Tsang Gamma and Beta sampling
	alpha := 2.0
	beta := 8.0
	expectedMean := BetaMean(alpha, beta) // 2 / 10 = 0.20
	expectedVar := BetaVariance(alpha, beta)

	if math.Abs(expectedMean-0.20) > 1e-6 {
		t.Fatalf("expected BetaMean 0.20, got %f", expectedMean)
	}

	samples := 10000
	var sum, sumSq float64
	for i := 0; i < samples; i++ {
		val := sampler.SampleBeta(alpha, beta)
		if val < 0.0 || val > 1.0 {
			t.Fatalf("sample value %f out of bounds [0, 1]", val)
		}
		sum += val
		sumSq += val * val
	}

	sampleMean := sum / float64(samples)
	sampleVar := (sumSq / float64(samples)) - (sampleMean * sampleMean)

	// Sample mean should be within 0.02 of 0.20
	if math.Abs(sampleMean-expectedMean) > 0.02 {
		t.Errorf("sample mean %f deviated too far from expected %f", sampleMean, expectedMean)
	}

	// Sample variance should be within 0.01 of expected variance
	if math.Abs(sampleVar-expectedVar) > 0.01 {
		t.Errorf("sample variance %f deviated too far from expected %f", sampleVar, expectedVar)
	}
}

func TestBanditEngine_ColdStartTransition(t *testing.T) {
	cfg := DefaultBanditConfig()
	cfg.ColdStartThreshold = 5
	engine := NewBanditEngine(cfg, NewSampler(42))

	category := "unit_test_cat"
	candidate := CandidateArm{
		CampaignID:     "cmp_test_1",
		AdvertiserName: "Test Advertiser",
		Category:       category,
		BidCPM:         10.0,
		RelevanceScore: 0.9,
	}

	// Select arm initially: impressions = 0 -> ModeColdStart
	outcome, err := engine.SelectArm(category, []CandidateArm{candidate})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Mode != ModeColdStart {
		t.Errorf("expected ModeColdStart, got %v", outcome.Mode)
	}

	// Record 4 impressions
	for i := 0; i < 4; i++ {
		_ = engine.RecordFeedback(category, candidate.CampaignID, EventImpression)
	}

	arm, ok := engine.GetArm(category, candidate.CampaignID)
	if !ok || !arm.InColdStart {
		t.Fatalf("arm should still be in cold start at 4 impressions")
	}

	// Record 5th impression
	_ = engine.RecordFeedback(category, candidate.CampaignID, EventImpression)
	arm, ok = engine.GetArm(category, candidate.CampaignID)
	if !ok || arm.InColdStart {
		t.Fatalf("arm should no longer be in cold start at 5 impressions")
	}
}

func TestBanditEngine_FeedbackUpdates(t *testing.T) {
	cfg := DefaultBanditConfig()
	cfg.PriorAlpha = 1.0
	cfg.PriorBeta = 1.0
	cfg.DecayFactor = 1.0 // Disable decay for direct arithmetic assertions
	engine := NewBanditEngine(cfg, NewSampler(99))

	category := "fintech"
	candidate := CandidateArm{
		CampaignID:     "cmp_stripe_clone",
		AdvertiserName: "Stripe Tooling",
		Category:       category,
		BidCPM:         20.0,
		RelevanceScore: 1.0,
	}

	_ = engine.GetOrCreateArm(category, candidate)

	// Record 10 impressions, 3 clicks, 1 conversion, 7 drops
	for i := 0; i < 10; i++ {
		_ = engine.RecordFeedback(category, candidate.CampaignID, EventImpression)
	}
	for i := 0; i < 3; i++ {
		_ = engine.RecordFeedback(category, candidate.CampaignID, EventClick)
	}
	_ = engine.RecordFeedback(category, candidate.CampaignID, EventConversion)
	for i := 0; i < 7; i++ {
		_ = engine.RecordFeedback(category, candidate.CampaignID, EventDrop)
	}

	arm, ok := engine.GetArm(category, candidate.CampaignID)
	if !ok {
		t.Fatalf("arm not found")
	}

	if arm.Impressions != 10 {
		t.Errorf("expected 10 impressions, got %d", arm.Impressions)
	}
	if arm.Clicks != 3 {
		t.Errorf("expected 3 clicks, got %d", arm.Clicks)
	}
	if arm.Conversions != 1 {
		t.Errorf("expected 1 conversion, got %d", arm.Conversions)
	}
	// Alpha = Prior(1.0) + 3 clicks(3.0) + 1 conversion(2.0) = 6.0
	if arm.Alpha != 6.0 {
		t.Errorf("expected Alpha 6.0, got %f", arm.Alpha)
	}
	// Beta = Prior(1.0) + 7 drops(7.0) = 8.0
	if arm.Beta != 8.0 {
		t.Errorf("expected Beta 8.0, got %f", arm.Beta)
	}
	// Empirical CTR = 3 / 10 = 0.30
	if math.Abs(arm.EmpiricalCTR-0.30) > 1e-6 {
		t.Errorf("expected empirical CTR 0.30, got %f", arm.EmpiricalCTR)
	}
}

func TestBanditEngine_Algorithms(t *testing.T) {
	algorithms := []BanditAlgorithm{
		AlgorithmThompsonSampling,
		AlgorithmUCB1,
		AlgorithmEpsilonGreedy,
	}

	candidates := []CandidateArm{
		{CampaignID: "cmp_a", AdvertiserName: "Adv A", Category: "cat_algo", BidCPM: 10.0, RelevanceScore: 0.8},
		{CampaignID: "cmp_b", AdvertiserName: "Adv B", Category: "cat_algo", BidCPM: 15.0, RelevanceScore: 0.9},
	}

	for _, algo := range algorithms {
		t.Run(string(algo), func(t *testing.T) {
			cfg := DefaultBanditConfig()
			cfg.Algorithm = algo
			engine := NewBanditEngine(cfg, NewSampler(123))

			outcome, err := engine.SelectArm("cat_algo", candidates)
			if err != nil {
				t.Fatalf("algorithm %s failed: %v", algo, err)
			}
			if outcome.SelectedArmID == "" {
				t.Errorf("algorithm %s returned empty selected arm", algo)
			}
			if outcome.EffectiveCPM <= 0 {
				t.Errorf("algorithm %s returned non-positive effective CPM: %f", algo, outcome.EffectiveCPM)
			}
			if len(outcome.CandidateRankings) != 2 {
				t.Errorf("expected 2 rankings, got %d", len(outcome.CandidateRankings))
			}
		})
	}
}

func TestBanditEngine_Concurrency(t *testing.T) {
	cfg := DefaultBanditConfig()
	engine := NewBanditEngine(cfg, NewSampler(1))

	candidates := []CandidateArm{
		{CampaignID: "cmp_c1", AdvertiserName: "C1", Category: "concurrent", BidCPM: 10.0, RelevanceScore: 0.8},
		{CampaignID: "cmp_c2", AdvertiserName: "C2", Category: "concurrent", BidCPM: 15.0, RelevanceScore: 0.9},
		{CampaignID: "cmp_c3", AdvertiserName: "C3", Category: "concurrent", BidCPM: 20.0, RelevanceScore: 0.95},
	}

	var wg sync.WaitGroup
	workers := 25
	iterations := 50

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				outcome, err := engine.SelectArm("concurrent", candidates)
				if err != nil {
					t.Errorf("worker %d selection error: %v", workerID, err)
					return
				}
				_ = engine.RecordFeedback("concurrent", outcome.SelectedArmID, EventImpression)
				if i%3 == 0 {
					_ = engine.RecordFeedback("concurrent", outcome.SelectedArmID, EventClick)
				} else {
					_ = engine.RecordFeedback("concurrent", outcome.SelectedArmID, EventDrop)
				}
			}
		}(w)
	}

	wg.Wait()

	arms := engine.GetAllArms("concurrent")
	if len(arms) != 3 {
		t.Fatalf("expected 3 arms, got %d", len(arms))
	}

	var totalImpr int
	for _, arm := range arms {
		totalImpr += arm.Impressions
	}

	expectedImpr := workers * iterations
	if totalImpr != expectedImpr {
		t.Errorf("expected total impressions %d, got %d", expectedImpr, totalImpr)
	}
}

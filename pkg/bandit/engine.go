package bandit

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

var (
	ErrNoCandidates   = errors.New("no candidate arms provided for selection")
	ErrArmNotFound    = errors.New("bandit arm not found")
	ErrInvalidEvent   = errors.New("invalid feedback event type")
)

// BanditEngine orchestrates Bayesian exploration, multi-armed bandit selection, and feedback tracking.
type BanditEngine struct {
	mu      sync.RWMutex
	config  BanditConfig
	sampler *Sampler
	// arms maps category -> campaignID -> *ArmState
	arms map[string]map[string]*ArmState
	// totalCategoryImpressions maps category -> total impressions across all arms
	categoryImpressions map[string]int
}

// NewBanditEngine creates a new thread-safe contextual bandit engine.
func NewBanditEngine(cfg BanditConfig, sampler *Sampler) *BanditEngine {
	if sampler == nil {
		sampler = NewSampler(0)
	}
	if cfg.PriorAlpha <= 0 {
		cfg.PriorAlpha = 1.0
	}
	if cfg.PriorBeta <= 0 {
		cfg.PriorBeta = 9.0
	}
	if cfg.ColdStartThreshold <= 0 {
		cfg.ColdStartThreshold = 10
	}
	if cfg.UCBConfidenceScale <= 0 {
		cfg.UCBConfidenceScale = 1.414
	}
	if cfg.Epsilon <= 0 {
		cfg.Epsilon = 0.10
	}
	if cfg.DecayFactor <= 0 || cfg.DecayFactor > 1.0 {
		cfg.DecayFactor = 0.995
	}
	if cfg.Algorithm == "" {
		cfg.Algorithm = AlgorithmThompsonSampling
	}

	return &BanditEngine{
		config:              cfg,
		sampler:             sampler,
		arms:                make(map[string]map[string]*ArmState),
		categoryImpressions: make(map[string]int),
	}
}

// Config returns the current bandit configuration.
func (e *BanditEngine) Config() BanditConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// GetOrCreateArm retrieves or initializes an ArmState for a given category and candidate.
func (e *BanditEngine) GetOrCreateArm(category string, candidate CandidateArm) *ArmState {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.getOrCreateArmLocked(category, candidate)
}

func (e *BanditEngine) getOrCreateArmLocked(category string, candidate CandidateArm) *ArmState {
	catMap, ok := e.arms[category]
	if !ok {
		catMap = make(map[string]*ArmState)
		e.arms[category] = catMap
	}

	arm, ok := catMap[candidate.CampaignID]
	if !ok {
		arm = &ArmState{
			CampaignID:     candidate.CampaignID,
			AdvertiserName: candidate.AdvertiserName,
			Category:       category,
			Impressions:    0,
			Clicks:         0,
			Conversions:    0,
			Alpha:          e.config.PriorAlpha,
			Beta:           e.config.PriorBeta,
			EmpiricalCTR:   0.0,
			EmpiricalCVR:   0.0,
			InColdStart:    true,
			LastUpdated:    time.Now().UTC(),
		}
		catMap[candidate.CampaignID] = arm
	}
	return arm
}

// GetArm retrieves an arm state by category and campaign ID.
func (e *BanditEngine) GetArm(category, campaignID string) (*ArmState, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	catMap, ok := e.arms[category]
	if !ok {
		return nil, false
	}
	arm, ok := catMap[campaignID]
	if !ok {
		return nil, false
	}
	copied := *arm
	return &copied, true
}

// GetAllArms returns copies of all arms registered in a category.
func (e *BanditEngine) GetAllArms(category string) []*ArmState {
	e.mu.RLock()
	defer e.mu.RUnlock()

	catMap, ok := e.arms[category]
	if !ok {
		return nil
	}
	result := make([]*ArmState, 0, len(catMap))
	for _, arm := range catMap {
		copied := *arm
		result = append(result, &copied)
	}
	return result
}

// SelectArm evaluates all candidate arms according to the configured multi-armed bandit algorithm.
func (e *BanditEngine) SelectArm(category string, candidates []CandidateArm) (*ArmSelectionOutcome, error) {
	start := time.Now()

	if len(candidates) == 0 {
		return nil, ErrNoCandidates
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Ensure all candidate arms exist in the category state
	armStates := make([]*ArmState, len(candidates))
	for i, c := range candidates {
		armStates[i] = e.getOrCreateArmLocked(category, c)
	}

	totalCatImpressions := e.categoryImpressions[category]
	rankings := make([]ArmRanking, len(candidates))
	var bestExpectedCPM float64

	// First pass: compute expected optimal eCPM for regret benchmark
	for _, c := range candidates {
		arm := e.getOrCreateArmLocked(category, c)
		empiricalOrPriorCTR := arm.EmpiricalCTR
		if arm.Impressions == 0 {
			empiricalOrPriorCTR = BetaMean(arm.Alpha, arm.Beta)
		}
		candidateExpectedCPM := empiricalOrPriorCTR * c.BidCPM * math.Max(0.1, c.RelevanceScore)
		if candidateExpectedCPM > bestExpectedCPM {
			bestExpectedCPM = candidateExpectedCPM
		}
	}

	// Determine cold start candidates
	var coldStartCandidates []int
	for i, arm := range armStates {
		if arm.Impressions < e.config.ColdStartThreshold {
			coldStartCandidates = append(coldStartCandidates, i)
		}
	}

	// Decision mode and scoring
	for i, c := range candidates {
		arm := armStates[i]
		var sampledCTR float64
		mode := ModeExploitation

		if arm.Impressions < e.config.ColdStartThreshold {
			mode = ModeColdStart
		}

		switch e.config.Algorithm {
		case AlgorithmThompsonSampling:
			// Bayesian Thompson Sampling: sample theta ~ Beta(alpha, beta)
			sampledCTR = e.sampler.SampleBeta(arm.Alpha, arm.Beta)
			arm.LastSampledVal = sampledCTR
			if mode != ModeColdStart {
				// If sampled higher than expected mean, it counts as exploration
				if sampledCTR > BetaMean(arm.Alpha, arm.Beta) {
					mode = ModeExploration
				}
			}

		case AlgorithmUCB1:
			// Upper Confidence Bound (UCB1)
			empiricalMean := arm.EmpiricalCTR
			if arm.Impressions == 0 {
				empiricalMean = BetaMean(arm.Alpha, arm.Beta)
			}
			var explorationBonus float64
			if totalCatImpressions > 0 && arm.Impressions > 0 {
				explorationBonus = e.config.UCBConfidenceScale * math.Sqrt((2.0*math.Log(float64(totalCatImpressions)))/float64(arm.Impressions))
			} else {
				// Infinite or maximum exploration incentive for unvisited arm
				explorationBonus = 1.0
			}
			sampledCTR = empiricalMean + explorationBonus
			if explorationBonus > 0.05 {
				mode = ModeExploration
			}

		case AlgorithmEpsilonGreedy:
			// Epsilon-Greedy
			isExploring := e.sampler.SampleUniform() < e.config.Epsilon
			if isExploring {
				mode = ModeExploration
				// Random perturbed exploration score
				sampledCTR = e.sampler.SampleUniform() * 0.5
			} else {
				sampledCTR = arm.EmpiricalCTR
				if arm.Impressions == 0 {
					sampledCTR = BetaMean(arm.Alpha, arm.Beta)
				}
			}

		default:
			sampledCTR = BetaMean(arm.Alpha, arm.Beta)
		}

		// Ensure non-negative CTR
		if sampledCTR < 0.0 {
			sampledCTR = 0.0001
		}

		// Effective CPM = Sampled CTR * BidCPM * RelevanceScore
		relevanceFactor := math.Max(0.1, c.RelevanceScore)
		effectiveCPM := sampledCTR * c.BidCPM * relevanceFactor

		rankings[i] = ArmRanking{
			CampaignID:     c.CampaignID,
			AdvertiserName: c.AdvertiserName,
			BidCPM:         c.BidCPM,
			RelevanceScore: c.RelevanceScore,
			SampledCTR:     sampledCTR,
			EffectiveCPM:   effectiveCPM,
			Mode:           mode,
		}
	}

	// Sort rankings descending by EffectiveCPM
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].EffectiveCPM > rankings[j].EffectiveCPM
	})

	for r := range rankings {
		rankings[r].Rank = r + 1
	}

	winner := rankings[0]
	var winnerCandidate CandidateArm
	for _, c := range candidates {
		if c.CampaignID == winner.CampaignID {
			winnerCandidate = c
			break
		}
	}

	winnerArm := e.getOrCreateArmLocked(category, winnerCandidate)
	expectedCTR := BetaMean(winnerArm.Alpha, winnerArm.Beta)

	// Regret: gap between theoretical optimal eCPM and chosen eCPM
	estimatedRegret := math.Max(0.0, bestExpectedCPM-winner.EffectiveCPM)

	outcome := &ArmSelectionOutcome{
		SelectedArmID:     winner.CampaignID,
		AdvertiserName:    winner.AdvertiserName,
		Category:          category,
		Mode:              winner.Mode,
		Algorithm:         e.config.Algorithm,
		SampledScore:      winner.SampledCTR,
		ExpectedCTR:       expectedCTR,
		EffectiveCPM:      winner.EffectiveCPM,
		CandidateCount:    len(candidates),
		CandidateRankings: rankings,
		EstimatedRegret:   estimatedRegret,
		ExecutionTimeUs:   time.Since(start).Microseconds(),
	}

	return outcome, nil
}

// RecordFeedback updates the Bayesian posterior parameters of the selected arm.
func (e *BanditEngine) RecordFeedback(category string, campaignID string, eventType FeedbackEventType) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	catMap, ok := e.arms[category]
	if !ok {
		return ErrArmNotFound
	}
	arm, ok := catMap[campaignID]
	if !ok {
		return ErrArmNotFound
	}

	// Apply decay factor if configured (discounting old observations over time)
	if e.config.DecayFactor > 0.0 && e.config.DecayFactor < 1.0 {
		arm.Alpha = (arm.Alpha * e.config.DecayFactor) + (e.config.PriorAlpha * (1.0 - e.config.DecayFactor))
		arm.Beta = (arm.Beta * e.config.DecayFactor) + (e.config.PriorBeta * (1.0 - e.config.DecayFactor))
	}

	switch eventType {
	case EventImpression:
		arm.Impressions++
		e.categoryImpressions[category]++

	case EventClick:
		arm.Clicks++
		arm.Alpha += 1.0

	case EventConversion:
		arm.Conversions++
		// High-value reward: boost posterior alpha by 2.0
		arm.Alpha += 2.0

	case EventDrop:
		// Negative signal: no interaction / discarded
		arm.Beta += 1.0

	case EventTimeout:
		// Weak negative signal
		arm.Beta += 0.5

	default:
		return fmt.Errorf("%w: %s", ErrInvalidEvent, eventType)
	}

	// Update empirical metrics
	if arm.Impressions > 0 {
		arm.EmpiricalCTR = float64(arm.Clicks) / float64(arm.Impressions)
		arm.EmpiricalCVR = float64(arm.Conversions) / float64(arm.Impressions)
	}
	arm.InColdStart = arm.Impressions < e.config.ColdStartThreshold
	arm.LastUpdated = time.Now().UTC()

	return nil
}

// SimulateBatch runs a multi-round bandit simulation comparing candidate arms under ground truth CTRs.
func (e *BanditEngine) SimulateBatch(scenarioName, category string, candidates []CandidateArm, rounds int, groundTruthCTRs map[string]float64) (*BatchSimulationSummary, error) {
	if rounds <= 0 {
		rounds = 100
	}

	summary := &BatchSimulationSummary{
		ScenarioName:     scenarioName,
		Category:         category,
		TotalRounds:      rounds,
		CumulativeRegret: 0.0,
		Rounds:           make([]*ArmSelectionOutcome, 0, rounds),
	}

	// Find optimal theoretical eCPM for true regret calculation
	var optimalTrueECPM float64
	for _, c := range candidates {
		trueCTR := groundTruthCTRs[c.CampaignID]
		ecpm := trueCTR * c.BidCPM * math.Max(0.1, c.RelevanceScore)
		if ecpm > optimalTrueECPM {
			optimalTrueECPM = ecpm
		}
	}

	for r := 0; r < rounds; r++ {
		outcome, err := e.SelectArm(category, candidates)
		if err != nil {
			return nil, err
		}

		// Always record impression
		_ = e.RecordFeedback(category, outcome.SelectedArmID, EventImpression)
		summary.TotalImpressions++

		// True underlying probability of click
		trueCTR := groundTruthCTRs[outcome.SelectedArmID]
		if e.sampler.SampleUniform() < trueCTR {
			_ = e.RecordFeedback(category, outcome.SelectedArmID, EventClick)
			summary.TotalClicks++

			// Check for conversion (assume 20% conversion conditional on click)
			if e.sampler.SampleUniform() < 0.20 {
				_ = e.RecordFeedback(category, outcome.SelectedArmID, EventConversion)
				summary.TotalConversions++
			}
		} else {
			_ = e.RecordFeedback(category, outcome.SelectedArmID, EventDrop)
		}

		// Cumulative true regret
		selectedTrueECPM := trueCTR * outcome.EffectiveCPM / math.Max(0.001, outcome.SampledScore)
		roundRegret := math.Max(0.0, optimalTrueECPM-selectedTrueECPM)
		summary.CumulativeRegret += roundRegret

		if r < 10 || r%25 == 0 || r == rounds-1 {
			summary.Rounds = append(summary.Rounds, outcome)
		}
	}

	if summary.TotalImpressions > 0 {
		summary.AverageCTR = float64(summary.TotalClicks) / float64(summary.TotalImpressions)
	}
	summary.Arms = e.GetAllArms(category)

	return summary, nil
}

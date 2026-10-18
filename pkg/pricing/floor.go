package pricing

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// FloorOptimizer calculates real-time dynamic reserve prices for publishers to maximize yield.
type FloorOptimizer struct {
	Config       DynamicFloorConfig
	publishers   map[string]*PublisherInventoryState
	mu           sync.RWMutex
}

// NewFloorOptimizer instantiates a dynamic floor pricing engine with the supplied configuration.
func NewFloorOptimizer(cfg DynamicFloorConfig) *FloorOptimizer {
	if cfg.BaseFloorCPM <= 0 {
		cfg.BaseFloorCPM = 1.00
	}
	if cfg.MinFloorCPM <= 0 {
		cfg.MinFloorCPM = 0.25
	}
	if cfg.MaxFloorCPM <= cfg.MinFloorCPM {
		cfg.MaxFloorCPM = 12.00
	}
	if cfg.TargetFillRate <= 0 || cfg.TargetFillRate > 1.0 {
		cfg.TargetFillRate = 0.70
	}
	if cfg.FillRateSensitivity <= 0 {
		cfg.FillRateSensitivity = 0.65
	}
	if cfg.HighCompetitionThreshold <= 0 {
		cfg.HighCompetitionThreshold = 4
	}
	if cfg.HighCompetitionBonus <= 0 {
		cfg.HighCompetitionBonus = 0.08
	}

	return &FloorOptimizer{
		Config:     cfg,
		publishers: make(map[string]*PublisherInventoryState),
	}
}

// CalculateFloor computes the real-time reserve CPM for a specific publisher, category, and competition level.
func (fo *FloorOptimizer) CalculateFloor(publisherID string, category string, activeBidders int, agentTrustScore float64) *FloorCalculationResult {
	fo.mu.Lock()
	state, exists := fo.publishers[publisherID]
	if !exists {
		state = &PublisherInventoryState{
			PublisherID:         publisherID,
			Category:            category,
			TotalAuctions:       10,
			FilledAuctions:      7, // Initial default assumed 70% fill rate
			CurrentFillRate:     fo.Config.TargetFillRate,
			AvgClearingPriceCPM: fo.Config.BaseFloorCPM * 1.5,
			LastUpdated:         time.Now(),
		}
		fo.publishers[publisherID] = state
	}
	fo.mu.Unlock()

	fo.mu.RLock()
	currentFillRate := state.CurrentFillRate
	fo.mu.RUnlock()

	// 1. Determine Category Base Floor
	baseFloor := fo.Config.BaseFloorCPM
	if catBase, ok := fo.Config.CategoryFloors[category]; ok && catBase > 0 {
		baseFloor = catBase
	}

	// 2. Compute Fill Rate Feedback Factor
	// If current fill rate is higher than target, stiffen the floor to extract higher surplus.
	// If current fill rate is lower than target, soften the floor to stimulate clearance.
	fillRateError := currentFillRate - fo.Config.TargetFillRate
	fillRateFactor := 1.0 + (fo.Config.FillRateSensitivity * fillRateError)
	if fillRateFactor < 0.40 {
		fillRateFactor = 0.40
	} else if fillRateFactor > 2.00 {
		fillRateFactor = 2.00
	}

	// 3. Compute Competition Density Factor
	competitionFactor := 1.0
	if activeBidders > fo.Config.HighCompetitionThreshold {
		extraBidders := float64(activeBidders - fo.Config.HighCompetitionThreshold)
		competitionFactor = 1.0 + (extraBidders * fo.Config.HighCompetitionBonus)
	} else if activeBidders <= 1 {
		// Monopoly or single bidder: reduce floor slightly to prevent zero-clearance
		competitionFactor = 0.85
	}

	// 4. Compute Agent Trust / Reputation Adjustment
	reputationAdjustment := 0.0
	if agentTrustScore >= 0.90 {
		// Tier-1 zero-jailbreak compliant agents receive a floor discount
		discountRatio := math.Min(fo.Config.AgentReputationDiscount, (agentTrustScore-0.90)*1.5)
		reputationAdjustment = -discountRatio
	} else if agentTrustScore < 0.50 {
		// Low reputation / unverified agent penalty
		reputationAdjustment = 0.20
	}

	// 5. Aggregate Computed Floor
	rawMultiplier := fillRateFactor * competitionFactor * (1.0 + reputationAdjustment)
	computedFloor := baseFloor * rawMultiplier

	// 6. Apply Hard Guardrails (Min/Max Clamping)
	finalFloor := computedFloor
	isClamped := false
	clampReason := ""

	if finalFloor < fo.Config.MinFloorCPM {
		finalFloor = fo.Config.MinFloorCPM
		isClamped = true
		clampReason = fmt.Sprintf("Clamped to MinFloorCPM ($%.2f)", fo.Config.MinFloorCPM)
	} else if finalFloor > fo.Config.MaxFloorCPM {
		finalFloor = fo.Config.MaxFloorCPM
		isClamped = true
		clampReason = fmt.Sprintf("Clamped to MaxFloorCPM ($%.2f)", fo.Config.MaxFloorCPM)
	}

	// Round to 3 decimal places for currency stability
	finalFloor = math.Round(finalFloor*1000) / 1000

	return &FloorCalculationResult{
		PublisherID:          publisherID,
		Category:             category,
		BaseFloorCPM:         baseFloor,
		FillRateFactor:       math.Round(fillRateFactor*1000) / 1000,
		CompetitionFactor:    math.Round(competitionFactor*1000) / 1000,
		ReputationAdjustment: math.Round(reputationAdjustment*1000) / 1000,
		ComputedFloorCPM:     math.Round(computedFloor*1000) / 1000,
		FinalFloorCPM:        finalFloor,
		IsClamped:            isClamped,
		ClampReason:          clampReason,
	}
}

// RecordAuctionOutcome updates the publisher's running fill rate and moving average clearing price.
func (fo *FloorOptimizer) RecordAuctionOutcome(publisherID string, category string, cleared bool, clearingPriceCPM float64) {
	fo.mu.Lock()
	defer fo.mu.Unlock()

	state, exists := fo.publishers[publisherID]
	if !exists {
		state = &PublisherInventoryState{
			PublisherID:     publisherID,
			Category:        category,
			TotalAuctions:   0,
			FilledAuctions:  0,
			CurrentFillRate: fo.Config.TargetFillRate,
			LastUpdated:     time.Now(),
		}
		fo.publishers[publisherID] = state
	}

	state.TotalAuctions++
	if cleared {
		state.FilledAuctions++
		// USD Yield per impression = CPM / 1000
		yieldUSD := clearingPriceCPM / 1000.0
		state.TotalYieldUSD += yieldUSD

		// Exponential Moving Average (alpha = 0.15) for clearing price
		if state.AvgClearingPriceCPM == 0 {
			state.AvgClearingPriceCPM = clearingPriceCPM
		} else {
			state.AvgClearingPriceCPM = (0.15 * clearingPriceCPM) + (0.85 * state.AvgClearingPriceCPM)
		}
	}

	// Calculate EWMA for fill rate
	currentBatchOutcome := 0.0
	if cleared {
		currentBatchOutcome = 1.0
	}
	state.CurrentFillRate = (0.10 * currentBatchOutcome) + (0.90 * state.CurrentFillRate)
	state.LastUpdated = time.Now()
}

// SetPublisherMetrics manually primes or updates publisher historical statistics.
func (fo *FloorOptimizer) SetPublisherMetrics(publisherID string, category string, fillRate float64, avgClearingCPM float64) {
	fo.mu.Lock()
	defer fo.mu.Unlock()

	fo.publishers[publisherID] = &PublisherInventoryState{
		PublisherID:         publisherID,
		Category:            category,
		TotalAuctions:       50,
		FilledAuctions:      int(50.0 * fillRate),
		CurrentFillRate:     fillRate,
		AvgClearingPriceCPM: avgClearingCPM,
		LastUpdated:         time.Now(),
	}
}

// GetPublisherState retrieves the current inventory state for inspection.
func (fo *FloorOptimizer) GetPublisherState(publisherID string) *PublisherInventoryState {
	fo.mu.RLock()
	defer fo.mu.RUnlock()

	if st, ok := fo.publishers[publisherID]; ok {
		cpy := *st
		return &cpy
	}
	return nil
}

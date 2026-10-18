package pricing

import (
	"math"
	"sort"
	"sync"
)

// BidShader adjusts buyer agent bids to optimize buyer surplus and avoid the winner's curse.
type BidShader struct {
	Config                BidShadingConfig
	historicalWins        []bool
	historicalBids        []float64
	historicalClearings   []float64
	mu                    sync.RWMutex
}

// NewBidShader constructs a new bid shading optimizer.
func NewBidShader(cfg BidShadingConfig) *BidShader {
	if cfg.MinShadingRatio <= 0 {
		cfg.MinShadingRatio = 0.50
	}
	if cfg.MaxShadingRatio <= cfg.MinShadingRatio || cfg.MaxShadingRatio > 1.0 {
		cfg.MaxShadingRatio = 0.95
	}
	if cfg.ReserveSafetyMarginCPM < 0 {
		cfg.ReserveSafetyMarginCPM = 0.10
	}
	if cfg.TargetWinRate <= 0 || cfg.TargetWinRate > 1.0 {
		cfg.TargetWinRate = 0.65
	}
	if cfg.HistoricalWindowSize <= 0 {
		cfg.HistoricalWindowSize = 100
	}

	return &BidShader{
		Config: cfg,
	}
}

// ShadeBid computes the shaded bid amount for a candidate buyer.
func (bs *BidShader) ShadeBid(rawBidCPM float64, category string, estimatedFloorCPM float64, competitorCount int) *BidShadingResult {
	if !bs.Config.Enabled || rawBidCPM <= 0 {
		return &BidShadingResult{
			OriginalBidCPM:    rawBidCPM,
			ShadedBidCPM:      rawBidCPM,
			ShadingRatio:      1.0,
			EstimatedFloorCPM: estimatedFloorCPM,
			SurplusSavedCPM:   0.0,
			AlgorithmUsed:     "none_disabled",
			WinProbabilityEst: 0.85,
		}
	}

	bs.mu.RLock()
	currentWinRate := bs.internalWinRate()
	historicalMedian := bs.internalPercentileClearing(0.50)
	bs.mu.RUnlock()

	var ratio float64
	winProb := 0.70

	switch bs.Config.Algorithm {
	case AlgorithmVickreyEmulator:
		// Order statistics for N competing bidders: E[B_(2)] / B_(1) = (N-1)/N
		if competitorCount <= 1 {
			ratio = bs.Config.MinShadingRatio // High shading power when alone
			winProb = 0.95
		} else {
			orderRatio := float64(competitorCount-1) / float64(competitorCount)
			ratio = math.Max(bs.Config.MinShadingRatio, math.Min(bs.Config.MaxShadingRatio, orderRatio))
			winProb = 1.0 / float64(competitorCount) * 1.5
		}

	case AlgorithmWinRateOptimizer:
		// Adjust shading based on win rate discrepancy
		winRateDiff := currentWinRate - bs.Config.TargetWinRate
		// If win rate is too high (> Target), shade more (lower ratio); if too low (< Target), shade less
		baseRatio := (bs.Config.MinShadingRatio + bs.Config.MaxShadingRatio) / 2.0
		ratio = baseRatio - (winRateDiff * 0.35)
		if ratio < bs.Config.MinShadingRatio {
			ratio = bs.Config.MinShadingRatio
		} else if ratio > bs.Config.MaxShadingRatio {
			ratio = bs.Config.MaxShadingRatio
		}
		winProb = currentWinRate

	case AlgorithmFloorAwareHybrid:
		fallthrough
	default:
		// Hybrid approach: Combines competitor count order statistics with median clearing and reserve floor awareness
		compRatio := 0.75
		if competitorCount > 1 {
			compRatio = float64(competitorCount) / float64(competitorCount+1)
		} else {
			compRatio = 0.58
		}

		if historicalMedian > 0 && historicalMedian < rawBidCPM {
			// Blend order ratio with historical median clearing price
			historicalRatio := historicalMedian / rawBidCPM
			compRatio = (0.60 * compRatio) + (0.40 * historicalRatio)
		}

		ratio = math.Max(bs.Config.MinShadingRatio, math.Min(bs.Config.MaxShadingRatio, compRatio))
		winProb = math.Min(0.95, math.Max(0.20, ratio*0.90))
	}

	// Calculate raw shaded bid
	shadedBid := rawBidCPM * ratio

	// Floor-Aware Enforcement: Shaded bid must not fall below the estimated dynamic floor plus safety margin
	minSafeBid := estimatedFloorCPM + bs.Config.ReserveSafetyMarginCPM
	if rawBidCPM > minSafeBid && shadedBid < minSafeBid {
		shadedBid = minSafeBid
		ratio = shadedBid / rawBidCPM
	} else if rawBidCPM <= minSafeBid {
		// Cannot shade if raw bid is already at or below floor safety margin
		shadedBid = rawBidCPM
		ratio = 1.0
	}

	// Round to 3 decimal places
	shadedBid = math.Round(shadedBid*1000) / 1000
	surplusSavedCPM := math.Max(0.0, rawBidCPM-shadedBid)

	return &BidShadingResult{
		OriginalBidCPM:    rawBidCPM,
		ShadedBidCPM:      shadedBid,
		ShadingRatio:      math.Round(ratio*1000) / 1000,
		EstimatedFloorCPM: estimatedFloorCPM,
		SurplusSavedCPM:   math.Round(surplusSavedCPM*1000) / 1000,
		AlgorithmUsed:     bs.Config.Algorithm,
		WinProbabilityEst: math.Round(winProb*1000) / 1000,
	}
}

// RecordAuctionResult feeds real auction clearing outcomes back into the shader.
func (bs *BidShader) RecordAuctionResult(clearingPriceCPM float64, won bool, bidCPM float64) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	bs.historicalWins = append(bs.historicalWins, won)
	bs.historicalBids = append(bs.historicalBids, bidCPM)
	if won {
		bs.historicalClearings = append(bs.historicalClearings, clearingPriceCPM)
	}

	// Trim window size
	limit := bs.Config.HistoricalWindowSize
	if len(bs.historicalWins) > limit {
		bs.historicalWins = bs.historicalWins[len(bs.historicalWins)-limit:]
	}
	if len(bs.historicalBids) > limit {
		bs.historicalBids = bs.historicalBids[len(bs.historicalBids)-limit:]
	}
	if len(bs.historicalClearings) > limit {
		bs.historicalClearings = bs.historicalClearings[len(bs.historicalClearings)-limit:]
	}
}

// CalculateWinRate returns the empirical win rate over the historical window.
func (bs *BidShader) CalculateWinRate() float64 {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.internalWinRate()
}

func (bs *BidShader) internalWinRate() float64 {
	if len(bs.historicalWins) == 0 {
		return bs.Config.TargetWinRate
	}
	wins := 0
	for _, w := range bs.historicalWins {
		if w {
			wins++
		}
	}
	return float64(wins) / float64(len(bs.historicalWins))
}

func (bs *BidShader) internalPercentileClearing(p float64) float64 {
	if len(bs.historicalClearings) == 0 {
		return 0.0
	}
	copied := make([]float64, len(bs.historicalClearings))
	copy(copied, bs.historicalClearings)
	sort.Float64s(copied)

	idx := int(float64(len(copied)-1) * p)
	return copied[idx]
}

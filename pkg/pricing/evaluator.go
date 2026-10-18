package pricing

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// PricingEvaluator coordinates publisher floor optimization and buyer bid shading in auction rounds.
type PricingEvaluator struct {
	FloorOpt       *FloorOptimizer
	Shader         *BidShader
	StaticFloorCPM float64
	MinRelevance   float64
	MinTrust       float64
	mu             sync.Mutex
}

// NewPricingEvaluator creates a unified auction pricing evaluator.
func NewPricingEvaluator(floorOpt *FloorOptimizer, shader *BidShader) *PricingEvaluator {
	if floorOpt == nil {
		floorOpt = NewFloorOptimizer(DefaultDynamicFloorConfig())
	}
	if shader == nil {
		shader = NewBidShader(DefaultBidShadingConfig())
	}

	return &PricingEvaluator{
		FloorOpt:       floorOpt,
		Shader:         shader,
		StaticFloorCPM: 1.00,
		MinRelevance:   0.35,
		MinTrust:       0.30,
	}
}

// EvaluateAuction simulates or executes an auction round with dynamic floor and bid shading.
func (pe *PricingEvaluator) EvaluateAuction(publisherID string, category string, rawCandidates []CandidateBid, applyShading bool) *AuctionEvaluationResult {
	start := time.Now()
	auctionID := generateID("auc_eval")

	// 1. Calculate Dynamic Publisher Floor
	biddersCount := len(rawCandidates)
	avgTrust := 0.85
	if biddersCount > 0 {
		sumTrust := 0.0
		for _, c := range rawCandidates {
			sumTrust += c.TrustScore
		}
		avgTrust = sumTrust / float64(biddersCount)
	}

	floorResult := pe.FloorOpt.CalculateFloor(publisherID, category, biddersCount, avgTrust)
	dynamicFloorCPM := floorResult.FinalFloorCPM

	// 2. Process Candidates & Apply Bid Shading if requested
	var evaluatedBids []CandidateBid
	var originalBids []CandidateBid

	for _, cand := range rawCandidates {
		orig := cand
		originalBids = append(originalBids, orig)

		shadedBid := cand.RawBidCPM
		if applyShading {
			shResult := pe.Shader.ShadeBid(cand.RawBidCPM, category, dynamicFloorCPM, biddersCount)
			shadedBid = shResult.ShadedBidCPM
		}

		cCopy := cand
		cCopy.ShadedBidCPM = shadedBid
		// Quality-Weighted Composite Score: Bid (50%) + Relevance (30%) + Trust (20%)
		cCopy.CompositeScore = (shadedBid * 0.50) + (cCopy.RelevanceScore * 3.0) + (cCopy.TrustScore * 2.0)
		evaluatedBids = append(evaluatedBids, cCopy)
	}

	// 3. Filter Candidates against Dynamic Floor, Relevance, and Trust
	var qualified []CandidateBid
	var rejected []RejectedCandidate

	for _, c := range evaluatedBids {
		if c.ShadedBidCPM < dynamicFloorCPM {
			rejected = append(rejected, RejectedCandidate{
				CampaignID:     c.CampaignID,
				AdvertiserName: c.AdvertiserName,
				BidCPM:         c.ShadedBidCPM,
				Reason:         RejectBelowDynamicFloor,
				FloorPriceCPM:  dynamicFloorCPM,
				Detail:         fmt.Sprintf("Bid $%.3f below dynamic reserve $%.3f", c.ShadedBidCPM, dynamicFloorCPM),
			})
			continue
		}
		if c.RelevanceScore < pe.MinRelevance {
			rejected = append(rejected, RejectedCandidate{
				CampaignID:     c.CampaignID,
				AdvertiserName: c.AdvertiserName,
				BidCPM:         c.ShadedBidCPM,
				Reason:         RejectLowRelevance,
				FloorPriceCPM:  dynamicFloorCPM,
				Detail:         fmt.Sprintf("Relevance %.2f below threshold %.2f", c.RelevanceScore, pe.MinRelevance),
			})
			continue
		}
		if c.TrustScore < pe.MinTrust {
			rejected = append(rejected, RejectedCandidate{
				CampaignID:     c.CampaignID,
				AdvertiserName: c.AdvertiserName,
				BidCPM:         c.ShadedBidCPM,
				Reason:         RejectLowTrust,
				FloorPriceCPM:  dynamicFloorCPM,
				Detail:         fmt.Sprintf("Trust %.2f below minimum %.2f", c.TrustScore, pe.MinTrust),
			})
			continue
		}
		qualified = append(qualified, c)
	}

	// Sort qualified by composite score descending
	sort.Slice(qualified, func(i, j int) bool {
		return qualified[i].CompositeScore > qualified[j].CompositeScore
	})

	// 4. Determine Dynamic Auction Winner & Clearing Price
	cleared := len(qualified) > 0
	var winner *CandidateBid
	clearingPriceCPM := 0.0
	publisherYieldUSD := 0.0
	buyerSavingsUSD := 0.0

	if cleared {
		w := qualified[0]
		winner = &w

		if len(qualified) > 1 {
			// Second-price clearing subject to dynamic floor
			runnerUpBid := qualified[1].ShadedBidCPM
			clearingPriceCPM = math.Max(dynamicFloorCPM, runnerUpBid+0.01)
		} else {
			// Monopoly winner clears exactly at the dynamic floor
			clearingPriceCPM = dynamicFloorCPM
		}

		// Cap clearing price at winner's shaded bid
		if clearingPriceCPM > winner.ShadedBidCPM {
			clearingPriceCPM = winner.ShadedBidCPM
		}

		publisherYieldUSD = clearingPriceCPM / 1000.0 // 1 impression
		if applyShading {
			savingsCPM := winner.RawBidCPM - clearingPriceCPM
			if savingsCPM > 0 {
				buyerSavingsUSD = savingsCPM / 1000.0
			}
		}

		// Feed back outcome to components
		pe.FloorOpt.RecordAuctionOutcome(publisherID, category, true, clearingPriceCPM)
		pe.Shader.RecordAuctionResult(clearingPriceCPM, true, winner.ShadedBidCPM)
	} else {
		pe.FloorOpt.RecordAuctionOutcome(publisherID, category, false, 0.0)
	}

	// 5. Baseline Comparison: Static Floor ($1.00 CPM) Simulation
	var staticQualified []CandidateBid
	for _, c := range originalBids {
		if c.RawBidCPM >= pe.StaticFloorCPM && c.RelevanceScore >= pe.MinRelevance && c.TrustScore >= pe.MinTrust {
			cCopy := c
			cCopy.CompositeScore = (c.RawBidCPM * 0.50) + (c.RelevanceScore * 3.0) + (c.TrustScore * 2.0)
			staticQualified = append(staticQualified, cCopy)
		}
	}

	sort.Slice(staticQualified, func(i, j int) bool {
		return staticQualified[i].CompositeScore > staticQualified[j].CompositeScore
	})

	staticCleared := len(staticQualified) > 0
	var staticWinner *CandidateBid
	staticClearingCPM := 0.0
	staticYieldUSD := 0.0

	if staticCleared {
		sw := staticQualified[0]
		staticWinner = &sw
		if len(staticQualified) > 1 {
			staticClearingCPM = math.Max(pe.StaticFloorCPM, staticQualified[1].RawBidCPM+0.01)
		} else {
			staticClearingCPM = pe.StaticFloorCPM
		}
		if staticClearingCPM > staticWinner.RawBidCPM {
			staticClearingCPM = staticWinner.RawBidCPM
		}
		staticYieldUSD = staticClearingCPM / 1000.0
	}

	// Yield Uplift
	yieldUpliftUSD := publisherYieldUSD - staticYieldUSD
	yieldUpliftPct := 0.0
	if staticYieldUSD > 0 {
		yieldUpliftPct = (yieldUpliftUSD / staticYieldUSD) * 100.0
	} else if publisherYieldUSD > 0 {
		yieldUpliftPct = 100.0 // Gained clearance where static floor failed
	}

	duration := time.Since(start)

	return &AuctionEvaluationResult{
		AuctionID:               auctionID,
		Category:                category,
		PublisherID:             publisherID,
		DynamicFloor:            floorResult,
		StaticFloorCPM:          pe.StaticFloorCPM,
		TotalBidsReceived:       len(rawCandidates),
		OriginalBids:            originalBids,
		EvaluatedBids:           evaluatedBids,
		QualifiedCandidates:     qualified,
		RejectedCandidates:      rejected,
		Winner:                  winner,
		ClearingPriceCPM:        math.Round(clearingPriceCPM*1000) / 1000,
		PublisherYieldUSD:       publisherYieldUSD,
		BuyerSavingsUSD:         buyerSavingsUSD,
		StaticWouldHaveCleared:  staticCleared,
		StaticWinner:            staticWinner,
		StaticClearingPriceCPM:  math.Round(staticClearingCPM*1000) / 1000,
		StaticPublisherYieldUSD: staticYieldUSD,
		Cleared:                 cleared,
		YieldUpliftUSD:          yieldUpliftUSD,
		YieldUpliftPercent:      math.Round(yieldUpliftPct*10) / 10,
		ExecutionDurationUs:     duration.Microseconds(),
		Timestamp:               time.Now(),
	}
}

func generateID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

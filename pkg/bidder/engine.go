package bidder

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
	"github.com/Noooblien/agentad/pkg/placement"
)

// BidderAgent coordinates automated campaign bidding, valuation, and cryptographic signing.
type BidderAgent struct {
	mu              sync.RWMutex
	advertiserID    string
	advertiserName  string
	signingKey      *attest.KeyPair
	campaigns       map[string]*CampaignConfig
	reports         map[string]*CampaignReport
	recentResponses []*BidResponse
	totalRequests   int64
	totalBids       int64
	totalNoBids     int64
	totalWins       int64
	totalGrossSpend float64
	totalShadedSpend float64
	totalShadingSaved float64
}

// NewBidderAgent initializes a new demand-side automated bidding agent.
func NewBidderAgent(advertiserID, name string, key *attest.KeyPair) *BidderAgent {
	if key == nil {
		generated, _ := attest.GenerateKeyPair()
		key = generated
	}

	return &BidderAgent{
		advertiserID:    advertiserID,
		advertiserName:  name,
		signingKey:      key,
		campaigns:       make(map[string]*CampaignConfig),
		reports:         make(map[string]*CampaignReport),
		recentResponses: make([]*BidResponse, 0, 64),
	}
}

// RegisterCampaign registers a new advertiser campaign for real-time bid arbitration.
func (b *BidderAgent) RegisterCampaign(camp CampaignConfig) error {
	if camp.CampaignID == "" {
		return fmt.Errorf("campaign_id cannot be empty")
	}
	if camp.MaxBidCPM <= 0 {
		return fmt.Errorf("max_bid_cpm must be positive")
	}
	if camp.DailyBudgetUSD <= 0 {
		camp.DailyBudgetUSD = 100.00 // Default $100 daily budget
	}
	if camp.ShadingDiscount <= 0 {
		camp.ShadingDiscount = 0.12 // Default 12% shading
	}
	if camp.Strategy == "" {
		camp.Strategy = StrategyExpectedValue
	}
	if camp.SigningKey == nil {
		camp.SigningKey = b.signingKey
	}
	if len(camp.TargetSlots) == 0 {
		camp.TargetSlots = []placement.SlotArchetype{
			placement.SlotToolDefinition,
			placement.SlotToolResponse,
			placement.SlotSynthesisEnvelope,
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.campaigns[camp.CampaignID] = &camp
	b.reports[camp.CampaignID] = &CampaignReport{
		CampaignID:      camp.CampaignID,
		AdvertiserName:  camp.AdvertiserName,
		Strategy:        camp.Strategy,
		BudgetRemaining: camp.DailyBudgetUSD - camp.TotalSpentUSD,
	}

	return nil
}

// EvaluateBidRequest processes an inbound RTB auction request and generates an optimal bid.
func (b *BidderAgent) EvaluateBidRequest(req BidRequest) *BidResponse {
	t0 := time.Now()

	b.mu.Lock()
	b.totalRequests++
	b.mu.Unlock()

	b.mu.RLock()
	defer b.mu.RUnlock()

	var bestCamp *CampaignConfig
	var bestGrossBid float64
	var bestShadedBid float64
	var bestRelevance float64
	var bestCTR float64
	var highestExpectedValue float64
	var noBidReason string = "No matching campaigns found"

	normQuery := strings.ToLower(req.ScrubbedIntent)

	for _, camp := range b.campaigns {
		// 1. Budget Pacing & Exhaustion Check
		remainingBudget := camp.DailyBudgetUSD - camp.TotalSpentUSD
		if remainingBudget <= 0.05 {
			noBidReason = "Daily budget depleted"
			continue
		}

		// 2. Slot Compatibility Check
		slotAllowed := false
		for _, slot := range camp.TargetSlots {
			if slot == req.SlotType {
				slotAllowed = true
				break
			}
		}
		if !slotAllowed {
			noBidReason = fmt.Sprintf("Slot archetype %s not targeted", req.SlotType)
			continue
		}

		// 3. Negative Keyword Filtering
		hasNegative := false
		for _, neg := range camp.NegativeKeywords {
			if strings.Contains(normQuery, strings.ToLower(neg)) {
				hasNegative = true
				break
			}
		}
		if hasNegative {
			noBidReason = "Negative keyword match"
			continue
		}

		// 4. Semantic Relevance & Keyword Alignment
		relevance := b.calculateRelevance(camp, req)
		if relevance < 0.20 {
			noBidReason = "Relevance below minimum matching threshold (0.20)"
			continue
		}

		// 5. Predicted CTR and Conversion Rate
		pCTR := math.Min(0.12, 0.025+(relevance*0.065))
		pCVR := math.Min(0.25, 0.040+(relevance*0.080))

		// 6. Valuation by Strategy
		var grossBid float64
		switch camp.Strategy {
		case StrategyFixedCPM:
			grossBid = camp.MaxBidCPM
		case StrategyTargetCPA:
			// eCPM = CPA * pCTR * pCVR * 1000
			targetCPA := camp.TargetCPAUSD
			if targetCPA <= 0 {
				targetCPA = 25.00
			}
			grossBid = targetCPA * pCTR * pCVR * 1000.0
		case StrategyCompetitiveShaded:
			grossBid = camp.MaxBidCPM * (0.50 + (0.50 * relevance))
		case StrategyExpectedValue:
			fallthrough
		default:
			// eCPM scaled by relevance and CTR multiplier
			grossBid = camp.MaxBidCPM * (0.40 + (0.60 * relevance)) * (pCTR / 0.045)
		}

		// Enforce campaign caps
		if camp.MinBidCPM > 0 && grossBid < camp.MinBidCPM {
			grossBid = camp.MinBidCPM
		}
		if grossBid > camp.MaxBidCPM {
			grossBid = camp.MaxBidCPM
		}

		// 7. Reserve Floor Gate
		if grossBid < req.ReserveFloorCPM {
			noBidReason = fmt.Sprintf("Valuation $%.2f below reserve floor $%.2f", grossBid, req.ReserveFloorCPM)
			continue
		}

		// 8. Buyer Bid Shading (Shaving bid to maximize consumer surplus)
		shadedBid := grossBid * (1.0 - camp.ShadingDiscount)
		if shadedBid < req.ReserveFloorCPM {
			shadedBid = req.ReserveFloorCPM
		}

		// Compare expected surplus: (grossBid - shadedBid) * pCTR
		surplus := (grossBid - shadedBid) * pCTR
		if surplus >= highestExpectedValue {
			highestExpectedValue = surplus
			bestCamp = camp
			bestGrossBid = grossBid
			bestShadedBid = shadedBid
			bestRelevance = relevance
			bestCTR = pCTR
		}
	}

	evalDuration := time.Since(t0).Seconds() * 1000

	if bestCamp == nil {
		b.recordOutcomeMetrics(nil, false, 0, evalDuration)
		return &BidResponse{
			RequestID:        req.RequestID,
			Submitted:        false,
			NoBidReason:      noBidReason,
			EvaluationTimeMs: evalDuration,
			Timestamp:        time.Now().UTC(),
		}
	}

	// 9. Generate Cryptographic Ed25519 Bid Signature
	sigPayload := fmt.Sprintf("%s:%s:%s:%.4f:%d",
		req.RequestID, bestCamp.CampaignID, bestCamp.AdvertiserID, bestShadedBid, req.Timestamp.Unix())
	sigHash := sha256.Sum256([]byte(sigPayload))
	sigBytes := ed25519.Sign(bestCamp.SigningKey.PrivateKey, sigHash[:])
	sigHex := hex.EncodeToString(sigBytes)

	resp := &BidResponse{
		RequestID:        req.RequestID,
		CampaignID:       bestCamp.CampaignID,
		AdvertiserID:     bestCamp.AdvertiserID,
		AdvertiserName:   bestCamp.AdvertiserName,
		Submitted:        true,
		GrossBidCPM:      bestGrossBid,
		ShadedBidCPM:     bestShadedBid,
		FactsheetPayload: bestCamp.RawFactsheet,
		PredictedCTR:     bestCTR,
		RelevanceScore:   bestRelevance,
		SignatureHex:     sigHex,
		EvaluationTimeMs: evalDuration,
		Timestamp:        time.Now().UTC(),
	}

	b.recordOutcomeMetrics(resp, true, 0, evalDuration)
	return resp
}

func (b *BidderAgent) calculateRelevance(camp *CampaignConfig, req BidRequest) float64 {
	normQuery := strings.ToLower(req.ScrubbedIntent)
	score := 0.25 // Base affinity

	// Category exact/fuzzy match
	if strings.EqualFold(camp.Category, req.Category) {
		score += 0.35
	} else {
		for _, cat := range camp.TargetCategories {
			if strings.EqualFold(cat, req.Category) {
				score += 0.30
				break
			}
		}
	}

	// Keyword matches
	for _, kw := range camp.TargetKeywords {
		if strings.Contains(normQuery, strings.ToLower(kw)) {
			score += 0.15
			if score >= 1.0 {
				return 1.0
			}
		}
	}

	return math.Min(1.0, score)
}

func (b *BidderAgent) recordOutcomeMetrics(resp *BidResponse, submitted bool, clearingCPM float64, evalMs float64) {
	if !submitted {
		b.totalNoBids++
		return
	}

	b.totalBids++
	b.totalGrossSpend += (resp.GrossBidCPM / 1000.0)
	b.totalShadedSpend += (resp.ShadedBidCPM / 1000.0)
	b.totalShadingSaved += ((resp.GrossBidCPM - resp.ShadedBidCPM) / 1000.0)

	b.recentResponses = append(b.recentResponses, resp)
	if len(b.recentResponses) > 64 {
		b.recentResponses = b.recentResponses[1:]
	}

	if rep, ok := b.reports[resp.CampaignID]; ok {
		rep.RequestsSeen++
		rep.BidsSubmitted++
		rep.AverageBidCPM = (rep.AverageBidCPM*float64(rep.BidsSubmitted-1) + resp.ShadedBidCPM) / float64(rep.BidsSubmitted)
	}
}

// RecordAuctionOutcome registers whether a submitted bid won the auction.
func (b *BidderAgent) RecordAuctionOutcome(campaignID string, won bool, clearingCPM float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	camp, campExists := b.campaigns[campaignID]
	rep, repExists := b.reports[campaignID]

	if won {
		b.totalWins++
		actualCostUSD := clearingCPM / 1000.0

		if campExists {
			camp.TotalSpentUSD += actualCostUSD
		}
		if repExists {
			rep.AuctionsWon++
			rep.TotalSpentUSD += actualCostUSD
			rep.BudgetRemaining = math.Max(0, camp.DailyBudgetUSD-camp.TotalSpentUSD)
			if rep.BidsSubmitted > 0 {
				rep.WinRatePercent = (float64(rep.AuctionsWon) / float64(rep.BidsSubmitted)) * 100.0
			}
		}
	}
}

// GetSummary compiles the operational status across all campaigns.
func (b *BidderAgent) GetSummary() *BidderSummary {
	b.mu.RLock()
	defer b.mu.RUnlock()

	summary := &BidderSummary{
		GeneratedAt:            time.Now().UTC(),
		AdvertiserID:           b.advertiserID,
		TotalRequestsEvaluated: b.totalRequests,
		TotalBidsSubmitted:     b.totalBids,
		TotalNoBids:            b.totalNoBids,
		TotalAuctionsWon:       b.totalWins,
		TotalGrossSpendUSD:     b.totalGrossSpend,
		TotalShadedSpendUSD:    b.totalShadedSpend,
		TotalShadingSavingsUSD: b.totalShadingSaved,
		CampaignReports:        make(map[string]*CampaignReport),
		RecentResponses:        make([]*BidResponse, len(b.recentResponses)),
	}

	copy(summary.RecentResponses, b.recentResponses)

	if b.totalBids > 0 {
		summary.OverallWinRatePercent = (float64(b.totalWins) / float64(b.totalBids)) * 100.0
	}

	for id, rep := range b.reports {
		repCopy := *rep
		summary.CampaignReports[id] = &repCopy
	}

	return summary
}

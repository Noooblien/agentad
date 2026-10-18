package pricing

import (
	"time"
)

// DynamicFloorConfig defines parameters for publisher reserve price optimization.
type DynamicFloorConfig struct {
	BaseFloorCPM             float64            `json:"base_floor_cpm"`
	MinFloorCPM              float64            `json:"min_floor_cpm"`
	MaxFloorCPM              float64            `json:"max_floor_cpm"`
	TargetFillRate           float64            `json:"target_fill_rate"`          // Optimal target e.g. 0.70 (70%)
	FillRateSensitivity      float64            `json:"fill_rate_sensitivity"`     // Factor adjusting floor based on fill rate gap
	CategoryFloors           map[string]float64 `json:"category_floors"`           // Base overrides per query intent category
	HighCompetitionThreshold int                `json:"high_competition_threshold"`// Bidder count triggering competitive boost
	HighCompetitionBonus     float64            `json:"high_competition_bonus"`    // Percent increase per bidder above threshold
	AgentReputationDiscount  float64            `json:"agent_reputation_discount"` // Max discount for high-reputation agents (0-1)
}

// DefaultDynamicFloorConfig returns battle-tested production defaults.
func DefaultDynamicFloorConfig() DynamicFloorConfig {
	return DynamicFloorConfig{
		BaseFloorCPM:             1.00,
		MinFloorCPM:              0.25,
		MaxFloorCPM:              12.00,
		TargetFillRate:           0.70,
		FillRateSensitivity:      0.65,
		HighCompetitionThreshold: 4,
		HighCompetitionBonus:     0.08, // +8% per bidder above 4
		AgentReputationDiscount:  0.15, // Up to 15% discount for 0.95+ trust
		CategoryFloors: map[string]float64{
			"ai-infra":        3.50,
			"cloud-compute":   2.75,
			"developer-tools": 1.50,
			"security-audit":  3.00,
			"fintech-crypto":  4.00,
			"general-search":  0.80,
		},
	}
}

// BidShadingAlgorithm identifies the mathematical model used to shade bids.
type BidShadingAlgorithm string

const (
	AlgorithmVickreyEmulator  BidShadingAlgorithm = "vickrey_emulator"
	AlgorithmWinRateOptimizer BidShadingAlgorithm = "winrate_optimizer"
	AlgorithmFloorAwareHybrid BidShadingAlgorithm = "floor_aware_hybrid"
)

// BidShadingConfig configures buyer-side bid reduction to eliminate winner's curse.
type BidShadingConfig struct {
	Enabled                 bool                `json:"enabled"`
	Algorithm               BidShadingAlgorithm `json:"algorithm"`
	MinShadingRatio         float64             `json:"min_shading_ratio"`          // Minimum multiplier allowed (e.g. 0.50)
	MaxShadingRatio         float64             `json:"max_shading_ratio"`          // Maximum multiplier allowed (e.g. 0.98)
	ReserveSafetyMarginCPM  float64             `json:"reserve_safety_margin_cpm"`   // Safety margin added over estimated reserve floor
	TargetWinRate           float64             `json:"target_win_rate"`            // Desired win rate (e.g. 0.65)
	HistoricalWindowSize    int                 `json:"historical_window_size"`
}

// DefaultBidShadingConfig returns production defaults for buyer-side bid shading.
func DefaultBidShadingConfig() BidShadingConfig {
	return BidShadingConfig{
		Enabled:                true,
		Algorithm:              AlgorithmFloorAwareHybrid,
		MinShadingRatio:        0.55,
		MaxShadingRatio:        0.95,
		ReserveSafetyMarginCPM: 0.10,
		TargetWinRate:          0.65,
		HistoricalWindowSize:   100,
	}
}

// PublisherInventoryState tracks moving fill rates and historical yield metrics for a publisher.
type PublisherInventoryState struct {
	PublisherID         string    `json:"publisher_id"`
	Category            string    `json:"category"`
	TotalAuctions       int       `json:"total_auctions"`
	FilledAuctions      int       `json:"filled_auctions"`
	CurrentFillRate     float64   `json:"current_fill_rate"`
	AvgClearingPriceCPM float64   `json:"avg_clearing_price_cpm"`
	TotalYieldUSD       float64   `json:"total_yield_usd"`
	LastUpdated         time.Time `json:"last_updated"`
}

// FloorCalculationResult summarizes the components and final value of a dynamic reserve price.
type FloorCalculationResult struct {
	PublisherID            string  `json:"publisher_id"`
	Category               string  `json:"category"`
	BaseFloorCPM           float64 `json:"base_floor_cpm"`
	FillRateFactor         float64 `json:"fill_rate_factor"`
	CompetitionFactor      float64 `json:"competition_factor"`
	ReputationAdjustment   float64 `json:"reputation_adjustment"`
	ComputedFloorCPM       float64 `json:"computed_floor_cpm"`
	FinalFloorCPM          float64 `json:"final_floor_cpm"`
	IsClamped              bool    `json:"is_clamped"`
	ClampReason            string  `json:"clamp_reason,omitempty"`
}

// BidShadingResult captures the calculated shaded bid and estimated savings.
type BidShadingResult struct {
	OriginalBidCPM     float64             `json:"original_bid_cpm"`
	ShadedBidCPM       float64             `json:"shaded_bid_cpm"`
	ShadingRatio       float64             `json:"shading_ratio"`
	EstimatedFloorCPM  float64             `json:"estimated_floor_cpm"`
	SurplusSavedCPM    float64             `json:"surplus_saved_cpm"`
	AlgorithmUsed      BidShadingAlgorithm `json:"algorithm_used"`
	WinProbabilityEst  float64             `json:"win_probability_est"`
}

// CandidateBid represents an incoming campaign bid evaluated in the auction.
type CandidateBid struct {
	CampaignID     string  `json:"campaign_id"`
	AdvertiserName string  `json:"advertiser_name"`
	AgentID        string  `json:"agent_id"`
	RawBidCPM      float64 `json:"raw_bid_cpm"`
	ShadedBidCPM   float64 `json:"shaded_bid_cpm"`
	RelevanceScore float64 `json:"relevance_score"`
	TrustScore     float64 `json:"trust_score"`
	CompositeScore float64 `json:"composite_score"`
}

// RejectionReason specifies why a candidate bid was filtered out before auction clearing.
type RejectionReason string

const (
	RejectBelowDynamicFloor RejectionReason = "BELOW_DYNAMIC_FLOOR"
	RejectBelowStaticFloor  RejectionReason = "BELOW_STATIC_FLOOR"
	RejectLowRelevance      RejectionReason = "LOW_RELEVANCE"
	RejectLowTrust          RejectionReason = "LOW_TRUST_REPUTATION"
	RejectBudgetExhausted   RejectionReason = "BUDGET_EXHAUSTED"
)

// RejectedCandidate records details of an eliminated candidate.
type RejectedCandidate struct {
	CampaignID     string          `json:"campaign_id"`
	AdvertiserName string          `json:"advertiser_name"`
	BidCPM         float64         `json:"bid_cpm"`
	Reason         RejectionReason `json:"reason"`
	FloorPriceCPM  float64         `json:"floor_price_cpm"`
	Detail         string          `json:"detail"`
}

// AuctionEvaluationResult details the comparative auction outcome under dynamic vs static floor pricing.
type AuctionEvaluationResult struct {
	AuctionID               string                  `json:"auction_id"`
	Category                string                  `json:"category"`
	PublisherID             string                  `json:"publisher_id"`
	DynamicFloor            *FloorCalculationResult `json:"dynamic_floor"`
	StaticFloorCPM          float64                 `json:"static_floor_cpm"`
	TotalBidsReceived       int                     `json:"total_bids_received"`
	OriginalBids            []CandidateBid          `json:"original_bids"`
	EvaluatedBids           []CandidateBid          `json:"evaluated_bids"`
	QualifiedCandidates     []CandidateBid          `json:"qualified_candidates"`
	RejectedCandidates      []RejectedCandidate     `json:"rejected_candidates"`
	Winner                  *CandidateBid           `json:"winner,omitempty"`
	ClearingPriceCPM        float64                 `json:"clearing_price_cpm"`
	PublisherYieldUSD       float64                 `json:"publisher_yield_usd"`
	BuyerSavingsUSD         float64                 `json:"buyer_savings_usd"`
	StaticWouldHaveCleared  bool                    `json:"static_would_have_cleared"`
	StaticWinner            *CandidateBid           `json:"static_winner,omitempty"`
	StaticClearingPriceCPM  float64                 `json:"static_clearing_price_cpm"`
	StaticPublisherYieldUSD float64                 `json:"static_publisher_yield_usd"`
	Cleared                 bool                    `json:"cleared"`
	YieldUpliftUSD          float64                 `json:"yield_uplift_usd"`
	YieldUpliftPercent      float64                 `json:"yield_uplift_percent"`
	ExecutionDurationUs     int64                   `json:"execution_duration_us"`
	Timestamp               time.Time               `json:"timestamp"`
}

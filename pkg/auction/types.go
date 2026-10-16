package auction

import (
	"time"

	"github.com/Noooblien/agentad/pkg/spec"
)

// Campaign represents an advertiser's active bidding campaign in the RTB exchange.
type Campaign struct {
	CampaignID      string         `json:"campaign_id"`
	AdvertiserID    string         `json:"advertiser_id"`
	AdvertiserName  string         `json:"advertiser_name"`
	Category        string         `json:"category"`
	TargetKeywords  []string       `json:"target_keywords"`
	TargetTokens    []string       `json:"target_tokens"`
	MaxBidCPM       float64        `json:"max_bid_cpm"`        // Maximum bid in CPM USD
	TotalBudgetUSD  float64        `json:"total_budget_usd"`
	SpentUSD        float64        `json:"spent_usd"`
	TrustScore      float64        `json:"trust_score"`        // 0.0 - 1.0 based on historical compliance
	Factsheet       spec.Factsheet `json:"factsheet"`
	RawFactsheet    []byte         `json:"raw_factsheet"`
	Active          bool           `json:"active"`
	CreatedAt       time.Time      `json:"created_at"`
}

// BidCandidate records an individual candidate evaluated during an auction round.
type BidCandidate struct {
	CampaignID     string  `json:"campaign_id"`
	AdvertiserName string  `json:"advertiser_name"`
	BidCPM         float64 `json:"bid_cpm"`
	RelevanceScore float64 `json:"relevance_score"`
	TrustScore     float64 `json:"trust_score"`
	CompositeScore float64 `json:"composite_score"`
}

// AuctionResult contains the outcome of a sub-50ms RTB auction round.
type AuctionResult struct {
	AuctionID          string         `json:"auction_id"`
	IntentQuery        string         `json:"intent_query"`
	Category           string         `json:"category,omitempty"`
	WinningCampaign    *Campaign      `json:"winning_campaign,omitempty"`
	ClearingPriceCPM   float64        `json:"clearing_price_cpm"`
	DebitAmountUSD     float64        `json:"debit_amount_usd"`
	RelevanceScore     float64        `json:"relevance_score"`
	CompositeScore     float64        `json:"composite_score"`
	CandidatesCount    int            `json:"candidates_count"`
	CandidatesRanked   []BidCandidate `json:"candidates_ranked,omitempty"`
	ExecutionDuration  time.Duration  `json:"execution_duration"`
	ExecutionTimeMs    float64        `json:"execution_time_ms"`
	Timestamp          time.Time      `json:"timestamp"`
	AuctionCleared     bool           `json:"auction_cleared"`
}

// AuctionConfig specifies reserve prices and relevance filtering thresholds.
type AuctionConfig struct {
	ReservePriceCPM    float64 `json:"reserve_price_cpm"`    // Floor price e.g. 1.00 CPM
	MinRelevanceScore  float64 `json:"min_relevance_score"`  // Minimum semantic match e.g. 0.35
	MaxExecutionTimeMs float64 `json:"max_execution_time_ms"`// SLA e.g. 50ms
}

// DefaultAuctionConfig returns production defaults for the RTB engine.
func DefaultAuctionConfig() AuctionConfig {
	return AuctionConfig{
		ReservePriceCPM:    1.00,
		MinRelevanceScore:  0.35,
		MaxExecutionTimeMs: 50.0,
	}
}

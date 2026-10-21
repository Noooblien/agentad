package bidder

import (
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
	"github.com/Noooblien/agentad/pkg/placement"
)

// BiddingStrategy defines the economic algorithm used to calculate bid valuations.
type BiddingStrategy string

const (
	StrategyFixedCPM          BiddingStrategy = "FIXED_CPM"
	StrategyTargetCPA         BiddingStrategy = "TARGET_CPA"
	StrategyExpectedValue     BiddingStrategy = "EXPECTED_VALUE"
	StrategyCompetitiveShaded BiddingStrategy = "COMPETITIVE_SHADED"
)

// CampaignConfig encapsulates an advertiser's targeting criteria, budget, and bidding logic.
type CampaignConfig struct {
	CampaignID       string                  `json:"campaign_id"`
	AdvertiserID     string                  `json:"advertiser_id"`
	AdvertiserName   string                  `json:"advertiser_name"`
	Category         string                  `json:"category"`
	TargetCategories []string                `json:"target_categories"`
	TargetKeywords   []string                `json:"target_keywords"`
	NegativeKeywords []string                `json:"negative_keywords"`
	MaxBidCPM        float64                 `json:"max_bid_cpm"`
	MinBidCPM        float64                 `json:"min_bid_cpm"`
	TargetCPAUSD     float64                 `json:"target_cpa_usd"`
	DailyBudgetUSD   float64                 `json:"daily_budget_usd"`
	TotalSpentUSD    float64                 `json:"total_spent_usd"`
	TargetSlots      []placement.SlotArchetype `json:"target_slots"`
	RawFactsheet     map[string]interface{}  `json:"raw_factsheet"`
	Strategy         BiddingStrategy         `json:"strategy"`
	ShadingDiscount  float64                 `json:"shading_discount"` // e.g. 0.15 for 15% shading
	SigningKey       *attest.KeyPair         `json:"-"`
}

// BidRequest represents an inbound RTB auction opportunity broadcast by an AAP exchange.
type BidRequest struct {
	RequestID          string                  `json:"request_id"`
	AuctionID          string                  `json:"auction_id"`
	TraceID            string                  `json:"trace_id"`
	PublisherDomain    string                  `json:"publisher_domain"`
	PublisherID        string                  `json:"publisher_id"`
	ScrubbedIntent     string                  `json:"scrubbed_intent"`
	Category           string                  `json:"category"`
	ReserveFloorCPM    float64                 `json:"reserve_floor_cpm"`
	SlotType           placement.SlotArchetype `json:"slot_type"`
	AgentContextTokens int                     `json:"agent_context_tokens"`
	Turn               int                     `json:"turn"`
	Timestamp          time.Time               `json:"timestamp"`
}

// BidResponse represents the advertiser's cryptographic bid submission or no-bid notice.
type BidResponse struct {
	RequestID        string                 `json:"request_id"`
	CampaignID       string                 `json:"campaign_id,omitempty"`
	AdvertiserID     string                 `json:"advertiser_id,omitempty"`
	AdvertiserName   string                 `json:"advertiser_name,omitempty"`
	Submitted        bool                   `json:"submitted"`
	NoBidReason      string                 `json:"no_bid_reason,omitempty"`
	GrossBidCPM      float64                `json:"gross_bid_cpm"`
	ShadedBidCPM     float64                `json:"shaded_bid_cpm"`
	FactsheetPayload map[string]interface{} `json:"factsheet_payload,omitempty"`
	PredictedCTR     float64                `json:"predicted_ctr"`
	RelevanceScore   float64                `json:"relevance_score"`
	SignatureHex     string                 `json:"signature_hex,omitempty"`
	EvaluationTimeMs float64                `json:"evaluation_time_ms"`
	Timestamp        time.Time              `json:"timestamp"`
}

// CampaignReport tracks the operational performance of an individual campaign.
type CampaignReport struct {
	CampaignID      string          `json:"campaign_id"`
	AdvertiserName  string          `json:"advertiser_name"`
	Strategy        BiddingStrategy `json:"strategy"`
	RequestsSeen    int64           `json:"requests_seen"`
	BidsSubmitted   int64           `json:"bids_submitted"`
	AuctionsWon     int64           `json:"auctions_won"`
	WinRatePercent  float64         `json:"win_rate_percent"`
	TotalSpentUSD   float64         `json:"total_spent_usd"`
	ShadingSavedUSD float64         `json:"shading_saved_usd"`
	AverageBidCPM   float64         `json:"average_bid_cpm"`
	BudgetRemaining float64         `json:"budget_remaining"`
}

// BidderSummary provides an aggregate report across all managed campaigns.
type BidderSummary struct {
	GeneratedAt           time.Time                  `json:"generated_at"`
	AdvertiserID          string                     `json:"advertiser_id"`
	TotalRequestsEvaluated int64                      `json:"total_requests_evaluated"`
	TotalBidsSubmitted    int64                      `json:"total_bids_submitted"`
	TotalNoBids           int64                      `json:"total_no_bids"`
	TotalAuctionsWon      int64                      `json:"total_auctions_won"`
	OverallWinRatePercent float64                    `json:"overall_win_rate_percent"`
	TotalGrossSpendUSD    float64                    `json:"total_gross_spend_usd"`
	TotalShadedSpendUSD   float64                    `json:"total_shaded_spend_usd"`
	TotalShadingSavingsUSD float64                   `json:"total_shading_savings_usd"`
	AverageEvaluationMs   float64                    `json:"average_evaluation_ms"`
	CampaignReports       map[string]*CampaignReport `json:"campaign_reports"`
	RecentResponses       []*BidResponse             `json:"recent_responses"`
}

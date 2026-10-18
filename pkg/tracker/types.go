package tracker

import (
	"time"

	"github.com/Noooblien/agentad/pkg/escrow"
)

// AttributionModel defines the algorithm for distributing conversion credit.
type AttributionModel string

const (
	ModelFirstTouch    AttributionModel = "first_touch"    // 100% to discovery impression
	ModelLastTouch     AttributionModel = "last_touch"     // 100% to final recommendation
	ModelLinear        AttributionModel = "linear"         // Equal split across all touches
	ModelTimeDecay     AttributionModel = "time_decay"     // Exponential decay giving weight to touches near conversion
	ModelPositionBased AttributionModel = "position_based" // 40% first, 40% last, 20% middle (U-shaped)
)

// ActionType specifies the nature of the downstream conversion event.
type ActionType string

const (
	ActionToolCall       ActionType = "tool_call"        // Invocation of sponsored tool
	ActionCheckout       ActionType = "checkout"         // Commercial transaction/purchase
	ActionGoalCompletion ActionType = "goal_completion"  // Multi-step task objective fulfilled
	ActionSignup         ActionType = "signup"           // Agent account/API provisioning
)

// AttributionStatus denotes the outcome of an attribution evaluation.
type AttributionStatus string

const (
	StatusAttributed     AttributionStatus = "ATTRIBUTED"
	StatusDeduplicated   AttributionStatus = "DEDUPLICATED"
	StatusWindowExpired  AttributionStatus = "WINDOW_EXPIRED"
	StatusUnattributed   AttributionStatus = "UNATTRIBUTED"
)

// TouchPoint represents an observed ad impression in an agent's reasoning path.
type TouchPoint struct {
	ImpressionID string    `json:"impression_id"`
	SessionID    string    `json:"session_id"`
	CampaignID   string    `json:"campaign_id"`
	AdvertiserID string    `json:"advertiser_id"`
	PublisherID  string    `json:"publisher_id"`
	AgentID      string    `json:"agent_id"`
	CostUSD      float64   `json:"cost_usd"`
	ContextHash  string    `json:"context_hash"`
	Timestamp    time.Time `json:"timestamp"`
}

// Conversion represents a confirmed downstream action performed by an agent.
type Conversion struct {
	ConversionID    string     `json:"conversion_id"`
	SessionID       string     `json:"session_id"`
	AdvertiserID    string     `json:"advertiser_id"`
	CampaignID      string     `json:"campaign_id"`
	ActionType      ActionType `json:"action_type"`
	RevenueUSD      float64    `json:"revenue_usd"`      // Merchant order value
	PayoutBountyUSD float64    `json:"payout_bounty_usd"`// CPA/CPS reward to network
	Timestamp       time.Time  `json:"timestamp"`
	Nonce           string     `json:"nonce"`
	AttestationHash string     `json:"attestation_hash"` // SHA-256 binding details
	SignatureHex    string     `json:"signature_hex"`    // Ed25519 signature
	PublicKeyHex    string     `json:"public_key_hex"`
}

// AttributionResult encapsulates the final credit distribution and payout splits.
type AttributionResult struct {
	ConversionID    string             `json:"conversion_id"`
	SessionID       string             `json:"session_id"`
	Model           AttributionModel   `json:"model"`
	Status          AttributionStatus  `json:"status"`
	ActionType      ActionType         `json:"action_type"`
	RevenueUSD      float64            `json:"revenue_usd"`
	PayoutBountyUSD float64            `json:"payout_bounty_usd"`
	ConversionTime  time.Time          `json:"conversion_time"`
	TimeToConvert   time.Duration      `json:"time_to_convert"`
	TouchesCount    int                `json:"touches_count"`
	TouchPoints     []TouchPoint       `json:"touch_points"`
	Credits         map[string]float64 `json:"credits"`          // ImpressionID -> percentage (0.0 to 1.0)
	PublisherPayouts map[string]float64 `json:"publisher_payouts"`// PublisherID -> payout USD
	HostPayouts     map[string]float64 `json:"host_payouts"`     // AgentID -> payout USD
	ProtocolFeeUSD  float64            `json:"protocol_fee_usd"`
	AttestationValid bool              `json:"attestation_valid"`
	Notes           string             `json:"notes,omitempty"`
}

// TrackerConfig sets constraints and default parameters for conversion tracking.
type TrackerConfig struct {
	ConversionWindow  time.Duration       `json:"conversion_window"`   // Max age of touchpoint (e.g. 24h)
	DefaultModel      AttributionModel    `json:"default_model"`
	TimeDecayHalfLife time.Duration       `json:"time_decay_half_life"`// Exponential decay half life (e.g. 7d)
	PositionWeights   [3]float64          `json:"position_weights"`    // [First, Middle, Last] default [0.4, 0.2, 0.4]
	ReplayProtection  bool                `json:"replay_protection"`
	RevenueSplit      escrow.RevenueSplit `json:"revenue_split"`       // 70/20/10 split
}

// DefaultTrackerConfig returns production defaults for the attribution engine.
func DefaultTrackerConfig() TrackerConfig {
	return TrackerConfig{
		ConversionWindow:  24 * time.Hour,
		DefaultModel:      ModelPositionBased,
		TimeDecayHalfLife: 48 * time.Hour,
		PositionWeights:   [3]float64{0.40, 0.20, 0.40},
		ReplayProtection:  true,
		RevenueSplit:      escrow.DefaultRevenueSplit(),
	}
}

// TrackerStats reports aggregate metrics on tracked touches and conversions.
type TrackerStats struct {
	TotalTouches       int64   `json:"total_touches"`
	TotalConversions   int64   `json:"total_conversions"`
	TotalRevenueUSD    float64 `json:"total_revenue_usd"`
	TotalBountiesUSD   float64 `json:"total_bounties_usd"`
	TotalPubPayoutUSD  float64 `json:"total_pub_payout_usd"`
	TotalHostPayoutUSD float64 `json:"total_host_payout_usd"`
	TotalProtocolUSD   float64 `json:"total_protocol_usd"`
	AvgTimeToConvertMs float64 `json:"avg_time_to_convert_ms"`
}

// String returns a clean representation of the attribution model.
func (m AttributionModel) String() string {
	switch m {
	case ModelFirstTouch:
		return "First-Touch (100% Discovery)"
	case ModelLastTouch:
		return "Last-Touch (100% Closing)"
	case ModelLinear:
		return "Linear (Equal Multi-Touch)"
	case ModelTimeDecay:
		return "Time-Decay (Exponential Half-Life)"
	case ModelPositionBased:
		return "Position-Based (40/20/40 U-Shaped)"
	default:
		return string(m)
	}
}

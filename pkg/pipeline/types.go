package pipeline

import (
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
	"github.com/Noooblien/agentad/pkg/auction"
	"github.com/Noooblien/agentad/pkg/bandit"
	"github.com/Noooblien/agentad/pkg/discovery"
	"github.com/Noooblien/agentad/pkg/escrow"
	"github.com/Noooblien/agentad/pkg/fraud"
	"github.com/Noooblien/agentad/pkg/guardrail"
	"github.com/Noooblien/agentad/pkg/matcher"
	"github.com/Noooblien/agentad/pkg/pacing"
	"github.com/Noooblien/agentad/pkg/placement"
	"github.com/Noooblien/agentad/pkg/pricing"
	"github.com/Noooblien/agentad/pkg/privacy"
	"github.com/Noooblien/agentad/pkg/quality"
	"github.com/Noooblien/agentad/pkg/safety"
)

// PipelineStage identifies an execution checkpoint within the AAP pipeline.
type PipelineStage string

const (
	StageTrafficScrubbing PipelineStage = "TRAFFIC_SCRUBBING"
	StagePrivacyScrubbing PipelineStage = "PRIVACY_SCRUBBING"
	StageIntentMatching   PipelineStage = "INTENT_MATCHING"
	StageBrandSafety      PipelineStage = "BRAND_SAFETY"
	StageQualityScoring   PipelineStage = "QUALITY_SCORING"
	StagePricingAndFloor  PipelineStage = "PRICING_AND_FLOOR"
	StageAuctionClearing  PipelineStage = "AUCTION_CLEARING"
	StagePacingCheck      PipelineStage = "PACING_CHECK"
	StagePlacementAudit   PipelineStage = "PLACEMENT_AUDIT"
	StageGuardrailPurify  PipelineStage = "GUARDRAIL_PURIFY"
	StageAttestationSign  PipelineStage = "ATTESTATION_SIGN"
	StageEscrowSettlement PipelineStage = "ESCROW_SETTLEMENT"
)

// PipelineStatus describes the outcome of the end-to-end evaluation.
type PipelineStatus string

const (
	StatusCleared    PipelineStatus = "CLEARED"    // Fully cleared and ready for injection
	StatusSuppressed PipelineStatus = "SUPPRESSED" // Suppressed safely by policy or quiet-zone
	StatusRejected   PipelineStatus = "REJECTED"   // Rejected due to security, fraud, or safety
	StatusNoBid      PipelineStatus = "NO_BID"     // No eligible candidates met minimum auction criteria
)

// PipelineRequest contains the inputs required for an end-to-end AAP execution.
type PipelineRequest struct {
	TraceID        string                   `json:"trace_id"`
	AgentID        string                   `json:"agent_id"`
	RawQuery       string                   `json:"raw_query"`
	Category       string                   `json:"category"`
	LifecycleStage placement.LifecycleStage `json:"lifecycle_stage"`
	Turn           int                      `json:"turn"`
	TotalTokens    int                      `json:"total_tokens"`
	SlotType       placement.SlotArchetype  `json:"slot_type"`
	Session        *placement.SessionState  `json:"session,omitempty"`
}

// PipelineResponse records the end-to-end outcome, stages, and cryptographic receipts.
type PipelineResponse struct {
	TraceID             string                     `json:"trace_id"`
	Status              PipelineStatus             `json:"status"`
	HaltStage           PipelineStage              `json:"halt_stage,omitempty"`
	Reason              string                     `json:"reason"`
	ScrubbedQuery       string                     `json:"scrubbed_query"`
	WinningCampaignID   string                     `json:"winning_campaign_id,omitempty"`
	WinningAdvertiser   string                     `json:"winning_advertiser,omitempty"`
	ClearingPriceCPM    float64                    `json:"clearing_price_cpm"`
	DynamicFloorCPM     float64                    `json:"dynamic_floor_cpm"`
	DebitUSD            float64                    `json:"debit_usd"`
	QualityScore        float64                    `json:"quality_score"`
	QualityTier         quality.QualityTier        `json:"quality_tier"`
	SelectedSlot        placement.SlotArchetype    `json:"selected_slot"`
	FramedContext       string                     `json:"framed_context,omitempty"`
	ReceiptID           string                     `json:"receipt_id,omitempty"`
	SettlementID        string                     `json:"settlement_id,omitempty"`
	StageLatencies      map[PipelineStage]float64  `json:"stage_latencies_ms"`
	TotalDurationMs     float64                    `json:"total_duration_ms"`
	Receipt             *attest.AttestationReceipt `json:"receipt,omitempty"`
	Settlement          *escrow.SettlementRecord   `json:"settlement,omitempty"`
	Timestamp           time.Time                  `json:"timestamp"`
}

// PipelineComponents holds references to the modular AAP engines.
type PipelineComponents struct {
	FraudScanner      *fraud.Detector
	PrivacyEngine     *privacy.Engine
	VectorMatcher     *matcher.Index
	SafetyGovernor    *safety.Engine
	QualityAuditor    *quality.Engine
	PricingEngine     *pricing.FloorOptimizer
	AuctionEngine     *auction.Engine
	PacingController  *pacing.Controller
	PlacementEngine   *placement.Engine
	GuardrailPurifier *guardrail.Purifier
	AttestSigner      *attest.Signer
	PublisherKey      *attest.KeyPair
	EscrowLedger      *escrow.Ledger
	BanditEngine      *bandit.BanditEngine
	Registry          *discovery.Registry
}

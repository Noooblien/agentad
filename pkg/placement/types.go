package placement

import (
	"time"
)

// LifecycleStage classifies an agent's current cognitive state in a multi-turn workflow.
type LifecycleStage string

const (
	StageExploration Stage = "EXPLORATION" // Broad discovery, web search, tool research (High eligibility)
	StageReasoning   Stage = "REASONING"   // Deep iterative reasoning, coding, math, debugging (Cognitive quiet zone - Ad suppression)
	StageDecision    Stage = "DECISION"    // Comparative vendor evaluation, synthesis, selection (High eligibility for disclosure envelope)
	StageExecution   Stage = "EXECUTION"   // Payment, irreversible deployment, checkout, execution (Strict zero-ad safety lock)
)

// Stage is an alias for LifecycleStage for ergonomic usage.
type Stage = LifecycleStage

// SlotArchetype defines structural positions where sponsored data can be injected into agent workflows.
type SlotArchetype string

const (
	SlotNone              SlotArchetype = "NONE"
	SlotToolDefinition    SlotArchetype = "TOOL_DEFINITION"    // Appended to MCP tools/list as an optional tool schema
	SlotToolResponse      SlotArchetype = "TOOL_RESPONSE"      // Injected into MCP tools/call return payload metadata envelope
	SlotScratchpad        SlotArchetype = "SCRATCHPAD"         // Injected into persistent agent working notes or compaction buffer
	SlotSynthesisEnvelope SlotArchetype = "SYNTHESIS_ENVELOPE" // Disclosed in final user-facing response envelope
)

// PlacementDecision categorizes the outcome of the placement arbitration.
type PlacementDecision string

const (
	DecisionApproved           PlacementDecision = "APPROVED"
	DecisionSuppressedStage    PlacementDecision = "SUPPRESSED_STAGE"
	DecisionCappedFatigue      PlacementDecision = "CAPPED_FATIGUE"
	DecisionCooldownActive     PlacementDecision = "COOLDOWN_ACTIVE"
	DecisionExceededTokenRatio PlacementDecision = "EXCEEDED_TOKEN_RATIO"
	DecisionSubeconomicROI     PlacementDecision = "SUBECONOMIC_ROI"
)

// ImpressionRecord tracks historical ad exposures within an agent session.
type ImpressionRecord struct {
	Turn       int           `json:"turn"`
	AdID       string        `json:"ad_id"`
	Advertiser string        `json:"advertiser"`
	Slot       SlotArchetype `json:"slot"`
	TokenCount int           `json:"token_count"`
	BidCPM     float64       `json:"bid_cpm"`
	Timestamp  time.Time     `json:"timestamp"`
}

// SessionState captures the active agent conversation context.
type SessionState struct {
	SessionID          string             `json:"session_id"`
	CurrentTurn        int                `json:"current_turn"`
	TotalContextTokens int                `json:"total_context_tokens"`
	RemainingTurnsEst  int                `json:"remaining_turns_est"`
	CurrentStage       LifecycleStage     `json:"current_stage"`
	Impressions        []ImpressionRecord `json:"impressions"`
}

// PlacementCandidate represents an ad entrant bidding for insertion.
type PlacementCandidate struct {
	AdID            string                 `json:"ad_id"`
	Advertiser      string                 `json:"advertiser"`
	Category        string                 `json:"category"`
	BidCPM          float64                `json:"bid_cpm"`
	FactsheetTokens int                    `json:"factsheet_tokens"`
	SupportedSlots  []SlotArchetype        `json:"supported_slots"`
	Factsheet       map[string]interface{} `json:"factsheet"`
}

// PlacementConfig defines policy parameters and thresholds for the placement arbiter.
type PlacementConfig struct {
	MaxImpressionsPerSession int     `json:"max_impressions_per_session"` // Hard ceiling on total ads per session (default: 3)
	MinTurnsBetweenAds       int     `json:"min_turns_between_ads"`        // Turn cooldown spacing between any ad (default: 2)
	AdvertiserCooldownTurns  int     `json:"advertiser_cooldown_turns"`   // Specific advertiser turn spacing (default: 5)
	FatigueThreshold         float64 `json:"fatigue_threshold"`            // Max allowable fatigue index (0.0 - 1.0, default: 0.70)
	FatigueHalfLifeTurns     float64 `json:"fatigue_half_life_turns"`      // Exponential decay half-life in turns (default: 3.0)
	MaxContextTokenRatio     float64 `json:"max_context_token_ratio"`      // Max allowable sponsored token ratio (default: 0.05 = 5%)
	InputTokenCostPer1K      float64 `json:"input_token_cost_per_1k"`      // Host agent input token cost per 1K tokens in USD (default: $0.003)
	MinROIRatio              float64 `json:"min_roi_ratio"`                // Minimum required Revenue / Token Cost ratio (default: 1.25)
}

// PlacementEvaluation details the complete arbiter audit for a candidate ad.
type PlacementEvaluation struct {
	Candidate           PlacementCandidate `json:"candidate"`
	Decision            PlacementDecision  `json:"decision"`
	SelectedSlot        SlotArchetype      `json:"selected_slot"`
	Approved            bool               `json:"approved"`
	Reason              string             `json:"reason"`
	CurrentFatigue      float64            `json:"current_fatigue"`
	ContextTokenRatio   float64            `json:"context_token_ratio"`
	EstTokenCostUSD     float64            `json:"est_token_cost_usd"`
	EstRevenueUSD       float64            `json:"est_revenue_usd"`
	ROIRatio            float64            `json:"roi_ratio"`
	FormattedSlotPayload string            `json:"formatted_slot_payload"`
	EvaluatedAt         time.Time          `json:"evaluated_at"`
}

// PlacementSummary aggregates results across multi-scenario evaluations.
type PlacementSummary struct {
	SessionID       string                 `json:"session_id"`
	TotalEvaluated  int                    `json:"total_evaluated"`
	ApprovedCount   int                    `json:"approved_count"`
	SuppressedCount int                    `json:"suppressed_count"`
	AverageFatigue  float64                `json:"average_fatigue"`
	Evaluations     []*PlacementEvaluation `json:"evaluations"`
}

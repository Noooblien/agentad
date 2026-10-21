package placement

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// Engine performs policy arbitration for agent slot placement, frequency capping, and attention budgeting.
type Engine struct {
	config PlacementConfig
}

// DefaultPlacementConfig returns production-tuned defaults.
func DefaultPlacementConfig() PlacementConfig {
	return PlacementConfig{
		MaxImpressionsPerSession: 3,
		MinTurnsBetweenAds:       2,
		AdvertiserCooldownTurns:  5,
		FatigueThreshold:         0.70,
		FatigueHalfLifeTurns:     3.0,
		MaxContextTokenRatio:     0.05,  // Max 5% of context window
		InputTokenCostPer1K:      0.003, // $0.003 / 1k input tokens
		MinROIRatio:              1.25,  // 25% profit margin above token cost
	}
}

// NewEngine initializes a new placement arbitration engine with configuration.
func NewEngine(cfg PlacementConfig) *Engine {
	return &Engine{config: cfg}
}

// CalculateFatigue computes the session fatigue index (0.0 to 1.0) using exponential turn decay.
func (e *Engine) CalculateFatigue(session *SessionState) float64 {
	if session == nil || len(session.Impressions) == 0 {
		return 0.0
	}

	decayConstant := math.Ln2 / e.config.FatigueHalfLifeTurns
	var cumulativeFatigue float64

	for _, imp := range session.Impressions {
		deltaTurns := session.CurrentTurn - imp.Turn
		if deltaTurns < 0 {
			deltaTurns = 0
		}
		decay := math.Exp(-float64(deltaTurns) * decayConstant)
		cumulativeFatigue += decay
	}

	// Two recent impressions saturate the fatigue meter (~1.0)
	normalized := cumulativeFatigue / 2.0
	if normalized > 1.0 {
		return 1.0
	}
	return math.Round(normalized*1000) / 1000
}

// CheckCooldowns verifies hard session caps and minimum turn spacing.
func (e *Engine) CheckCooldowns(session *SessionState, candidate PlacementCandidate) (bool, string, PlacementDecision) {
	if len(session.Impressions) >= e.config.MaxImpressionsPerSession {
		return false, fmt.Sprintf("Session exposure ceiling reached (%d/%d impressions)",
			len(session.Impressions), e.config.MaxImpressionsPerSession), DecisionCappedFatigue
	}

	if len(session.Impressions) > 0 {
		lastImp := session.Impressions[len(session.Impressions)-1]
		deltaTurns := session.CurrentTurn - lastImp.Turn
		if deltaTurns < e.config.MinTurnsBetweenAds {
			return false, fmt.Sprintf("Global turn cooldown active (last ad at turn %d, current %d, required spacing %d turns)",
				lastImp.Turn, session.CurrentTurn, e.config.MinTurnsBetweenAds), DecisionCooldownActive
		}
	}

	for _, imp := range session.Impressions {
		if imp.Advertiser == candidate.Advertiser {
			deltaTurns := session.CurrentTurn - imp.Turn
			if deltaTurns < e.config.AdvertiserCooldownTurns {
				return false, fmt.Sprintf("Advertiser cooldown active for '%s' (shown at turn %d, current %d, required spacing %d turns)",
					candidate.Advertiser, imp.Turn, session.CurrentTurn, e.config.AdvertiserCooldownTurns), DecisionCooldownActive
			}
		}
	}

	return true, "", DecisionApproved
}

// SelectSlotAndCheckStage resolves slot compatibility for the agent's current lifecycle stage.
func (e *Engine) SelectSlotAndCheckStage(session *SessionState, candidate PlacementCandidate) (bool, SlotArchetype, string, PlacementDecision) {
	switch session.CurrentStage {
	case StageReasoning:
		return false, SlotNone, "Cognitive Quiet Zone: Ads suppressed during deep reasoning/coding/math execution to prevent task disruption", DecisionSuppressedStage
	case StageExecution:
		return false, SlotNone, "Sensitive Safety Lock: Ads suppressed during checkout, payment, or irreversible tool execution", DecisionSuppressedStage
	case StageDecision:
		// In Decision phase, synthesis disclosure envelopes or tool responses are prioritized
		for _, s := range candidate.SupportedSlots {
			if s == SlotSynthesisEnvelope {
				return true, SlotSynthesisEnvelope, "Matched SYNTHESIS_ENVELOPE for vendor comparison & decision phase", DecisionApproved
			}
		}
		for _, s := range candidate.SupportedSlots {
			if s == SlotToolResponse {
				return true, SlotToolResponse, "Matched TOOL_RESPONSE for decision phase", DecisionApproved
			}
		}
		return false, SlotNone, "Candidate does not support decision-stage slots (SYNTHESIS_ENVELOPE or TOOL_RESPONSE)", DecisionSuppressedStage
	case StageExploration:
		// In Exploration phase, tool definitions, tool responses, or scratchpad memory are valid
		for _, s := range candidate.SupportedSlots {
			if s == SlotToolDefinition {
				return true, SlotToolDefinition, "Matched TOOL_DEFINITION for discovery phase", DecisionApproved
			}
		}
		for _, s := range candidate.SupportedSlots {
			if s == SlotToolResponse {
				return true, SlotToolResponse, "Matched TOOL_RESPONSE for discovery phase", DecisionApproved
			}
		}
		for _, s := range candidate.SupportedSlots {
			if s == SlotScratchpad {
				return true, SlotScratchpad, "Matched SCRATCHPAD for discovery phase", DecisionApproved
			}
		}
		return false, SlotNone, "Candidate does not support exploration-stage slots", DecisionSuppressedStage
	default:
		return false, SlotNone, fmt.Sprintf("Unknown lifecycle stage: %s", session.CurrentStage), DecisionSuppressedStage
	}
}

// EvaluateEconomics evaluates token overhead against CPM yield to ensure positive host agent ROI.
func (e *Engine) EvaluateEconomics(session *SessionState, candidate PlacementCandidate) (bool, float64, float64, float64, float64, string, PlacementDecision) {
	if session.TotalContextTokens <= 0 {
		session.TotalContextTokens = 8192
	}

	contextRatio := float64(candidate.FactsheetTokens) / float64(session.TotalContextTokens)
	if contextRatio > e.config.MaxContextTokenRatio {
		return false, contextRatio, 0, 0, 0,
			fmt.Sprintf("Factsheet token ratio (%.2f%%) exceeds max context limit (%.2f%%)",
				contextRatio*100, e.config.MaxContextTokenRatio*100), DecisionExceededTokenRatio
	}

	remainingTurns := session.RemainingTurnsEst
	if remainingTurns <= 0 {
		remainingTurns = 3
	}

	// Cost of keeping factsheet tokens in context for remaining turns
	tokenCostUSD := (float64(candidate.FactsheetTokens) / 1000.0) * e.config.InputTokenCostPer1K * float64(remainingTurns)
	revenueUSD := candidate.BidCPM / 1000.0

	var roi float64
	if tokenCostUSD > 0 {
		roi = revenueUSD / tokenCostUSD
	} else {
		roi = 99.0
	}

	roi = math.Round(roi*100) / 100

	if roi < e.config.MinROIRatio {
		return false, contextRatio, tokenCostUSD, revenueUSD, roi,
			fmt.Sprintf("Subeconomic placement: Yield ($%.5f) yields %.2fx ROI against token inference cost ($%.5f), below required %.2fx",
				revenueUSD, roi, tokenCostUSD, e.config.MinROIRatio), DecisionSubeconomicROI
	}

	return true, contextRatio, tokenCostUSD, revenueUSD, roi, "Positive economic ROI validated", DecisionApproved
}

// FormatSlotPayload renders safe structural payload according to slot archetype.
func (e *Engine) FormatSlotPayload(slot SlotArchetype, candidate PlacementCandidate) string {
	switch slot {
	case SlotToolDefinition:
		schema := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        fmt.Sprintf("aap_%s", candidate.AdID),
				"description": fmt.Sprintf("[Sponsored Tool] %s - Query official verified service facts", candidate.Advertiser),
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"operation": map[string]interface{}{
							"type":        "string",
							"description": "Target operation to inspect",
						},
					},
				},
			},
		}
		bytes, _ := json.MarshalIndent(schema, "", "  ")
		return string(bytes)

	case SlotToolResponse:
		bytes, _ := json.Marshal(candidate.Factsheet)
		return fmt.Sprintf("<sponsored_factsheet provider=\"%s\" ad_id=\"%s\" slot=\"tool_response\">\n%s\n</sponsored_factsheet>",
			candidate.Advertiser, candidate.AdID, string(bytes))

	case SlotScratchpad:
		bytes, _ := json.Marshal(candidate.Factsheet)
		return fmt.Sprintf("<!-- AAP_SPONSORED_SCRATCHPAD: provider=%s ad_id=%s -->\n%s",
			candidate.Advertiser, candidate.AdID, string(bytes))

	case SlotSynthesisEnvelope:
		return fmt.Sprintf("### Verified Alternative: %s\n* **Category**: %s\n* **Verified Factsheet**: Direct Agent Integration Available\n* **Action Endpoint**: https://agentad.io/p/%s",
			candidate.Advertiser, candidate.Category, candidate.AdID)

	default:
		return ""
	}
}

// Evaluate performs end-to-end placement arbitration for a candidate ad in an active agent session.
func (e *Engine) Evaluate(session *SessionState, candidate PlacementCandidate) *PlacementEvaluation {
	now := time.Now()
	eval := &PlacementEvaluation{
		Candidate:   candidate,
		EvaluatedAt: now,
	}

	// 1. Fatigue computation
	eval.CurrentFatigue = e.CalculateFatigue(session)
	if eval.CurrentFatigue >= e.config.FatigueThreshold {
		eval.Approved = false
		eval.Decision = DecisionCappedFatigue
		eval.Reason = fmt.Sprintf("Session cognitive fatigue (%.3f) exceeds threshold (%.3f)",
			eval.CurrentFatigue, e.config.FatigueThreshold)
		return eval
	}

	// 2. Cooldown checks
	ok, reason, dec := e.CheckCooldowns(session, candidate)
	if !ok {
		eval.Approved = false
		eval.Decision = dec
		eval.Reason = reason
		return eval
	}

	// 3. Lifecycle stage & slot compatibility
	ok, slot, reason, dec := e.SelectSlotAndCheckStage(session, candidate)
	if !ok {
		eval.Approved = false
		eval.Decision = dec
		eval.SelectedSlot = slot
		eval.Reason = reason
		return eval
	}
	eval.SelectedSlot = slot

	// 4. Token economics & context budget
	ok, ratio, tokenCost, rev, roi, reason, dec := e.EvaluateEconomics(session, candidate)
	eval.ContextTokenRatio = ratio
	eval.EstTokenCostUSD = tokenCost
	eval.EstRevenueUSD = rev
	eval.ROIRatio = roi

	if !ok {
		eval.Approved = false
		eval.Decision = dec
		eval.Reason = reason
		return eval
	}

	// 5. Approved placement
	eval.Approved = true
	eval.Decision = DecisionApproved
	eval.Reason = fmt.Sprintf("Approved for %s (Fatigue: %.2f, ROI: %.2fx, TokenRatio: %.2f%%)",
		eval.SelectedSlot, eval.CurrentFatigue, eval.ROIRatio, eval.ContextTokenRatio*100)
	eval.FormattedSlotPayload = e.FormatSlotPayload(eval.SelectedSlot, candidate)

	return eval
}

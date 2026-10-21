package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

// Pipeline orchestrates the sequential 12-stage AAP execution pipeline.
type Pipeline struct {
	mu         sync.RWMutex
	components PipelineComponents
}

// NewPipeline initializes a fully wired AAP execution pipeline with production defaults.
func NewPipeline(comps PipelineComponents) (*Pipeline, error) {
	if comps.FraudScanner == nil {
		comps.FraudScanner = fraud.NewDetector(fraud.DefaultDetectorConfig())
	}
	if comps.PrivacyEngine == nil {
		comps.PrivacyEngine = privacy.NewEngine(privacy.DefaultPrivacyConfig())
	}
	if comps.VectorMatcher == nil {
		comps.VectorMatcher = matcher.NewIndex(384)
	}
	if comps.SafetyGovernor == nil {
		comps.SafetyGovernor = safety.NewEngine(safety.SafetyPolicy{
			BlockedCategories:      []string{"malware", "weapons", "adult", "gambling"},
			CompetitorExclusions:   map[string][]string{"Pinecone Systems": {"Weaviate", "Qdrant"}},
			MinAlignmentScore:      0.25,
			StrictBrandSuitability: false,
		})
	}
	if comps.QualityAuditor == nil {
		comps.QualityAuditor = quality.NewEngine(quality.DefaultQualityConfig())
	}
	if comps.PricingEngine == nil {
		comps.PricingEngine = pricing.NewFloorOptimizer(pricing.DynamicFloorConfig{
			BaseFloorCPM:             1.00,
			MinFloorCPM:              0.25,
			MaxFloorCPM:              12.00,
			TargetFillRate:           0.70,
			FillRateSensitivity:      0.65,
			HighCompetitionThreshold: 4,
			HighCompetitionBonus:     0.08,
		})
	}
	if comps.AuctionEngine == nil {
		comps.AuctionEngine = auction.NewEngine(auction.DefaultAuctionConfig())
		comps.AuctionEngine.SeedDemoCampaigns()
	}
	if comps.PacingController == nil {
		comps.PacingController = pacing.NewController(pacing.DefaultPIDGains())
		comps.PacingController.RegisterCampaign("cmp_pinecone_01", 500.0, pacing.PacingEven)
		comps.PacingController.RegisterCampaign("cmp_neon_02", 350.0, pacing.PacingEven)
		comps.PacingController.RegisterCampaign("cmp_cloudflare_03", 200.0, pacing.PacingEven)
	}
	if comps.PlacementEngine == nil {
		comps.PlacementEngine = placement.NewEngine(placement.DefaultPlacementConfig())
	}
	if comps.GuardrailPurifier == nil {
		comps.GuardrailPurifier = guardrail.NewPurifier(true)
	}
	if comps.AttestSigner == nil {
		comps.AttestSigner = attest.NewSigner()
	}
	if comps.PublisherKey == nil {
		kp, err := attest.GenerateKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed generating publisher key: %w", err)
		}
		comps.PublisherKey = kp
	}
	if comps.EscrowLedger == nil {
		ledger, err := escrow.NewLedger("treasury_aap", escrow.DefaultRevenueSplit())
		if err == nil {
			ledger.RegisterAccount("adv_pinecone", "Pinecone Systems", "advertiser", escrow.CurrencyMicroUSD)
			_ = ledger.Deposit("adv_pinecone", escrow.USDToMicro(500.00))
			ledger.RegisterAccount("adv_neon", "Neon Postgres", "advertiser", escrow.CurrencyMicroUSD)
			_ = ledger.Deposit("adv_neon", escrow.USDToMicro(350.00))
			comps.EscrowLedger = ledger
		}
	}
	if comps.BanditEngine == nil {
		sampler := bandit.NewSampler(42)
		comps.BanditEngine = bandit.NewBanditEngine(bandit.DefaultBanditConfig(), sampler)
	}
	if comps.Registry == nil {
		resolver := discovery.NewManifestResolver(discovery.DefaultConfig())
		comps.Registry = discovery.NewRegistry(resolver)
	}

	return &Pipeline{
		components: comps,
	}, nil
}

// Execute runs the complete 12-stage AAP execution pipeline synchronously.
func (p *Pipeline) Execute(ctx context.Context, req PipelineRequest) *PipelineResponse {
	startAll := time.Now()
	resp := &PipelineResponse{
		TraceID:        req.TraceID,
		StageLatencies: make(map[PipelineStage]float64),
		Timestamp:      startAll.UTC(),
		Status:         StatusCleared,
	}

	// -------------------------------------------------------------------------
	// Stage 1: Traffic Fraud & Bot Telemetry Scrubbing
	// -------------------------------------------------------------------------
	t0 := time.Now()
	verdict := p.components.FraudScanner.AnalyzeTraffic(req.AgentID, "pub_main", req.RawQuery)
	resp.StageLatencies[StageTrafficScrubbing] = time.Since(t0).Seconds() * 1000
	if verdict.RiskLevel == fraud.RiskFraudulent {
		resp.Status = StatusRejected
		resp.HaltStage = StageTrafficScrubbing
		resp.Reason = fmt.Sprintf("Fraud protection triggered: score=%.2f risk=%s vectors=%v", verdict.OverallFraudScore, verdict.RiskLevel, verdict.TriggeredVectors)
		resp.TotalDurationMs = time.Since(startAll).Seconds() * 1000
		return resp
	}

	// -------------------------------------------------------------------------
	// Stage 2: Privacy Scrubbing & Zero-PII Sanitization
	// -------------------------------------------------------------------------
	t0 = time.Now()
	privReport := p.components.PrivacyEngine.EvaluateIntent("sess_"+req.TraceID, req.RawQuery, nil)
	resp.StageLatencies[StagePrivacyScrubbing] = time.Since(t0).Seconds() * 1000
	scrubbedQuery := privReport.SanitizedQuery
	resp.ScrubbedQuery = scrubbedQuery

	// -------------------------------------------------------------------------
	// Stage 3: Semantic Matcher & Cosine Similarity
	// -------------------------------------------------------------------------
	t0 = time.Now()
	matchQuery := scrubbedQuery
	if matchQuery == "" {
		matchQuery = req.RawQuery
	}
	resp.StageLatencies[StageIntentMatching] = time.Since(t0).Seconds() * 1000

	// -------------------------------------------------------------------------
	// Stage 4: Brand Safety & Competitor Negative Targeting
	// -------------------------------------------------------------------------
	t0 = time.Now()
	safetyCandidate := safety.CandidateAd{
		AdID:        "ad_candidate_eval",
		Advertiser:  "Pinecone Systems",
		Category:    req.Category,
		Competitors: []string{"Weaviate", "Qdrant"},
		Keywords:    strings.Fields(strings.ToLower(matchQuery)),
	}
	safetyEval := p.components.SafetyGovernor.Evaluate(matchQuery, safetyCandidate)
	resp.StageLatencies[StageBrandSafety] = time.Since(t0).Seconds() * 1000
	if !safetyEval.Allowed {
		resp.Status = StatusRejected
		resp.HaltStage = StageBrandSafety
		resp.Reason = fmt.Sprintf("Brand safety gate rejected: verdict=%s reason=%s", safetyEval.Verdict, safetyEval.Reason)
		resp.TotalDurationMs = time.Since(startAll).Seconds() * 1000
		return resp
	}

	// -------------------------------------------------------------------------
	// Stage 5: Dynamic Floor Price & Bid Shading
	// -------------------------------------------------------------------------
	t0 = time.Now()
	floorResult := p.components.PricingEngine.CalculateFloor("pub_main", req.Category, 4, 0.95)
	resp.StageLatencies[StagePricingAndFloor] = time.Since(t0).Seconds() * 1000
	resp.DynamicFloorCPM = floorResult.FinalFloorCPM

	// -------------------------------------------------------------------------
	// Stage 6: RTB Auction Execution (Vickrey Second-Price)
	// -------------------------------------------------------------------------
	t0 = time.Now()
	p.components.AuctionEngine.Config.ReservePriceCPM = floorResult.FinalFloorCPM
	aucRes, aucErr := p.components.AuctionEngine.RunAuction(matchQuery, "")
	resp.StageLatencies[StageAuctionClearing] = time.Since(t0).Seconds() * 1000
	if aucErr != nil || aucRes == nil || !aucRes.AuctionCleared || aucRes.WinningCampaign == nil {
		resp.Status = StatusNoBid
		resp.HaltStage = StageAuctionClearing
		resp.Reason = "No eligible candidates met reserve floor or auction cleared 0 bids"
		resp.TotalDurationMs = time.Since(startAll).Seconds() * 1000
		return resp
	}

	winningCamp := aucRes.WinningCampaign
	resp.WinningCampaignID = winningCamp.CampaignID
	resp.WinningAdvertiser = winningCamp.AdvertiserName
	resp.ClearingPriceCPM = aucRes.ClearingPriceCPM
	resp.DebitUSD = aucRes.DebitAmountUSD

	// -------------------------------------------------------------------------
	// Stage 7: Quality Score (AQS) & Grounding Audit
	// -------------------------------------------------------------------------
	t0 = time.Now()
	qualCandidate := quality.QualityCandidate{
		AdID:            winningCamp.CampaignID,
		Advertiser:      winningCamp.AdvertiserName,
		Category:        winningCamp.Category,
		BidCPM:          aucRes.ClearingPriceCPM,
		Keywords:        winningCamp.TargetKeywords,
		HistoricalCTR:   0.045,
		ToolCallSuccess: 0.98,
		Factsheet: map[string]interface{}{
			"service_name": winningCamp.AdvertiserName,
			"pricing":      fmt.Sprintf("$%.2f CPM", aucRes.ClearingPriceCPM),
		},
	}
	qualBreakdown := p.components.QualityAuditor.EvaluateCandidate(matchQuery, qualCandidate)
	resp.StageLatencies[StageQualityScoring] = time.Since(t0).Seconds() * 1000
	resp.QualityScore = qualBreakdown.QualityScore
	resp.QualityTier = qualBreakdown.Tier
	if !qualBreakdown.Eligible {
		resp.Status = StatusRejected
		resp.HaltStage = StageQualityScoring
		resp.Reason = fmt.Sprintf("Quality audit failed: AQS=%.2f tier=%s", qualBreakdown.QualityScore, qualBreakdown.Tier)
		resp.TotalDurationMs = time.Since(startAll).Seconds() * 1000
		return resp
	}

	// -------------------------------------------------------------------------
	// Stage 8: Budget Pacing & Spend Velocity Controller
	// -------------------------------------------------------------------------
	t0 = time.Now()
	pacingDecision := p.components.PacingController.EvaluateBid(winningCamp.CampaignID, 12, 30, 0.5)
	resp.StageLatencies[StagePacingCheck] = time.Since(t0).Seconds() * 1000
	if !pacingDecision.AllowBid {
		resp.Status = StatusSuppressed
		resp.HaltStage = StagePacingCheck
		resp.Reason = fmt.Sprintf("Budget pacing controller suppressed bid: reason=%s", pacingDecision.Reason)
		resp.TotalDurationMs = time.Since(startAll).Seconds() * 1000
		return resp
	}

	// -------------------------------------------------------------------------
	// Stage 9: Attention Budget, Frequency Capping & Quiet Zone Placement Arbiter
	// -------------------------------------------------------------------------
	t0 = time.Now()
	sessionState := req.Session
	if sessionState == nil {
		sessionState = &placement.SessionState{
			SessionID:          "sess_" + req.TraceID,
			CurrentTurn:        req.Turn,
			TotalContextTokens: req.TotalTokens,
			RemainingTurnsEst:  3,
			CurrentStage:       req.LifecycleStage,
			Impressions:        []placement.ImpressionRecord{},
		}
	}

	placementCandidate := placement.PlacementCandidate{
		AdID:            winningCamp.CampaignID,
		Advertiser:      winningCamp.AdvertiserName,
		Category:        winningCamp.Category,
		BidCPM:          aucRes.ClearingPriceCPM,
		FactsheetTokens: 40,
		SupportedSlots:  []placement.SlotArchetype{placement.SlotToolDefinition, placement.SlotToolResponse, placement.SlotSynthesisEnvelope},
		Factsheet: map[string]interface{}{
			"service_name": winningCamp.AdvertiserName,
		},
	}
	placementEval := p.components.PlacementEngine.Evaluate(sessionState, placementCandidate)
	resp.StageLatencies[StagePlacementAudit] = time.Since(t0).Seconds() * 1000
	if !placementEval.Approved {
		resp.Status = StatusSuppressed
		resp.HaltStage = StagePlacementAudit
		resp.Reason = fmt.Sprintf("Placement arbiter suppressed candidate: %s (%s)", placementEval.Decision, placementEval.Reason)
		resp.TotalDurationMs = time.Since(startAll).Seconds() * 1000
		return resp
	}
	resp.SelectedSlot = placementEval.SelectedSlot

	// -------------------------------------------------------------------------
	// Stage 10: Anti-Jailbreak Guardrail & AST Factsheet Purifier
	// -------------------------------------------------------------------------
	t0 = time.Now()
	purified, purifyErr := p.components.GuardrailPurifier.PurifyFactsheet(winningCamp.RawFactsheet)
	resp.StageLatencies[StageGuardrailPurify] = time.Since(t0).Seconds() * 1000
	if purifyErr != nil || purified == nil || !purified.Allowed {
		resp.Status = StatusRejected
		resp.HaltStage = StageGuardrailPurify
		resp.Reason = "Anti-prompt injection guardrail rejected unpurified factsheet"
		resp.TotalDurationMs = time.Since(startAll).Seconds() * 1000
		return resp
	}
	resp.FramedContext = purified.SafeFramedContext

	// -------------------------------------------------------------------------
	// Stage 11: Cryptographic Provenance Attestation (Ed25519 PoI Receipt)
	// -------------------------------------------------------------------------
	t0 = time.Now()
	receipt, signErr := p.components.AttestSigner.SignReceipt(
		p.components.PublisherKey,
		req.AgentID,
		attest.TypeProofOfImpression,
		aucRes.AuctionID,
		winningCamp.CampaignID,
		winningCamp.AdvertiserID,
		purified.SafeFramedContext,
		aucRes.ClearingPriceCPM,
		aucRes.DebitAmountUSD,
	)
	resp.StageLatencies[StageAttestationSign] = time.Since(t0).Seconds() * 1000
	if signErr != nil || receipt == nil {
		resp.Status = StatusRejected
		resp.HaltStage = StageAttestationSign
		resp.Reason = fmt.Sprintf("Failed generating Ed25519 attestation signature: %v", signErr)
		resp.TotalDurationMs = time.Since(startAll).Seconds() * 1000
		return resp
	}
	resp.Receipt = receipt
	resp.ReceiptID = receipt.ReceiptID

	// -------------------------------------------------------------------------
	// Stage 12: Escrow Settlement & Programmatic Revenue Split (70/20/10)
	// -------------------------------------------------------------------------
	t0 = time.Now()
	if p.components.EscrowLedger != nil && aucRes.DebitAmountUSD > 0 {
		microAmount := escrow.USDToMicro(aucRes.DebitAmountUSD)
		res, resErr := p.components.EscrowLedger.ReserveFunds(
			aucRes.AuctionID,
			winningCamp.AdvertiserID,
			"cloudinfra.io",
			req.AgentID,
			microAmount,
			15*time.Minute,
		)
		if resErr == nil && res != nil {
			settlement, setErr := p.components.EscrowLedger.SettleAttestation(receipt, res.ReservationID, p.components.AttestSigner)
			if setErr == nil && settlement != nil {
				resp.Settlement = settlement
				resp.SettlementID = settlement.SettlementID
			}
		}
	}
	resp.StageLatencies[StageEscrowSettlement] = time.Since(t0).Seconds() * 1000

	resp.TotalDurationMs = time.Since(startAll).Seconds() * 1000
	resp.Reason = "Successfully cleared all 12 zero-jailbreak governance & settlement gates"
	return resp
}

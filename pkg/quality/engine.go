package quality

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Engine performs factual grounding audits and computes Agent Ad Quality Scores.
type Engine struct {
	config QualityConfig
}

// NewEngine constructs a quality evaluation engine with specified or default config.
func NewEngine(cfg QualityConfig) *Engine {
	if cfg.WeightGrounding <= 0 && cfg.WeightRelevance <= 0 && cfg.WeightAdoption <= 0 {
		cfg = DefaultQualityConfig()
	}
	return &Engine{config: cfg}
}

// EvaluateCandidate inspects an ad candidate against an agent query context.
func (e *Engine) EvaluateCandidate(query string, candidate QualityCandidate) *QualityBreakdown {
	start := time.Now()

	claimsAudited := make([]ClaimAudit, 0)
	flaggedIssues := make([]string, 0)

	// 1. Audit Factsheet Grounding & Claims
	groundingScore, riskPenalty := e.auditFactsheet(candidate.Factsheet, &claimsAudited, &flaggedIssues)

	// 2. Compute Intent Semantic Relevance
	relevanceScore := e.computeRelevance(query, candidate)

	// 3. Compute Historical Agent Adoption Propensity
	adoptionScore := e.computeAdoptionScore(candidate)

	// 4. Synthesize Composite Raw Score
	rawComposite := (e.config.WeightGrounding * groundingScore) +
		(e.config.WeightRelevance * relevanceScore) +
		(e.config.WeightAdoption * adoptionScore) -
		riskPenalty

	rawClamped := math.Max(0.0, math.Min(1.0, rawComposite))

	// Standard 1.0 to 10.0 AQS scale
	aqs := 1.0 + (9.0 * rawClamped)
	aqs = math.Round(aqs*10) / 10 // round to 1 decimal place

	// 5. Determine Quality Tier, Auction Discount/Penalty & Eligibility
	tier, clearingMultiplier, eligible := e.classifyTier(aqs)

	// Calculate Effective AdRank = BidCPM * (AQS / 10.0)
	var effectiveAdRank float64
	var effectiveCostCPM float64
	if eligible {
		effectiveAdRank = math.Round((candidate.BidCPM*(aqs/10.0))*100) / 100
		effectiveCostCPM = math.Round((candidate.BidCPM*clearingMultiplier)*100) / 100
	} else {
		effectiveAdRank = 0.0
		effectiveCostCPM = 0.0
		if len(flaggedIssues) == 0 {
			flaggedIssues = append(flaggedIssues, fmt.Sprintf("Quality Score %.1f is below minimum threshold %.1f", aqs, e.config.MinEligibleAQS))
		}
	}

	return &QualityBreakdown{
		AdID:               candidate.AdID,
		Advertiser:         candidate.Advertiser,
		QueryIntent:        query,
		GroundingScore:     math.Round(groundingScore*100) / 100,
		RelevanceScore:     math.Round(relevanceScore*100) / 100,
		AdoptionScore:      math.Round(adoptionScore*100) / 100,
		RiskPenalty:        math.Round(riskPenalty*100) / 100,
		RawScore:           math.Round(rawClamped*100) / 100,
		QualityScore:       aqs,
		Tier:               tier,
		RawBidCPM:          candidate.BidCPM,
		EffectiveAdRank:    effectiveAdRank,
		ClearingMultiplier: clearingMultiplier,
		EffectiveCostCPM:   effectiveCostCPM,
		Eligible:           eligible,
		ClaimsAudited:      claimsAudited,
		FlaggedIssues:      flaggedIssues,
		Duration:           time.Since(start),
	}
}

// EvaluateBatch audits multiple ad candidates and ranks them by Effective AdRank.
func (e *Engine) EvaluateBatch(query string, candidates []QualityCandidate) *QualitySummary {
	start := time.Now()

	evals := make([]*QualityBreakdown, len(candidates))
	tierCounts := map[QualityTier]int{
		TierExcellent:    0,
		TierGood:         0,
		TierNeutral:      0,
		TierSubstandard:  0,
		TierDisqualified: 0,
	}

	eligibleCount := 0
	disqualifiedCount := 0
	totalAQS := 0.0

	for i, c := range candidates {
		eval := e.EvaluateCandidate(query, c)
		evals[i] = eval
		tierCounts[eval.Tier]++
		totalAQS += eval.QualityScore

		if eval.Eligible {
			eligibleCount++
		} else {
			disqualifiedCount++
		}
	}

	// Sort candidates by Effective AdRank descending (highest ranking ad wins)
	sort.Slice(evals, func(i, j int) bool {
		return evals[i].EffectiveAdRank > evals[j].EffectiveAdRank
	})

	avgAQS := 0.0
	if len(candidates) > 0 {
		avgAQS = math.Round((totalAQS/float64(len(candidates)))*10) / 10
	}

	return &QualitySummary{
		TotalEvaluated:    len(candidates),
		EligibleCount:     eligibleCount,
		DisqualifiedCount: disqualifiedCount,
		AverageAQS:        avgAQS,
		TierBreakdown:     tierCounts,
		Evaluations:       evals,
		Duration:          time.Since(start),
	}
}

// auditFactsheet verifies completeness, endpoints, authentication, and hype claim flags.
func (e *Engine) auditFactsheet(factsheet map[string]interface{}, claims *[]ClaimAudit, flags *[]string) (float64, float64) {
	if factsheet == nil || len(factsheet) == 0 {
		*flags = append(*flags, "Empty or missing factsheet payload")
		return 0.0, 0.40
	}

	scoreComponents := 0.0
	maxComponents := 4.0 // mandatory keys, valid endpoint, structured auth/pricing, claim verifiability
	penalty := 0.0

	// 1. Mandatory Schema Key Verification
	missingKeys := 0
	for _, key := range e.config.MandatorySchemaKeys {
		if val, exists := factsheet[key]; !exists || val == nil || fmt.Sprintf("%v", val) == "" {
			missingKeys++
		}
	}
	if missingKeys == 0 {
		scoreComponents += 1.0
		*claims = append(*claims, ClaimAudit{
			Claim:       "All mandatory schema attributes present",
			Field:       "schema_completeness",
			Flag:        FlagValidClaim,
			IsGrounded:  true,
			Explanation: "Factsheet satisfies complete structured metadata standard",
		})
	} else {
		scoreComponents += math.Max(0, 1.0-(float64(missingKeys)*0.25))
		*flags = append(*flags, fmt.Sprintf("Missing %d required factsheet schema keys", missingKeys))
	}

	// 2. API Endpoint Verification
	if endpointVal, exists := factsheet["api_endpoint"]; exists {
		endpointStr := fmt.Sprintf("%v", endpointVal)
		u, err := url.Parse(endpointStr)
		if err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
			scoreComponents += 1.0
			*claims = append(*claims, ClaimAudit{
				Claim:       fmt.Sprintf("Verifiable REST/MCP Endpoint (%s)", u.Host),
				Field:       "api_endpoint",
				Flag:        FlagValidClaim,
				IsGrounded:  true,
				Explanation: "Host routing syntax is valid and directly resolvable",
			})
		} else {
			penalty += 0.15
			*flags = append(*flags, "Invalid or unresolvable api_endpoint URI")
			*claims = append(*claims, ClaimAudit{
				Claim:       endpointStr,
				Field:       "api_endpoint",
				Flag:        FlagMissingEndpoint,
				IsGrounded:  false,
				Explanation: "Endpoint must be a valid http/https URI",
			})
		}
	} else {
		penalty += 0.15
		*flags = append(*flags, "Missing api_endpoint in factsheet")
	}

	// 3. Authentication & Pricing Completeness
	hasValidAuth := false
	if authVal, exists := factsheet["authentication"]; exists {
		authStr := strings.ToLower(fmt.Sprintf("%v", authVal))
		if strings.Contains(authStr, "bearer") || strings.Contains(authStr, "api_key") ||
			strings.Contains(authStr, "oauth") || strings.Contains(authStr, "mcp") ||
			strings.Contains(authStr, "l402") || strings.Contains(authStr, "none") {
			hasValidAuth = true
		}
	}
	hasValidPricing := false
	if pricingVal, exists := factsheet["pricing"]; exists {
		pricingStr := fmt.Sprintf("%v", pricingVal)
		if len(pricingStr) > 2 {
			hasValidPricing = true
		}
	}
	if hasValidAuth && hasValidPricing {
		scoreComponents += 1.0
	} else {
		scoreComponents += 0.5
		if !hasValidAuth {
			*flags = append(*flags, "Ambiguous authentication specification")
		}
		if !hasValidPricing {
			*flags = append(*flags, "Ambiguous pricing model in factsheet")
		}
	}

	// 4. Hype Superlative & Hallucinatory Claim Audit
	factsheetBytes, _ := json.Marshal(factsheet)
	factsheetText := strings.ToLower(string(factsheetBytes))

	hypeFound := 0
	for _, hype := range e.config.HypeSuperlatives {
		if strings.Contains(factsheetText, strings.ToLower(hype)) {
			hypeFound++
			penalty += 0.15
			issue := fmt.Sprintf("Unverifiable marketing hype claim: '%s'", hype)
			*flags = append(*flags, issue)
			*claims = append(*claims, ClaimAudit{
				Claim:       hype,
				Field:       "factsheet_content",
				Flag:        FlagHypeSuperlative,
				IsGrounded:  false,
				Explanation: "Superlative cannot be mathematically verified by autonomous agents",
			})
		}
	}

	// SLA latency claim audit
	if strings.Contains(factsheetText, "0ms") || strings.Contains(factsheetText, "0.00ms") || strings.Contains(factsheetText, "zero latency") {
		penalty += 0.20
		*flags = append(*flags, "Physically impossible 0ms latency assertion detected")
		*claims = append(*claims, ClaimAudit{
			Claim:       "0ms latency",
			Field:       "latency_sla",
			Flag:        FlagUnrealisticSLA,
			IsGrounded:  false,
			Explanation: "Zero-latency assertion violates physics and network propagation standards",
		})
	}

	if hypeFound == 0 {
		scoreComponents += 1.0
	}

	// Token budget check
	approxTokens := len(factsheetText) / 4
	if approxTokens > e.config.MaxFactsheetTokens {
		penalty += 0.10
		*flags = append(*flags, fmt.Sprintf("Factsheet size (%d tokens) exceeds recommended limit (%d)", approxTokens, e.config.MaxFactsheetTokens))
	}

	rawGrounding := scoreComponents / maxComponents
	return math.Max(0.0, math.Min(1.0, rawGrounding)), math.Min(0.50, penalty)
}

// computeRelevance assesses semantic overlap between the query and candidate ad keywords/attributes.
func (e *Engine) computeRelevance(query string, candidate QualityCandidate) float64 {
	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		return 0.50 // neutral baseline
	}

	// Build candidate corpus
	candidateTokens := make(map[string]bool)
	for _, kw := range candidate.Keywords {
		for _, t := range tokenize(kw) {
			candidateTokens[t] = true
		}
	}
	for _, t := range tokenize(candidate.Category) {
		candidateTokens[t] = true
	}
	for _, t := range tokenize(candidate.Advertiser) {
		candidateTokens[t] = true
	}
	if svc, ok := candidate.Factsheet["service_name"]; ok {
		for _, t := range tokenize(fmt.Sprintf("%v", svc)) {
			candidateTokens[t] = true
		}
	}

	matchCount := 0
	for _, qt := range qTokens {
		if candidateTokens[qt] {
			matchCount++
		}
	}

	overlapRatio := float64(matchCount) / float64(len(qTokens))

	// Boost if category appears directly in query
	categoryTokens := tokenize(candidate.Category)
	categoryMatch := false
	for _, ct := range categoryTokens {
		for _, qt := range qTokens {
			if ct == qt {
				categoryMatch = true
				break
			}
		}
	}

	finalRelevance := overlapRatio
	if categoryMatch {
		finalRelevance = math.Min(1.0, finalRelevance+0.25)
	}

	// Floor at 0.10 so partially related tools have a small non-zero score
	return math.Max(0.10, math.Min(1.0, finalRelevance))
}

// computeAdoptionScore blends CTR and tool call execution success with Bayesian priors.
func (e *Engine) computeAdoptionScore(candidate QualityCandidate) float64 {
	// If candidate is brand new (0 CTR and 0 tool success), return neutral prior 0.50
	if candidate.HistoricalCTR <= 0 && candidate.ToolCallSuccess <= 0 {
		return 0.50
	}

	// Prior expectation: CTR ~0.08 (8%) is considered high for agent tool adoption
	normalizedCTR := math.Min(1.0, candidate.HistoricalCTR/0.10) // 10% CTR = 1.0

	// Tool execution success rate directly reflects reliability
	toolSuccess := candidate.ToolCallSuccess
	if toolSuccess <= 0 {
		toolSuccess = 0.50
	}

	// 40% CTR adoption + 60% tool call reliability
	return (0.40 * normalizedCTR) + (0.60 * toolSuccess)
}

// classifyTier maps the numerical AQS to an operational QualityTier and clearing multiplier.
func (e *Engine) classifyTier(aqs float64) (QualityTier, float64, bool) {
	switch {
	case aqs >= 9.0:
		return TierExcellent, 0.80, true // -20% discount
	case aqs >= 7.5:
		return TierGood, 0.90, true // -10% discount
	case aqs >= 5.0:
		return TierNeutral, 1.00, true // baseline price
	case aqs >= e.config.MinEligibleAQS:
		return TierSubstandard, 1.25, true // +25% penalty premium
	default:
		return TierDisqualified, 0.00, false // Disqualified
	}
}

// tokenize splits text into normalized alphanumeric lower-case word tokens.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			if current.Len() > 1 && !isStopWord(current.String()) {
				tokens = append(tokens, current.String())
			}
			current.Reset()
		}
	}
	if current.Len() > 1 && !isStopWord(current.String()) {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// isStopWord filters common trivial English particles.
func isStopWord(w string) bool {
	switch w {
	case "a", "an", "the", "and", "or", "in", "on", "at", "to", "for", "with", "by", "is", "it", "of", "from", "as", "need", "want":
		return true
	default:
		return false
	}
}

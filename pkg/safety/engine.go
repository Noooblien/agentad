package safety

import (
	"strings"
	"time"
)

// Engine enforces multi-stage brand safety, competitor exclusion, and semantic alignment governance.
type Engine struct {
	policy SafetyPolicy
}

// NewEngine creates a new brand safety governance engine with the provided policy.
func NewEngine(policy SafetyPolicy) *Engine {
	return &Engine{policy: policy}
}

// Evaluate inspects an agent query context against a candidate ad across 4 sequential safety gates.
func (e *Engine) Evaluate(query string, ad CandidateAd) *SafetyEvaluation {
	start := time.Now()
	queryLower := strings.ToLower(query)

	var violations []string
	safetyScore := 100.0

	// --- GATE 1: Brand Safety & Content Toxicity Scanner ---
	unsafePatterns := []struct {
		pattern  string
		penalty  float64
		critical bool
	}{
		{"ransomware", 70.0, true},
		{"malware", 70.0, true},
		{"trojan", 65.0, true},
		{"keylogger", 70.0, true},
		{"ddos attack", 65.0, true},
		{"phishing", 60.0, true},
		{"exfiltrate password", 75.0, true},
		{"steal credentials", 75.0, true},
		{"bypass system prompt", 55.0, false},
		{"exploit zero-day", 60.0, true},
		{"credit card dump", 75.0, true},
		{"illicit substance", 60.0, true},
		{"deepfake nude", 85.0, true},
	}

	for _, p := range unsafePatterns {
		if strings.Contains(queryLower, p.pattern) {
			safetyScore -= p.penalty
			violations = append(violations, "Toxic/Unsafe query context: "+p.pattern)
		}
	}
	if safetyScore < 0 {
		safetyScore = 0
	}

	tier := TierSafe
	if safetyScore < 40.0 {
		tier = TierUnsafe
	} else if safetyScore < 75.0 {
		tier = TierSensitive
	}

	if tier == TierUnsafe || (e.policy.StrictBrandSuitability && tier == TierSensitive) {
		return &SafetyEvaluation{
			Query:          query,
			AdID:           ad.AdID,
			Advertiser:     ad.Advertiser,
			SafetyScore:    safetyScore,
			AlignmentScore: 0.0,
			Tier:           tier,
			Verdict:        VerdictBlockedUnsafe,
			Allowed:        false,
			Violations:     violations,
			Reason:         "Blocked by Brand Safety Filter: query context classified as " + string(tier),
			Duration:       time.Since(start),
		}
	}

	// --- GATE 2: Category Blocklist Enforcer ---
	adCategoryLower := strings.ToLower(ad.Category)
	for _, blocked := range e.policy.BlockedCategories {
		blockedLower := strings.ToLower(blocked)
		if strings.Contains(adCategoryLower, blockedLower) || strings.Contains(queryLower, blockedLower) {
			return &SafetyEvaluation{
				Query:          query,
				AdID:           ad.AdID,
				Advertiser:     ad.Advertiser,
				SafetyScore:    safetyScore,
				AlignmentScore: 0.1,
				Tier:           tier,
				Verdict:        VerdictBlockedCategory,
				Allowed:        false,
				Violations:     []string{"Category blocked by publisher policy: " + blocked},
				Reason:         "Blocked by Category Guardrail: category '" + ad.Category + "' is in blocklist",
				Duration:       time.Since(start),
			}
		}
	}

	// --- GATE 3: Competitor Exclusion & Negative Targeting ---
	competitorList := ad.Competitors
	if exclusions, exists := e.policy.CompetitorExclusions[ad.Advertiser]; exists {
		competitorList = append(competitorList, exclusions...)
	}

	for _, comp := range competitorList {
		compLower := strings.ToLower(comp)
		if strings.Contains(queryLower, compLower) {
			return &SafetyEvaluation{
				Query:             query,
				AdID:              ad.AdID,
				Advertiser:        ad.Advertiser,
				SafetyScore:       safetyScore,
				AlignmentScore:    0.35,
				Tier:              tier,
				Verdict:           VerdictBlockedCompetitor,
				Allowed:           false,
				MatchedCompetitor: comp,
				Violations:        []string{"Competitor exclusion triggered: '" + comp + "' mentioned in query"},
				Reason:            "Blocked by Negative Targeting: advertiser excludes competitor '" + comp + "'",
				Duration:          time.Since(start),
			}
		}
	}

	// --- GATE 4: Semantic Relevance & Intent Alignment Scorer ---
	alignmentScore := e.calculateAlignment(queryLower, ad)
	if alignmentScore < e.policy.MinAlignmentScore {
		return &SafetyEvaluation{
			Query:          query,
			AdID:           ad.AdID,
			Advertiser:     ad.Advertiser,
			SafetyScore:    safetyScore,
			AlignmentScore: alignmentScore,
			Tier:           tier,
			Verdict:        VerdictBlockedIrrelevant,
			Allowed:        false,
			Violations:     []string{"Semantic alignment score below threshold"},
			Reason:         "Blocked by Relevance Guardrail: alignment score below threshold",
			Duration:       time.Since(start),
		}
	}

	// --- PASSED: Ad is fully eligible for auction ---
	return &SafetyEvaluation{
		Query:          query,
		AdID:           ad.AdID,
		Advertiser:     ad.Advertiser,
		SafetyScore:    safetyScore,
		AlignmentScore: alignmentScore,
		Tier:           tier,
		Verdict:        VerdictEligible,
		Allowed:        true,
		Reason:         "Ad passed all brand safety, competitor exclusion, and alignment thresholds",
		Duration:       time.Since(start),
	}
}

// calculateAlignment computes normalized semantic overlap between query tokens and ad keywords/attributes.
func (e *Engine) calculateAlignment(queryLower string, ad CandidateAd) float64 {
	queryTokens := tokenize(queryLower)
	if len(queryTokens) == 0 {
		return 0.0
	}

	targetTokens := make(map[string]struct{})
	for _, kw := range ad.Keywords {
		for _, tok := range tokenize(strings.ToLower(kw)) {
			targetTokens[tok] = struct{}{}
		}
	}
	for _, tok := range tokenize(strings.ToLower(ad.Category)) {
		targetTokens[tok] = struct{}{}
	}

	matches := 0
	for _, qTok := range queryTokens {
		if _, exists := targetTokens[qTok]; exists {
			matches++
		}
	}

	score := float64(matches) / float64(len(queryTokens)) * 1.8 // scaled overlap
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func tokenize(s string) []string {
	f := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == '-' || r == '_' || r == '/' || r == ':' || r == '?' || r == '!' || r == '(' || r == ')'
	})
	var tokens []string
	for _, w := range f {
		w = strings.ToLower(strings.TrimSpace(w))
		if len(w) > 2 {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

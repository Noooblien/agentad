package quality

import (
	"time"
)

// QualityTier classifies an ad's overall quality and auction eligibility.
type QualityTier string

const (
	TierExcellent    QualityTier = "EXCELLENT"    // AQS 9.0 - 10.0: High factual grounding, earns 20% discount
	TierGood         QualityTier = "GOOD"         // AQS 7.5 - 8.9: Strong relevance & grounding, earns 10% discount
	TierNeutral      QualityTier = "NEUTRAL"      // AQS 5.0 - 7.4: Standard baseline, no discount or penalty
	TierSubstandard  QualityTier = "SUBSTANDARD"  // AQS 3.5 - 4.9: Marginal, incurs 25% penalty premium
	TierDisqualified QualityTier = "DISQUALIFIED" // AQS < 3.5: Failed factual grounding / hallucination audit
)

// ClaimFlag categorizes findings from factsheet statement verification.
type ClaimFlag string

const (
	FlagValidClaim       ClaimFlag = "VALID_CLAIM"
	FlagHypeSuperlative  ClaimFlag = "HYPE_SUPERLATIVE"
	FlagUnverifiableCost ClaimFlag = "UNVERIFIABLE_COST"
	FlagUnrealisticSLA   ClaimFlag = "UNREALISTIC_SLA"
	FlagMissingAuth      ClaimFlag = "MISSING_AUTH"
	FlagMissingEndpoint  ClaimFlag = "MISSING_ENDPOINT"
	FlagExcessiveTokens  ClaimFlag = "EXCESSIVE_TOKENS"
)

// ClaimAudit represents the granular verification of an individual ad claim.
type ClaimAudit struct {
	Claim       string    `json:"claim"`
	Field       string    `json:"field"`
	Flag        ClaimFlag `json:"flag"`
	IsGrounded  bool      `json:"is_grounded"`
	Explanation string    `json:"explanation"`
}

// QualityCandidate represents an ad entrant evaluated by the quality engine.
type QualityCandidate struct {
	AdID            string                 `json:"ad_id"`
	Advertiser      string                 `json:"advertiser"`
	Category        string                 `json:"category"`
	BidCPM          float64                `json:"bid_cpm"`
	Keywords        []string               `json:"keywords"`
	HistoricalCTR   float64                `json:"historical_ctr"`    // Historical agent adoption rate (0.0 to 1.0)
	ToolCallSuccess float64                `json:"tool_call_success"` // Historical successful completion (0.0 to 1.0)
	Factsheet       map[string]interface{} `json:"factsheet"`
}

// QualityBreakdown details the mathematical components of the Agent Ad Quality Score.
type QualityBreakdown struct {
	AdID               string        `json:"ad_id"`
	Advertiser         string        `json:"advertiser"`
	QueryIntent        string        `json:"query_intent"`
	GroundingScore     float64       `json:"grounding_score"`     // 0.0 - 1.0 (factsheet verification & schema completeness)
	RelevanceScore     float64       `json:"relevance_score"`     // 0.0 - 1.0 (intent token & semantic fit)
	AdoptionScore      float64       `json:"adoption_score"`      // 0.0 - 1.0 (historical agent adoption & execution)
	RiskPenalty        float64       `json:"risk_penalty"`        // 0.0 - 0.5 (hallucination & token bloat penalty)
	RawScore           float64       `json:"raw_score"`           // 0.0 - 1.0 composite
	QualityScore       float64       `json:"quality_score"`       // 1.0 - 10.0 standard AQS
	Tier               QualityTier   `json:"tier"`
	RawBidCPM          float64       `json:"raw_bid_cpm"`
	EffectiveAdRank    float64       `json:"effective_ad_rank"`    // RawBidCPM * (QualityScore / 10.0)
	ClearingMultiplier float64       `json:"clearing_multiplier"` // e.g. 0.80 for Excellent (-20%), 1.25 for Substandard (+25%)
	EffectiveCostCPM   float64       `json:"effective_cost_cpm"`  // RawBidCPM * ClearingMultiplier
	Eligible           bool          `json:"eligible"`
	ClaimsAudited      []ClaimAudit  `json:"claims_audited"`
	FlaggedIssues      []string      `json:"flagged_issues,omitempty"`
	Duration           time.Duration `json:"duration"`
}

// QualitySummary aggregates quality score distribution across candidate ads.
type QualitySummary struct {
	TotalEvaluated    int                 `json:"total_evaluated"`
	EligibleCount     int                 `json:"eligible_count"`
	DisqualifiedCount int                 `json:"disqualified_count"`
	AverageAQS        float64             `json:"average_aqs"`
	TierBreakdown     map[QualityTier]int `json:"tier_breakdown"`
	Evaluations       []*QualityBreakdown `json:"evaluations"`
	Duration          time.Duration       `json:"duration"`
}

// QualityConfig parametrizes the Quality Scoring and Grounding Engine.
type QualityConfig struct {
	WeightGrounding      float64  `json:"weight_grounding"`       // Default: 0.40
	WeightRelevance      float64  `json:"weight_relevance"`       // Default: 0.35
	WeightAdoption       float64  `json:"weight_adoption"`        // Default: 0.25
	MinEligibleAQS       float64  `json:"min_eligible_aqs"`       // Default: 3.5
	MaxFactsheetTokens   int      `json:"max_factsheet_tokens"`   // Context token budget limit
	HypeSuperlatives     []string `json:"hype_superlatives"`      // Tokens triggering hype penalty
	MandatorySchemaKeys  []string `json:"mandatory_schema_keys"`  // Required keys in factsheet
}

// DefaultQualityConfig returns balanced production defaults.
func DefaultQualityConfig() QualityConfig {
	return QualityConfig{
		WeightGrounding:    0.40,
		WeightRelevance:    0.35,
		WeightAdoption:     0.25,
		MinEligibleAQS:     3.50,
		MaxFactsheetTokens: 500,
		HypeSuperlatives: []string{
			"100% free forever",
			"unlimited free",
			"best in the world",
			"revolutionary magic",
			"miracle ai",
			"guaranteed 1000x",
			"zero latency infinite",
			"unbeatable superior",
		},
		MandatorySchemaKeys: []string{
			"service_name",
			"api_endpoint",
			"authentication",
			"pricing",
		},
	}
}

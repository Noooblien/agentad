package safety

import "time"

// SafetyTier indicates the brand suitability and risk classification of an agent query context.
type SafetyTier string

const (
	TierSafe      SafetyTier = "SAFE"      // Context is clean, professional, and compliant with brand standards
	TierSensitive SafetyTier = "SENSITIVE" // Context involves mild controversy or sensitive topics requiring advertiser opt-in
	TierUnsafe    SafetyTier = "UNSAFE"    // Context involves malware, illicit acts, hate speech, or dangerous exploits
)

// SafetyVerdict defines the final governance decision for ad candidate eligibility in the auction.
type SafetyVerdict string

const (
	VerdictEligible          SafetyVerdict = "ELIGIBLE"           // Ad passes all brand safety, competitor, and alignment gates
	VerdictBlockedUnsafe     SafetyVerdict = "BLOCKED_UNSAFE"     // Blocked due to toxic, illegal, or malicious context
	VerdictBlockedCategory   SafetyVerdict = "BLOCKED_CATEGORY"   // Blocked by publisher or advertiser category blocklists
	VerdictBlockedCompetitor SafetyVerdict = "BLOCKED_COMPETITOR" // Blocked due to active competitor conflict or negative targeting
	VerdictBlockedIrrelevant SafetyVerdict = "BLOCKED_IRRELEVANT" // Blocked due to low semantic alignment with user intent
)

// SafetyPolicy defines the brand safety guardrails, blocklists, and alignment thresholds.
type SafetyPolicy struct {
	BlockedCategories       []string            `json:"blocked_categories"`        // e.g. "malware", "weapons", "adult", "gambling"
	CompetitorExclusions    map[string][]string `json:"competitor_exclusions"`     // AdvertiserName -> list of competitor brand tokens
	MinAlignmentScore       float64             `json:"min_alignment_score"`       // Threshold (0.0 to 1.0) below which ad is rejected
	StrictBrandSuitability  bool                `json:"strict_brand_suitability"`  // Reject TierSensitive contexts
}

// CandidateAd represents an ad candidate entering the brand safety and alignment pipeline.
type CandidateAd struct {
	AdID         string                 `json:"ad_id"`
	Advertiser   string                 `json:"advertiser"`
	Category     string                 `json:"category"`
	Competitors  []string               `json:"competitors,omitempty"`
	Keywords     []string               `json:"keywords"`
	Factsheet    map[string]interface{} `json:"factsheet,omitempty"`
}

// SafetyEvaluation records the multi-stage brand safety inspection result.
type SafetyEvaluation struct {
	Query             string        `json:"query"`
	AdID              string        `json:"ad_id"`
	Advertiser        string        `json:"advertiser"`
	SafetyScore       float64       `json:"safety_score"`       // 0.0 to 100.0 (higher = safer)
	AlignmentScore    float64       `json:"alignment_score"`    // 0.0 to 1.0 (semantic relevance)
	Tier              SafetyTier    `json:"tier"`
	Verdict           SafetyVerdict `json:"verdict"`
	Allowed           bool          `json:"allowed"`
	Violations        []string      `json:"violations,omitempty"`
	MatchedCompetitor string        `json:"matched_competitor,omitempty"`
	Reason            string        `json:"reason"`
	Duration          time.Duration `json:"duration"`
}

// SafetySummary aggregates metrics across multiple brand safety evaluations.
type SafetySummary struct {
	TotalEvaluated    int                 `json:"total_evaluated"`
	PassedCount       int                 `json:"passed_count"`
	UnsafeBlocked     int                 `json:"unsafe_blocked"`
	CategoryBlocked   int                 `json:"category_blocked"`
	CompetitorBlocked int                 `json:"competitor_blocked"`
	IrrelevantBlocked int                 `json:"irrelevant_blocked"`
	Evaluations       []*SafetyEvaluation `json:"evaluations"`
	Duration          time.Duration       `json:"duration"`
}

// DefaultSafetyPolicy returns a robust production brand safety configuration.
func DefaultSafetyPolicy() SafetyPolicy {
	return SafetyPolicy{
		BlockedCategories: []string{
			"malware",
			"ransomware",
			"phishing",
			"weapons",
			"illicit_substances",
			"gambling",
			"adult",
			"hate_speech",
			"crypto_drainer",
		},
		CompetitorExclusions: map[string][]string{
			"Pinecone Serverless": {"weaviate", "milvus", "qdrant", "chroma"},
			"Datadog Agent":       {"dynatrace", "newrelic", "splunk"},
			"Snowflake Cortex":    {"databricks", "bigquery", "redshift"},
		},
		MinAlignmentScore:      0.40,
		StrictBrandSuitability: true,
	}
}

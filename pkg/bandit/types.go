package bandit

import (
	"time"
)

// BanditAlgorithm specifies the exploration/exploitation algorithm.
type BanditAlgorithm string

const (
	AlgorithmThompsonSampling BanditAlgorithm = "thompson_sampling"
	AlgorithmUCB1             BanditAlgorithm = "ucb1"
	AlgorithmEpsilonGreedy    BanditAlgorithm = "epsilon_greedy"
)

// FeedbackEventType represents a user or agent interaction event.
type FeedbackEventType string

const (
	EventImpression FeedbackEventType = "impression"
	EventClick      FeedbackEventType = "click"
	EventConversion FeedbackEventType = "conversion"
	EventDrop       FeedbackEventType = "drop"
	EventTimeout    FeedbackEventType = "timeout"
)

// SelectionMode specifies whether an arm was chosen via exploration or exploitation.
type SelectionMode string

const (
	ModeExploitation SelectionMode = "EXPLOITATION"
	ModeExploration  SelectionMode = "EXPLORATION"
	ModeColdStart    SelectionMode = "COLD_START"
)

// BanditConfig holds tuning parameters for Bayesian exploration.
type BanditConfig struct {
	Algorithm               BanditAlgorithm `json:"algorithm"`
	PriorAlpha              float64         `json:"prior_alpha"`                 // Default 1.0 (uninformative prior)
	PriorBeta               float64         `json:"prior_beta"`                  // Default 1.0 (uninformative prior)
	ColdStartThreshold      int             `json:"cold_start_threshold"`        // Minimum impressions before full exploitation
	UCBConfidenceScale      float64         `json:"ucb_confidence_scale"`        // Exploration bonus scaling factor (default 1.414)
	Epsilon                 float64         `json:"epsilon"`                     // Exploration probability for epsilon-greedy (default 0.10)
	DecayFactor             float64         `json:"decay_factor"`                // Discount factor for historical observations (default 0.995)
}

// DefaultBanditConfig returns production-tuned defaults for agent discovery auctions.
func DefaultBanditConfig() BanditConfig {
	return BanditConfig{
		Algorithm:          AlgorithmThompsonSampling,
		PriorAlpha:         1.0,
		PriorBeta:          9.0, // Prior assumption: ~10% baseline CTR for targeted sponsored tools
		ColdStartThreshold: 10,
		UCBConfidenceScale: 1.414,
		Epsilon:            0.10,
		DecayFactor:        0.995,
	}
}

// ArmState represents the empirical tracking state of an advertiser campaign arm.
type ArmState struct {
	CampaignID      string    `json:"campaign_id"`
	AdvertiserName  string    `json:"advertiser_name"`
	Category        string    `json:"category"`
	Impressions     int       `json:"impressions"`
	Clicks          int       `json:"clicks"`
	Conversions     int       `json:"conversions"`
	Alpha           float64   `json:"alpha"`           // Beta distribution parameter (successes + prior)
	Beta            float64   `json:"beta"`            // Beta distribution parameter (failures + prior)
	EmpiricalCTR    float64   `json:"empirical_ctr"`   // Clicks / Impressions
	EmpiricalCVR    float64   `json:"empirical_cvr"`   // Conversions / Impressions
	LastSampledVal  float64   `json:"last_sampled_val"`
	InColdStart     bool      `json:"in_cold_start"`
	LastUpdated     time.Time `json:"last_updated"`
}

// CandidateArm represents an arm candidate entering an auction round.
type CandidateArm struct {
	CampaignID     string  `json:"campaign_id"`
	AdvertiserName string  `json:"advertiser_name"`
	Category       string  `json:"category"`
	BidCPM         float64 `json:"bid_cpm"`
	RelevanceScore float64 `json:"relevance_score"`
	TrustScore     float64 `json:"trust_score"`
}

// ArmSelectionOutcome represents the selected arm and the decision rationale.
type ArmSelectionOutcome struct {
	SelectedArmID      string          `json:"selected_arm_id"`
	AdvertiserName     string          `json:"advertiser_name"`
	Category           string          `json:"category"`
	Mode               SelectionMode   `json:"mode"`
	Algorithm          BanditAlgorithm `json:"algorithm"`
	SampledScore       float64         `json:"sampled_score"`
	ExpectedCTR        float64         `json:"expected_ctr"`
	EffectiveCPM       float64         `json:"effective_cpm"` // eCPM = SampledCTR * BidCPM * Relevance
	CandidateCount     int             `json:"candidate_count"`
	CandidateRankings  []ArmRanking    `json:"candidate_rankings"`
	EstimatedRegret    float64         `json:"estimated_regret"`
	ExecutionTimeUs    int64           `json:"execution_time_us"`
}

// ArmRanking details a candidate's evaluated score in the selection round.
type ArmRanking struct {
	CampaignID     string        `json:"campaign_id"`
	AdvertiserName string        `json:"advertiser_name"`
	BidCPM         float64       `json:"bid_cpm"`
	RelevanceScore float64       `json:"relevance_score"`
	SampledCTR     float64       `json:"sampled_ctr"`
	EffectiveCPM   float64       `json:"effective_cpm"`
	Mode           SelectionMode `json:"mode"`
	Rank           int           `json:"rank"`
}

// BatchSimulationSummary summarizes multi-round bandit simulation performance.
type BatchSimulationSummary struct {
	ScenarioName      string                 `json:"scenario_name"`
	Category          string                 `json:"category"`
	TotalRounds       int                    `json:"total_rounds"`
	TotalImpressions  int                    `json:"total_impressions"`
	TotalClicks       int                    `json:"total_clicks"`
	TotalConversions  int                    `json:"total_conversions"`
	AverageCTR        float64                `json:"average_ctr"`
	CumulativeRegret  float64                `json:"cumulative_regret"`
	Arms              []*ArmState            `json:"arms"`
	Rounds            []*ArmSelectionOutcome `json:"rounds,omitempty"`
}

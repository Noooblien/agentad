package fraud

import (
	"time"
)

// FraudVector categorizes specific automated click/impression fraud and prompt leakage attack patterns.
type FraudVector string

const (
	VectorSyntheticFlooding   FraudVector = "SYNTHETIC_QUERY_FLOODING"
	VectorKeywordStacking     FraudVector = "KEYWORD_STACKING"
	VectorPromptLeakageProbe  FraudVector = "PROMPT_LEAKAGE_PROBE"
	VectorSybilRecycling      FraudVector = "SYBIL_AGENT_RECYCLING"
	VectorUnnaturalPacing     FraudVector = "UNNATURAL_BURST_PACING"
	VectorCanaryExfiltration  FraudVector = "CANARY_TOKEN_EXFILTRATION"
)

// FraudRiskLevel categorizes the evaluated threat severity of incoming traffic.
type FraudRiskLevel string

const (
	RiskClean      FraudRiskLevel = "CLEAN"
	RiskSuspicious FraudRiskLevel = "SUSPICIOUS"
	RiskFraudulent FraudRiskLevel = "FRAUDULENT"
)

// TrafficVerdict holds the comprehensive evaluation results of an intent query or bid request.
type TrafficVerdict struct {
	IntentQuery        string         `json:"intent_query"`
	AgentID            string         `json:"agent_id"`
	PublisherID        string         `json:"publisher_id"`
	Timestamp          time.Time      `json:"timestamp"`
	OverallFraudScore  float64        `json:"overall_fraud_score"`  // 0.00 (Clean) to 1.00 (Fraudulent)
	RiskLevel          FraudRiskLevel `json:"risk_level"`           // CLEAN, SUSPICIOUS, FRAUDULENT
	ActionTaken        string         `json:"action_taken"`         // ALLOW, THROTTLE_AND_FLAG, BLOCK_AND_FREEZE_ESCROW
	EntropyScore       float64        `json:"entropy_score"`        // Shannon entropy in bits per character (typical natural: 3.5 - 4.8)
	BurstVelocityRPM   float64        `json:"burst_velocity_rpm"`   // Requests per minute in current sliding window
	RepetitionRatio    float64        `json:"repetition_ratio"`     // Ratio of duplicate/near-identical queries
	TriggeredVectors   []FraudVector  `json:"triggered_vectors"`
	Reasons            []string       `json:"reasons"`
	ExecutionLatencyMs float64        `json:"execution_latency_ms"`
}

// DetectorConfig defines tunable thresholds for fraud detection and prompt protection.
type DetectorConfig struct {
	MaxVelocityRPM       float64       `json:"max_velocity_rpm"`        // Default: 120 RPM
	WindowDuration       time.Duration `json:"window_duration"`        // Sliding window duration (default: 60s)
	MinShannonEntropy    float64       `json:"min_shannon_entropy"`     // Minimum entropy for natural language queries (default: 2.8)
	MaxRepetitionRatio   float64       `json:"max_repetition_ratio"`    // Max acceptable duplicate ratio (default: 0.30)
	FraudScoreThreshold  float64       `json:"fraud_score_threshold"`   // Threshold to flag FRAUDULENT (default: 0.60)
	SuspiciousThreshold  float64       `json:"suspicious_threshold"`    // Threshold to flag SUSPICIOUS (default: 0.25)
}

// DefaultDetectorConfig returns production-grade baseline thresholds for AAP ad networks.
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		MaxVelocityRPM:      120.0,
		WindowDuration:      60 * time.Second,
		MinShannonEntropy:   2.80,
		MaxRepetitionRatio:  0.30,
		FraudScoreThreshold: 0.60,
		SuspiciousThreshold: 0.25,
	}
}

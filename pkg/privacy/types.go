package privacy

import (
	"time"
)

// PIICategory classifies types of Personally Identifiable Information and enterprise secrets.
type PIICategory string

const (
	PIIEmail            PIICategory = "EMAIL"
	PIIPhone            PIICategory = "PHONE"
	PIISSN              PIICategory = "SSN"
	PIICreditCard       PIICategory = "CREDIT_CARD"
	PIIIPAddress        PIICategory = "IP_ADDRESS"
	PIIAPIToken         PIICategory = "API_KEY"
	PIIAWSCredential    PIICategory = "AWS_CREDENTIAL"
	PIIJWT              PIICategory = "JWT_TOKEN"
	PIIMedicalCondition PIICategory = "HEALTH_HIPAA"
	PIIGeoAddress       PIICategory = "STREET_ADDRESS"
)

// RedactionEntity represents an identified and scrubbed PII element.
type RedactionEntity struct {
	Category    PIICategory `json:"category"`
	Original    string      `json:"original"`
	Mask        string      `json:"mask"`
	Start       int         `json:"start"`
	End         int         `json:"end"`
	Severity    string      `json:"severity"` // "CRITICAL", "HIGH", "MEDIUM"
	Description string      `json:"description"`
}

// PrivacyConfig configures the Differential Privacy & Redaction Engine.
type PrivacyConfig struct {
	Epsilon           float64 `json:"epsilon"`             // Differential Privacy Budget (e.g. 1.0)
	Delta             float64 `json:"delta"`               // Differential Privacy Relaxation factor (e.g. 1e-5)
	KAnonymity        int     `json:"k_anonymity"`         // Minimum cluster size for semantic generalization
	EnableRedaction   bool    `json:"enable_redaction"`    // Enable automated PII entity scrubbing
	EnableDPNoise     bool    `json:"enable_dp_noise"`     // Enable Laplace vector noise addition
	MaxSessionEpsilon float64 `json:"max_session_epsilon"` // Maximum allowable privacy loss per session
}

// DefaultPrivacyConfig returns battle-tested production enterprise privacy settings.
func DefaultPrivacyConfig() PrivacyConfig {
	return PrivacyConfig{
		Epsilon:           1.0,
		Delta:             1e-5,
		KAnonymity:        5,
		EnableRedaction:   true,
		EnableDPNoise:     true,
		MaxSessionEpsilon: 10.0,
	}
}

// PrivacyEvaluationResult encapsulates the output of a privacy evaluation pass.
type PrivacyEvaluationResult struct {
	SessionID              string            `json:"session_id"`
	OriginalQuery          string            `json:"original_query"`
	SanitizedQuery         string            `json:"sanitized_query"`
	GeneralizationCategory string            `json:"generalization_category"`
	RedactedEntities       []RedactionEntity `json:"redacted_entities"`
	TotalPIIFound          int               `json:"total_pii_found"`
	EpsilonCost            float64           `json:"epsilon_cost"`
	RemainingBudget        float64           `json:"remaining_budget"`
	BudgetExhausted        bool              `json:"budget_exhausted"`
	VectorSimilarityLoss   float64           `json:"vector_similarity_loss"` // Cosine drift from DP noise (0.0 to 1.0)
	PrivacyScore           float64           `json:"privacy_score"`          // 0.0 to 100.0 (Higher is safer)
	Allowed                bool              `json:"allowed"`
	AuditHash              string            `json:"audit_hash"`
	DurationMicroseconds   int64             `json:"duration_us"`
	Timestamp              time.Time         `json:"timestamp"`
}

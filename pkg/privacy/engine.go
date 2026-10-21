package privacy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"time"
)

// Engine is the central coordinator for Zero-PII sanitization and Differential Privacy.
type Engine struct {
	config     PrivacyConfig
	redactor   *Redactor
	anonymizer *Anonymizer
	budgets    map[string]float64 // session_id -> accumulated epsilon
	mu         sync.RWMutex
}

// NewEngine initializes the Differential Privacy Engine.
func NewEngine(cfg PrivacyConfig) *Engine {
	return &Engine{
		config:     cfg,
		redactor:   NewRedactor(),
		anonymizer: NewAnonymizer(cfg.KAnonymity, cfg.Epsilon),
		budgets:    make(map[string]float64),
	}
}

// EvaluateIntent scrubs PII, generalizes intent category, injects DP noise, and accounts privacy budget.
func (e *Engine) EvaluateIntent(sessionID, rawQuery string, optionalVector []float64) *PrivacyEvaluationResult {
	start := time.Now()

	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. Session Privacy Budget Check
	currSpent := e.budgets[sessionID]
	cost := e.config.Epsilon
	remaining := e.config.MaxSessionEpsilon - currSpent
	if remaining < 0 {
		remaining = 0
	}

	budgetExhausted := (currSpent + cost) > e.config.MaxSessionEpsilon

	// 2. Automated PII Redaction
	sanitizedQuery := rawQuery
	var entities []RedactionEntity
	if e.config.EnableRedaction {
		sanitizedQuery, entities = e.redactor.Redact(rawQuery)
	}

	// 3. Semantic K-Anonymity Generalization
	genCategory := e.anonymizer.Generalize(sanitizedQuery)

	// 4. Local Differential Privacy Laplace Noise
	simLoss := 0.0
	if e.config.EnableDPNoise && len(optionalVector) > 0 {
		_, simLoss = e.anonymizer.PerturbVector(optionalVector)
	}

	// Deduct privacy budget for this auction query
	if !budgetExhausted {
		e.budgets[sessionID] += cost
		remaining -= cost
		if remaining < 0 {
			remaining = 0
		}
	}

	// 5. Calculate Comprehensive Privacy Score (0.0 to 100.0)
	// Base score 100. Deduct if critical PII was present in raw input, reward successful redaction & DP protection.
	privacyScore := 100.0
	if len(entities) > 0 {
		// PII was intercepted! Score reflects that data was sanitized before reaching ad network
		for _, ent := range entities {
			switch ent.Severity {
			case "CRITICAL":
				privacyScore = math.Max(85.0, privacyScore-2.0)
			case "HIGH":
				privacyScore = math.Max(90.0, privacyScore-1.0)
			default:
				privacyScore = math.Max(95.0, privacyScore-0.5)
			}
		}
	}

	if budgetExhausted {
		privacyScore = math.Max(0.0, privacyScore-40.0)
	}

	allowed := !budgetExhausted

	// Compute audit hash
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s:%s:%.4f", sessionID, sanitizedQuery, genCategory, e.config.Epsilon)))
	auditHash := hex.EncodeToString(h.Sum(nil))

	return &PrivacyEvaluationResult{
		SessionID:              sessionID,
		OriginalQuery:          rawQuery,
		SanitizedQuery:         sanitizedQuery,
		GeneralizationCategory: genCategory,
		RedactedEntities:       entities,
		TotalPIIFound:          len(entities),
		EpsilonCost:            cost,
		RemainingBudget:        remaining,
		BudgetExhausted:        budgetExhausted,
		VectorSimilarityLoss:   simLoss,
		PrivacyScore:           privacyScore,
		Allowed:                allowed,
		AuditHash:              auditHash,
		DurationMicroseconds:   time.Since(start).Microseconds(),
		Timestamp:              start,
	}
}

// ResetSession resets the accumulated privacy budget for a given session.
func (e *Engine) ResetSession(sessionID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.budgets, sessionID)
}

// GetBudgetStatus returns spent and remaining epsilon for a session.
func (e *Engine) GetBudgetStatus(sessionID string) (spent, remaining float64) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	spent = e.budgets[sessionID]
	remaining = e.config.MaxSessionEpsilon - spent
	if remaining < 0 {
		remaining = 0
	}
	return spent, remaining
}

package fraud

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	promptLeakRegex = regexp.MustCompile(`(?i)(system prompt|reveal system|print prompt|repeat previous|ignore previous|ignore all instructions|show system instructions|output memory|what are your instructions|leak prompt|developer instruction|bypass guardrail|dump prompt|print conversation|repeat above)`)
	canaryRegex     = regexp.MustCompile(`(?i)(canary_[a-f0-9]{8,}|token_leak_|bearer\s+[a-z0-9_\-\.]{20,}|sk-[a-zA-Z0-9]{20,})`)
)

var highValueAdKeywords = map[string]bool{
	"buy": true, "best": true, "cheap": true, "pricing": true, "discount": true,
	"database": true, "serverless": true, "gpu": true, "cloud": true, "hosting": true,
	"deal": true, "free": true, "credit": true, "compare": true, "vector": true,
	"fastest": true, "lowest": true, "provider": true, "trial": true, "quote": true,
}

// Detector provides thread-safe click fraud, synthetic traffic scrubbing, and prompt leakage defense.
type Detector struct {
	mu            sync.RWMutex
	config        DetectorConfig
	requestLog    map[string][]time.Time
	queryHistory  map[string][]string
}

// NewDetector initializes a fraud and leakage detector with the provided configuration.
func NewDetector(cfg DetectorConfig) *Detector {
	return &Detector{
		config:       cfg,
		requestLog:   make(map[string][]time.Time),
		queryHistory: make(map[string][]string),
	}
}

// CalculateShannonEntropy calculates information entropy H(X) in bits per character.
// Natural English prose typically falls in the range 3.5 - 4.8 bits/char.
// Synthetic repetition or padded spam produces significantly lower entropy (< 2.8).
func CalculateShannonEntropy(text string) float64 {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return 0.0
	}

	freq := make(map[rune]float64)
	for _, r := range text {
		freq[r]++
	}

	total := float64(len([]rune(text)))
	var entropy float64
	for _, count := range freq {
		p := count / total
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return math.Round(entropy*100) / 100
}

// DetectKeywordStacking checks if an intent query is unnaturally stuffed with commercial triggers.
func DetectKeywordStacking(text string) (bool, float64) {
	words := strings.Fields(strings.ToLower(text))
	if len(words) < 6 {
		return false, 0.0
	}

	matches := 0
	for _, w := range words {
		// Clean punctuation
		cleaned := strings.Trim(w, ",.!?\"';:()[]{}")
		if highValueAdKeywords[cleaned] {
			matches++
		}
	}

	density := float64(matches) / float64(len(words))
	// If more than 40% of words are commercial keywords, flag stacking
	return density >= 0.40, math.Round(density*100) / 100
}

// DetectPromptLeakageProbe checks if a query attempts to extract system instructions or conversational memory.
func DetectPromptLeakageProbe(text string) (bool, string) {
	if match := promptLeakRegex.FindString(text); match != "" {
		return true, fmt.Sprintf("Probing trigger pattern detected: '%s'", match)
	}
	return false, ""
}

// DetectCanaryExfiltration checks for canary tracking tokens or exfiltrated API keys in queries.
func DetectCanaryExfiltration(text string) bool {
	return canaryRegex.MatchString(text)
}

// AnalyzeTraffic inspects an incoming agent ad request and computes a multi-vector fraud and leakage verdict.
func (d *Detector) AnalyzeTraffic(agentID, pubID, query string) *TrafficVerdict {
	start := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	key := agentID
	if key == "" {
		key = pubID
	}
	if key == "" {
		key = "anonymous_agent"
	}

	// 1. Sliding window velocity calculation (RPM)
	cutoff := now.Add(-d.config.WindowDuration)
	var active []time.Time
	for _, t := range d.requestLog[key] {
		if t.After(cutoff) {
			active = append(active, t)
		}
	}
	active = append(active, now)
	d.requestLog[key] = active

	windowSec := d.config.WindowDuration.Seconds()
	if windowSec <= 0 {
		windowSec = 60.0
	}
	rpm := float64(len(active)) * (60.0 / windowSec)

	// 2. Query repetition tracking
	qHash := hashQuery(query)
	recent := d.queryHistory[key]
	duplicateCount := 0
	for _, h := range recent {
		if h == qHash {
			duplicateCount++
		}
	}
	recent = append(recent, qHash)
	if len(recent) > 50 {
		recent = recent[len(recent)-50:]
	}
	d.queryHistory[key] = recent

	var repRatio float64
	if len(recent) > 0 {
		repRatio = float64(duplicateCount) / float64(len(recent))
	}

	// 3. Vector evaluations
	var vectors []FraudVector
	var reasons []string
	var score float64

	// Shannon Entropy
	entropy := CalculateShannonEntropy(query)
	if len(query) >= 12 && entropy < d.config.MinShannonEntropy {
		score += 0.30
		vectors = append(vectors, VectorSyntheticFlooding)
		reasons = append(reasons, fmt.Sprintf("Low Shannon entropy (%.2f bits/char < %.2f threshold): synthetic/padded query pattern", entropy, d.config.MinShannonEntropy))
	}

	// Burst Velocity
	if rpm > d.config.MaxVelocityRPM {
		score += 0.40
		vectors = append(vectors, VectorUnnaturalPacing)
		reasons = append(reasons, fmt.Sprintf("Unnatural burst request velocity (%.1f RPM exceeds %.1f limit)", rpm, d.config.MaxVelocityRPM))
	}

	// Synthetic Loop Repetition
	if duplicateCount >= 3 || repRatio > d.config.MaxRepetitionRatio {
		score += 0.45
		if !containsVector(vectors, VectorSyntheticFlooding) {
			vectors = append(vectors, VectorSyntheticFlooding)
		}
		reasons = append(reasons, fmt.Sprintf("Repetitive intent flooding detected (%d identical queries in recent window)", duplicateCount))
	}

	// Keyword Stacking
	stacked, density := DetectKeywordStacking(query)
	if stacked {
		score += 0.35
		vectors = append(vectors, VectorKeywordStacking)
		reasons = append(reasons, fmt.Sprintf("Unnatural keyword stacking (density: %.0f%% commercial triggers)", density*100))
	}

	// Prompt Leakage Probing
	isProbe, probeReason := DetectPromptLeakageProbe(query)
	if isProbe {
		score += 0.65
		vectors = append(vectors, VectorPromptLeakageProbe)
		reasons = append(reasons, probeReason)
	}

	// Canary / Secret Exfiltration
	if DetectCanaryExfiltration(query) {
		score += 0.60
		vectors = append(vectors, VectorCanaryExfiltration)
		reasons = append(reasons, "Canary tracking beacon or serialized secret token detected in intent query")
	}

	finalScore := math.Min(1.00, math.Round(score*100)/100)

	// Determine risk level & action
	var risk FraudRiskLevel
	var action string

	if finalScore >= d.config.FraudScoreThreshold {
		risk = RiskFraudulent
		action = "BLOCK_AND_FREEZE_ESCROW"
	} else if finalScore >= d.config.SuspiciousThreshold {
		risk = RiskSuspicious
		action = "THROTTLE_AND_FLAG"
	} else {
		risk = RiskClean
		action = "ALLOW"
		if len(reasons) == 0 {
			reasons = append(reasons, "Traffic verified: natural language entropy and pacing within legitimate thresholds.")
		}
	}

	latencyMs := float64(time.Since(start).Microseconds()) / 1000.0

	return &TrafficVerdict{
		IntentQuery:        query,
		AgentID:            agentID,
		PublisherID:        pubID,
		Timestamp:          now,
		OverallFraudScore:  finalScore,
		RiskLevel:          risk,
		ActionTaken:        action,
		EntropyScore:       entropy,
		BurstVelocityRPM:   math.Round(rpm*10) / 10,
		RepetitionRatio:    math.Round(repRatio*100) / 100,
		TriggeredVectors:   vectors,
		Reasons:            reasons,
		ExecutionLatencyMs: latencyMs,
	}
}

func hashQuery(q string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(q))))
	return hex.EncodeToString(h[:8])
}

func containsVector(slice []FraudVector, v FraudVector) bool {
	for _, item := range slice {
		if item == v {
			return true
		}
	}
	return false
}

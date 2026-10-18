package tracker

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
)

var (
	ErrDuplicateConversion   = errors.New("conversion has already been recorded (idempotency triggered)")
	ErrReplayNonceDetected   = errors.New("cryptographic replay detected: nonce has already been utilized")
	ErrConversionWindowExceeded = errors.New("all touchpoints exceed the maximum lookback attribution window")
	ErrNoTouchesForSession   = errors.New("no preceding impressions found for this agent dialogue session")
	ErrInvalidSignature      = errors.New("conversion attestation signature verification failed")
)

// Tracker provides thread-safe conversion tracking, multi-touch attribution, and cryptographic attestation.
type Tracker struct {
	mu           sync.RWMutex
	config       TrackerConfig
	touches      map[string][]TouchPoint // sessionID -> chronological touches
	conversions  map[string]Conversion   // conversionID -> Conversion
	seenNonces   map[string]time.Time    // nonce -> timestamp
	stats        TrackerStats
	totalTimeMs  float64
}

// NewTracker creates an instance of the conversion tracking engine.
func NewTracker(config TrackerConfig) *Tracker {
	return &Tracker{
		config:      config,
		touches:     make(map[string][]TouchPoint),
		conversions: make(map[string]Conversion),
		seenNonces:  make(map[string]time.Time),
	}
}

// RecordTouch stores an observed impression touchpoint for a session.
func (t *Tracker) RecordTouch(touch TouchPoint) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if touch.Timestamp.IsZero() {
		touch.Timestamp = time.Now().UTC()
	}

	t.touches[touch.SessionID] = append(t.touches[touch.SessionID], touch)
	t.stats.TotalTouches++
	return nil
}

// ComputeConversionHash produces a deterministic cryptographic fingerprint of the conversion event.
func ComputeConversionHash(conv Conversion) string {
	payload := fmt.Sprintf("%s:%s:%s:%s:%s:%.4f:%.4f:%s:%d",
		conv.ConversionID, conv.SessionID, conv.AdvertiserID, conv.CampaignID,
		conv.ActionType, conv.RevenueUSD, conv.PayoutBountyUSD, conv.Nonce,
		conv.Timestamp.UnixNano())
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}

// SignConversion creates an Ed25519 cryptographic Proof-of-Conversion receipt.
func SignConversion(conv *Conversion, keyPair *attest.KeyPair) error {
	if conv.Timestamp.IsZero() {
		conv.Timestamp = time.Now().UTC()
	}
	conv.AttestationHash = ComputeConversionHash(*conv)
	sig := ed25519.Sign(keyPair.PrivateKey, []byte(conv.AttestationHash))
	conv.SignatureHex = hex.EncodeToString(sig)
	conv.PublicKeyHex = keyPair.PubKeyHex
	return nil
}

// VerifyConversion verifies the Ed25519 signature on a conversion receipt.
func VerifyConversion(conv Conversion) bool {
	if conv.SignatureHex == "" || conv.PublicKeyHex == "" {
		return false
	}
	pubBytes, err := hex.DecodeString(conv.PublicKeyHex)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return false
	}
	sigBytes, err := hex.DecodeString(conv.SignatureHex)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}
	computedHash := ComputeConversionHash(conv)
	if computedHash != conv.AttestationHash {
		return false
	}
	return ed25519.Verify(pubBytes, []byte(conv.AttestationHash), sigBytes)
}

// RecordConversion attributes a completed conversion against recorded session touches.
func (t *Tracker) RecordConversion(conv Conversion, model AttributionModel) (*AttributionResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if model == "" {
		model = t.config.DefaultModel
	}

	if conv.Timestamp.IsZero() {
		conv.Timestamp = time.Now().UTC()
	}

	// 1. Idempotency Check
	if _, exists := t.conversions[conv.ConversionID]; exists {
		return &AttributionResult{
			ConversionID: conv.ConversionID,
			SessionID:    conv.SessionID,
			Model:        model,
			Status:       StatusDeduplicated,
			Notes:        "Conversion already processed previously",
		}, ErrDuplicateConversion
	}

	// 2. Cryptographic Replay Protection
	if t.config.ReplayProtection && conv.Nonce != "" {
		if _, seen := t.seenNonces[conv.Nonce]; seen {
			return &AttributionResult{
				ConversionID: conv.ConversionID,
				SessionID:    conv.SessionID,
				Model:        model,
				Status:       StatusDeduplicated,
				Notes:        "Cryptographic nonce replay detected",
			}, ErrReplayNonceDetected
		}
		t.seenNonces[conv.Nonce] = conv.Timestamp
	}

	// 3. Attestation Signature Verification (if signature is provided)
	attestationValid := true
	if conv.SignatureHex != "" {
		if !VerifyConversion(conv) {
			attestationValid = false
			return nil, ErrInvalidSignature
		}
	}

	// 4. Retrieve preceding touches
	sessionTouches, ok := t.touches[conv.SessionID]
	if !ok || len(sessionTouches) == 0 {
		res := &AttributionResult{
			ConversionID:     conv.ConversionID,
			SessionID:        conv.SessionID,
			Model:            model,
			Status:           StatusUnattributed,
			ActionType:       conv.ActionType,
			RevenueUSD:       conv.RevenueUSD,
			PayoutBountyUSD:  conv.PayoutBountyUSD,
			ConversionTime:   conv.Timestamp,
			AttestationValid: attestationValid,
			Notes:            "No preceding impressions recorded for this session",
		}
		t.conversions[conv.ConversionID] = conv
		return res, nil
	}

	// 5. Filter touches within lookback conversion window
	var validTouches []TouchPoint
	for _, touch := range sessionTouches {
		if conv.Timestamp.Sub(touch.Timestamp) <= t.config.ConversionWindow {
			validTouches = append(validTouches, touch)
		}
	}

	if len(validTouches) == 0 {
		res := &AttributionResult{
			ConversionID:     conv.ConversionID,
			SessionID:        conv.SessionID,
			Model:            model,
			Status:           StatusWindowExpired,
			ActionType:       conv.ActionType,
			RevenueUSD:       conv.RevenueUSD,
			PayoutBountyUSD:  conv.PayoutBountyUSD,
			ConversionTime:   conv.Timestamp,
			AttestationValid: attestationValid,
			Notes:            fmt.Sprintf("All %d touches occurred outside the %v lookback window", len(sessionTouches), t.config.ConversionWindow),
		}
		t.conversions[conv.ConversionID] = conv
		return res, nil
	}

	// 6. Calculate Attribution Credits
	credits := CalculateCredits(
		model,
		validTouches,
		conv.Timestamp,
		t.config.TimeDecayHalfLife,
		t.config.PositionWeights,
	)

	// 7. Calculate Payout Distribution
	split := [3]float64{
		t.config.RevenueSplit.PublisherShare,
		t.config.RevenueSplit.AgentHostShare,
		t.config.RevenueSplit.ProtocolShare,
	}
	pubPayouts, hostPayouts, protocolFee := DistributePayouts(conv.PayoutBountyUSD, credits, validTouches, split)

	// Calculate Time to Convert (from first valid touch to conversion)
	timeToConvert := conv.Timestamp.Sub(validTouches[0].Timestamp)

	result := &AttributionResult{
		ConversionID:     conv.ConversionID,
		SessionID:        conv.SessionID,
		Model:            model,
		Status:           StatusAttributed,
		ActionType:       conv.ActionType,
		RevenueUSD:       conv.RevenueUSD,
		PayoutBountyUSD:  conv.PayoutBountyUSD,
		ConversionTime:   conv.Timestamp,
		TimeToConvert:    timeToConvert,
		TouchesCount:     len(validTouches),
		TouchPoints:      validTouches,
		Credits:          credits,
		PublisherPayouts: pubPayouts,
		HostPayouts:      hostPayouts,
		ProtocolFeeUSD:   protocolFee,
		AttestationValid: attestationValid,
	}

	// Commit conversion and update metrics
	t.conversions[conv.ConversionID] = conv
	t.stats.TotalConversions++
	t.stats.TotalRevenueUSD += conv.RevenueUSD
	t.stats.TotalBountiesUSD += conv.PayoutBountyUSD
	for _, p := range pubPayouts {
		t.stats.TotalPubPayoutUSD += p
	}
	for _, h := range hostPayouts {
		t.stats.TotalHostPayoutUSD += h
	}
	t.stats.TotalProtocolUSD += protocolFee
	t.totalTimeMs += float64(timeToConvert.Milliseconds())
	t.stats.AvgTimeToConvertMs = t.totalTimeMs / float64(t.stats.TotalConversions)

	return result, nil
}

// GetSessionTouches returns all recorded impressions for a session.
func (t *Tracker) GetSessionTouches(sessionID string) []TouchPoint {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return append([]TouchPoint(nil), t.touches[sessionID]...)
}

// GetStats returns aggregate performance metrics.
func (t *Tracker) GetStats() TrackerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stats
}

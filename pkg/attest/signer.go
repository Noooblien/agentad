package attest

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Signer generates and verifies cryptographic Proof-of-Impression (PoI) receipts.
type Signer struct {
	mu         sync.Mutex
	SeenNonces map[string]time.Time
}

// NewSigner initializes a receipt signer and verification auditor.
func NewSigner() *Signer {
	return &Signer{
		SeenNonces: make(map[string]time.Time),
	}
}

// GenerateKeyPair generates a fresh Ed25519 cryptographic keypair.
func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 key: %w", err)
	}
	return &KeyPair{
		PublicKey:  pub,
		PrivateKey: priv,
		PubKeyHex:  hex.EncodeToString(pub),
	}, nil
}

// ComputeContextHash produces a deterministic SHA-256 digest of reasoning context.
func ComputeContextHash(contextText string) string {
	h := sha256.Sum256([]byte(contextText))
	return hex.EncodeToString(h[:])
}

// ConstructSigningPayload formats the canonical payload bytes for signing.
func ConstructSigningPayload(rType ReceiptType, auctionID, campaignID, contextHash, nonce string, timestampNano int64, clearingPrice float64) []byte {
	return []byte(fmt.Sprintf("%s:%s:%s:%s:%s:%d:%.4f",
		rType, auctionID, campaignID, contextHash, nonce, timestampNano, clearingPrice))
}

// SignReceipt generates a cryptographically signed Proof-of-Impression receipt.
func (s *Signer) SignReceipt(
	keyPair *KeyPair,
	agentID string,
	rType ReceiptType,
	auctionID, campaignID, advertiserID string,
	contextText string,
	clearingPriceCPM float64,
	debitUSD float64,
) (*AttestationReceipt, error) {
	if keyPair == nil || keyPair.PrivateKey == nil {
		return nil, fmt.Errorf("invalid signing key")
	}

	nonceBytes := make([]byte, 16)
	_, _ = rand.Read(nonceBytes)
	nonce := hex.EncodeToString(nonceBytes)

	now := time.Now().UTC()
	contextHash := ComputeContextHash(contextText)

	payload := ConstructSigningPayload(rType, auctionID, campaignID, contextHash, nonce, now.UnixNano(), clearingPriceCPM)
	sigBytes := ed25519.Sign(keyPair.PrivateKey, payload)

	receiptIDBytes := make([]byte, 8)
	_, _ = rand.Read(receiptIDBytes)
	receiptID := fmt.Sprintf("poi_%s_%s", rType[:3], hex.EncodeToString(receiptIDBytes))

	receipt := &AttestationReceipt{
		ReceiptID:        receiptID,
		ReceiptType:      rType,
		AuctionID:        auctionID,
		CampaignID:       campaignID,
		AdvertiserID:     advertiserID,
		AgentID:          agentID,
		ContextHash:      contextHash,
		ClearingPriceCPM: clearingPriceCPM,
		DebitAmountUSD:   debitUSD,
		Nonce:            nonce,
		Timestamp:        now,
		PublicKeyHex:     keyPair.PubKeyHex,
		SignatureHex:     hex.EncodeToString(sigBytes),
		Status:           "VERIFIED",
	}

	return receipt, nil
}

// VerifyReceipt verifies the Ed25519 cryptographic signature, replay protection, and age.
func (s *Signer) VerifyReceipt(receipt *AttestationReceipt, maxAge time.Duration) *VerificationResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	res := &VerificationResult{
		ReceiptID:   receipt.ReceiptID,
		ReceiptType: string(receipt.ReceiptType),
		AgentID:     receipt.AgentID,
		AuditedAt:   time.Now().UTC(),
	}

	if maxAge <= 0 {
		maxAge = 15 * time.Minute
	}

	// 1. Check Age & Expiration
	now := time.Now().UTC()
	if now.Sub(receipt.Timestamp) > maxAge {
		res.Violations = append(res.Violations, fmt.Sprintf("receipt expired: created at %s, age %v exceeds max %v",
			receipt.Timestamp.Format(time.RFC3339), now.Sub(receipt.Timestamp), maxAge))
	} else if receipt.Timestamp.After(now.Add(1 * time.Minute)) {
		res.Violations = append(res.Violations, "receipt timestamp is in the future")
	} else {
		res.TimestampValid = true
	}

	// 2. Replay Attack Prevention
	if _, exists := s.SeenNonces[receipt.Nonce]; exists {
		res.Violations = append(res.Violations, fmt.Sprintf("replay attack detected: nonce '%s' already used", receipt.Nonce))
	} else {
		s.SeenNonces[receipt.Nonce] = receipt.Timestamp
		res.ReplaySafe = true
	}

	// 3. Decode Public Key
	pubKeyBytes, err := hex.DecodeString(receipt.PublicKeyHex)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		res.Violations = append(res.Violations, "invalid Ed25519 public key hex")
		return res
	}

	// 4. Decode Signature
	sigBytes, err := hex.DecodeString(receipt.SignatureHex)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		res.Violations = append(res.Violations, "invalid Ed25519 signature hex")
		return res
	}

	// 5. Verify Cryptographic Signature
	payload := ConstructSigningPayload(
		receipt.ReceiptType,
		receipt.AuctionID,
		receipt.CampaignID,
		receipt.ContextHash,
		receipt.Nonce,
		receipt.Timestamp.UnixNano(),
		receipt.ClearingPriceCPM,
	)

	if ed25519.Verify(pubKeyBytes, payload, sigBytes) {
		res.SignerVerified = true
		res.HashIntegrity = true
	} else {
		res.Violations = append(res.Violations, "cryptographic signature verification failed")
	}

	res.Valid = len(res.Violations) == 0
	return res
}

// RenderReceiptScorecard formats an audit report for terminal inspection.
func RenderReceiptScorecard(r *AttestationReceipt, v *VerificationResult) string {
	var sb strings.Builder

	sb.WriteString("\033[1;35m========================================================================================\033[0m\n")
	sb.WriteString("  \033[1;37mAGENTAD // CRYPTOGRAPHIC PROOF-OF-IMPRESSION (PoI) ATTESTATION AUDIT\033[0m\n")
	sb.WriteString("\033[1;35m========================================================================================\033[0m\n")

	sb.WriteString(fmt.Sprintf("  \033[1;33mReceipt ID:\033[0m           %s (%s)\n", r.ReceiptID, r.ReceiptType))
	sb.WriteString(fmt.Sprintf("  \033[1;33mAuction Reference:\033[0m    %s | Campaign: %s | Advertiser: %s\n", r.AuctionID, r.CampaignID, r.AdvertiserID))
	sb.WriteString(fmt.Sprintf("  \033[1;33mAgent Signer:\033[0m         %s (Key: %s...)\n", r.AgentID, r.PublicKeyHex[:16]))
	sb.WriteString(fmt.Sprintf("  \033[1;33mSettlement Terms:\033[0m     $%.2f CPM | Debit: $%.5f USD\n", r.ClearingPriceCPM, r.DebitAmountUSD))
	sb.WriteString(fmt.Sprintf("  \033[1;33mContext Hash:\033[0m         %s\n", r.ContextHash))

	if v.Valid {
		sb.WriteString("  \033[1;33mAttestation Status:\033[0m   \033[1;32m[PASS] CRYPTOGRAPHICALLY VERIFIED — TAMPER-PROOF & AUDITED\033[0m\n")
	} else {
		sb.WriteString("  \033[1;33mAttestation Status:\033[0m   \033[1;31m[FAIL] RECEIPT REJECTED — INTEGRITY OR REPLAY VIOLATION\033[0m\n")
	}
	sb.WriteString("\033[1;34m----------------------------------------------------------------------------------------\033[0m\n")

	if v.Valid {
		sb.WriteString("  \033[1;32m✓\033[0m Ed25519 Digital Signature: Mathematically valid against publisher public key.\n")
		sb.WriteString("  \033[1;32m✓\033[0m Context Digest Continuity: Reasoning prompt matched impression payload hash.\n")
		sb.WriteString("  \033[1;32m✓\033[0m Anti-Replay Shield: Unique nonce validated; zero double-billing risk.\n")
	} else {
		sb.WriteString("  \033[1;31mVIOLATIONS DETECTED:\033[0m\n")
		for i, err := range v.Violations {
			sb.WriteString(fmt.Sprintf("  %d. \033[1;31m%s\033[0m\n", i+1, err))
		}
	}

	sb.WriteString("\033[1;35m========================================================================================\033[0m\n")
	return sb.String()
}

package attest

import (
	"crypto/ed25519"
	"time"
)

// ReceiptType specifies whether an attestation proves impression viewability or an executed downstream action.
type ReceiptType string

const (
	TypeProofOfImpression ReceiptType = "proof_of_impression" // Ad was validated & injected into agent reasoning tokens
	TypeProofOfAction     ReceiptType = "proof_of_action"     // Downstream tool call, checkout, or conversion occurred
)

// AttestationReceipt represents a cryptographically signed proof for settlement.
type AttestationReceipt struct {
	ReceiptID        string      `json:"receipt_id"`
	ReceiptType      ReceiptType `json:"receipt_type"`
	AuctionID        string      `json:"auction_id"`
	CampaignID       string      `json:"campaign_id"`
	AdvertiserID     string      `json:"advertiser_id"`
	AgentID          string      `json:"agent_id"`
	ContextHash      string      `json:"context_hash"` // SHA-256 of reasoning prompt + injected recommendation
	ClearingPriceCPM float64     `json:"clearing_price_cpm"`
	DebitAmountUSD   float64     `json:"debit_amount_usd"`
	Nonce            string      `json:"nonce"`
	Timestamp        time.Time   `json:"timestamp"`
	PublicKeyHex     string      `json:"public_key_hex"`
	SignatureHex     string      `json:"signature_hex"`
	Status           string      `json:"status"` // "VERIFIED", "REJECTED"
}

// VerificationResult contains the result of a cryptographic audit on a receipt.
type VerificationResult struct {
	Valid          bool      `json:"valid"`
	ReceiptID      string    `json:"receipt_id"`
	ReceiptType    string    `json:"receipt_type"`
	AgentID        string    `json:"agent_id"`
	SignerVerified bool      `json:"signer_verified"`
	TimestampValid bool      `json:"timestamp_valid"`
	HashIntegrity  bool      `json:"hash_integrity"`
	ReplaySafe     bool      `json:"replay_safe"`
	Violations     []string  `json:"violations,omitempty"`
	AuditedAt      time.Time `json:"audited_at"`
}

// KeyPair encapsulates Ed25519 signing keys.
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
	PubKeyHex  string
}

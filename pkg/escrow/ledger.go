package escrow

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
)

var (
	ErrInsufficientBalance = errors.New("insufficient escrow balance")
	ErrAccountNotFound     = errors.New("account not found")
	ErrReservationNotFound = errors.New("escrow reservation not found")
	ErrReservationExpired  = errors.New("escrow reservation expired")
	ErrInvalidPreimage     = errors.New("invalid L402 payment preimage")
	ErrInvalidSplit        = errors.New("revenue split shares must sum to 1.0 (100%)")
	ErrReplaySettlement    = errors.New("receipt has already been settled")
)

// Currency represents the accounting denomination.
type Currency string

const (
	CurrencyMicroUSD Currency = "uUSD" // 1 USD = 1,000,000 uUSD
	CurrencySat      Currency = "SAT"  // 1 satoshi
)

// RevenueSplit defines the fractional distribution of ad spend.
type RevenueSplit struct {
	PublisherShare float64 `json:"publisher_share"` // default 0.70 (70%)
	AgentHostShare float64 `json:"agent_host_share"` // default 0.20 (20%)
	ProtocolShare  float64 `json:"protocol_share"`   // default 0.10 (10%)
}

// DefaultRevenueSplit provides the canonical AAP 70/20/10 distribution.
func DefaultRevenueSplit() RevenueSplit {
	return RevenueSplit{
		PublisherShare: 0.70,
		AgentHostShare: 0.20,
		ProtocolShare:  0.10,
	}
}

// Account tracks deposits, reserved funds, and settled balances.
type Account struct {
	AccountID       string    `json:"account_id"`
	OwnerName       string    `json:"owner_name"`
	Role            string    `json:"role"` // advertiser, publisher, host, treasury
	Currency        Currency  `json:"currency"`
	AvailableAmount int64     `json:"available_amount"` // In micro-units (uUSD or sat)
	ReservedAmount  int64     `json:"reserved_amount"`
	TotalSettled    int64     `json:"total_settled"`
	LastUpdated     time.Time `json:"last_updated"`
}

// Reservation represents funds temporarily frozen during an in-flight auction/tool call.
type Reservation struct {
	ReservationID string    `json:"reservation_id"`
	AuctionID     string    `json:"auction_id"`
	AdvertiserID  string    `json:"advertiser_id"`
	PublisherID   string    `json:"publisher_id"`
	HostID        string    `json:"host_id"`
	Amount        int64     `json:"amount"` // micro-units
	Currency      Currency  `json:"currency"`
	ExpiresAt     time.Time `json:"expires_at"`
	Settled       bool      `json:"settled"`
	CreatedAt     time.Time `json:"created_at"`
}

// SettlementRecord records an immutable cryptographic disbursement.
type SettlementRecord struct {
	SettlementID     string       `json:"settlement_id"`
	AuctionID        string       `json:"auction_id"`
	ReceiptHash      string       `json:"receipt_hash"`
	AdvertiserID     string       `json:"advertiser_id"`
	PublisherID      string       `json:"publisher_id"`
	HostID           string       `json:"host_id"`
	TotalAmount      int64        `json:"total_amount"`
	PublisherPayout  int64        `json:"publisher_payout"`
	HostPayout       int64        `json:"host_payout"`
	ProtocolFee      int64        `json:"protocol_fee"`
	Currency         Currency     `json:"currency"`
	Split            RevenueSplit `json:"split"`
	Timestamp        time.Time    `json:"timestamp"`
	PreviousRecordID string       `json:"prev_record_id"`
	ChainHash        string       `json:"chain_hash"`
}

// L402Challenge represents an HTTP 402 Payment Required token challenge.
type L402Challenge struct {
	Macaroon    string    `json:"macaroon"`
	PaymentHash string    `json:"payment_hash"`
	Amount      int64     `json:"amount"`
	Currency    Currency  `json:"currency"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Ledger is the thread-safe micro-settlement engine for AAP.
type Ledger struct {
	mu           sync.RWMutex
	accounts     map[string]*Account
	reservations map[string]*Reservation
	settlements  []*SettlementRecord
	settledNonces map[string]bool
	treasuryID   string
	defaultSplit RevenueSplit
	lastChainHash string
}

// NewLedger initializes a fresh escrow clearing ledger.
func NewLedger(treasuryID string, split RevenueSplit) (*Ledger, error) {
	if split.PublisherShare+split.AgentHostShare+split.ProtocolShare != 1.0 {
		// allow slight floating point variance
		diff := (split.PublisherShare + split.AgentHostShare + split.ProtocolShare) - 1.0
		if diff < -0.0001 || diff > 0.0001 {
			return nil, ErrInvalidSplit
		}
	}

	l := &Ledger{
		accounts:      make(map[string]*Account),
		reservations:  make(map[string]*Reservation),
		settlements:   make([]*SettlementRecord, 0),
		settledNonces: make(map[string]bool),
		treasuryID:    treasuryID,
		defaultSplit:  split,
		lastChainHash: "genesis_aap_escrow_v1",
	}

	// Initialize protocol treasury account
	l.accounts[treasuryID] = &Account{
		AccountID:   treasuryID,
		OwnerName:   "AAP Protocol Treasury",
		Role:        "treasury",
		Currency:    CurrencyMicroUSD,
		LastUpdated: time.Now().UTC(),
	}

	return l, nil
}

// RegisterAccount creates or registers a new wallet/escrow account.
func (l *Ledger) RegisterAccount(id, ownerName, role string, currency Currency) *Account {
	l.mu.Lock()
	defer l.mu.Unlock()

	if acc, exists := l.accounts[id]; exists {
		return acc
	}

	acc := &Account{
		AccountID:   id,
		OwnerName:   ownerName,
		Role:        role,
		Currency:    currency,
		LastUpdated: time.Now().UTC(),
	}
	l.accounts[id] = acc
	return acc
}

// Deposit credits an account's available balance.
func (l *Ledger) Deposit(accountID string, amount int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	acc, exists := l.accounts[accountID]
	if !exists {
		return ErrAccountNotFound
	}
	if amount <= 0 {
		return fmt.Errorf("deposit amount must be positive")
	}

	acc.AvailableAmount += amount
	acc.LastUpdated = time.Now().UTC()
	return nil
}

// USDToMicro converts a standard USD dollar amount (e.g. $0.0035) to micro-USD (3500 uUSD).
func USDToMicro(usd float64) int64 {
	return int64(usd * 1_000_000.0)
}

// MicroToUSD converts micro-USD to floating point USD.
func MicroToUSD(micro int64) float64 {
	return float64(micro) / 1_000_000.0
}

// ReserveFunds freezes funds from an advertiser account pending auction attestation.
func (l *Ledger) ReserveFunds(auctionID, advertiserID, publisherID, hostID string, amount int64, ttl time.Duration) (*Reservation, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	acc, exists := l.accounts[advertiserID]
	if !exists {
		return nil, ErrAccountNotFound
	}

	if acc.AvailableAmount < amount {
		return nil, ErrInsufficientBalance
	}

	acc.AvailableAmount -= amount
	acc.ReservedAmount += amount
	acc.LastUpdated = time.Now().UTC()

	resIDBytes := make([]byte, 8)
	_, _ = rand.Read(resIDBytes)
	resID := "res_" + hex.EncodeToString(resIDBytes)

	res := &Reservation{
		ReservationID: resID,
		AuctionID:     auctionID,
		AdvertiserID:  advertiserID,
		PublisherID:   publisherID,
		HostID:        hostID,
		Amount:        amount,
		Currency:      acc.Currency,
		ExpiresAt:     time.Now().UTC().Add(ttl),
		Settled:       false,
		CreatedAt:     time.Now().UTC(),
	}

	l.reservations[resID] = res
	return res, nil
}

// SettleAttestation cryptographically settles reserved funds using a verified Proof-of-Impression receipt.
func (l *Ledger) SettleAttestation(receipt *attest.AttestationReceipt, resID string, verifier *attest.Signer) (*SettlementRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Verify attestation receipt validity
	verResult := verifier.VerifyReceipt(receipt, 30*time.Minute)
	if !verResult.Valid {
		return nil, fmt.Errorf("attestation verification failed: %v", verResult.Violations)
	}

	// Prevent replay of already-settled receipt nonces
	if l.settledNonces[receipt.Nonce] {
		return nil, ErrReplaySettlement
	}

	res, exists := l.reservations[resID]
	if !exists {
		return nil, ErrReservationNotFound
	}
	if res.Settled {
		return nil, fmt.Errorf("reservation already settled")
	}
	if time.Now().UTC().After(res.ExpiresAt) {
		return nil, ErrReservationExpired
	}

	advAcc, exists := l.accounts[res.AdvertiserID]
	if !exists {
		return nil, fmt.Errorf("advertiser account %s not found", res.AdvertiserID)
	}

	pubAcc, exists := l.accounts[res.PublisherID]
	if !exists {
		pubAcc = &Account{AccountID: res.PublisherID, Role: "publisher", Currency: res.Currency, LastUpdated: time.Now().UTC()}
		l.accounts[res.PublisherID] = pubAcc
	}

	hostAcc, exists := l.accounts[res.HostID]
	if !exists {
		hostAcc = &Account{AccountID: res.HostID, Role: "host", Currency: res.Currency, LastUpdated: time.Now().UTC()}
		l.accounts[res.HostID] = hostAcc
	}

	treasuryAcc := l.accounts[l.treasuryID]

	// Calculate payouts based on revenue split
	totalAmount := res.Amount
	pubPayout := int64(float64(totalAmount) * l.defaultSplit.PublisherShare)
	hostPayout := int64(float64(totalAmount) * l.defaultSplit.AgentHostShare)
	protocolFee := totalAmount - pubPayout - hostPayout

	// Update balances
	advAcc.ReservedAmount -= totalAmount
	advAcc.TotalSettled += totalAmount
	advAcc.LastUpdated = time.Now().UTC()

	pubAcc.AvailableAmount += pubPayout
	pubAcc.TotalSettled += pubPayout
	pubAcc.LastUpdated = time.Now().UTC()

	hostAcc.AvailableAmount += hostPayout
	hostAcc.TotalSettled += hostPayout
	hostAcc.LastUpdated = time.Now().UTC()

	treasuryAcc.AvailableAmount += protocolFee
	treasuryAcc.TotalSettled += protocolFee
	treasuryAcc.LastUpdated = time.Now().UTC()

	res.Settled = true
	l.settledNonces[receipt.Nonce] = true

	// Compute immutable audit chain hash
	receiptHash := sha256.Sum256([]byte(receipt.SignatureHex + receipt.Nonce + receipt.AuctionID))
	receiptHashHex := hex.EncodeToString(receiptHash[:])

	recordBytes := fmt.Sprintf("%s:%s:%d:%s", l.lastChainHash, receiptHashHex, totalAmount, time.Now().UTC().Format(time.RFC3339Nano))
	chainHash := sha256.Sum256([]byte(recordBytes))
	chainHashHex := hex.EncodeToString(chainHash[:])

	recIDBytes := make([]byte, 8)
	_, _ = rand.Read(recIDBytes)
	recID := "set_" + hex.EncodeToString(recIDBytes)

	record := &SettlementRecord{
		SettlementID:     recID,
		AuctionID:        res.AuctionID,
		ReceiptHash:      receiptHashHex,
		AdvertiserID:     res.AdvertiserID,
		PublisherID:      res.PublisherID,
		HostID:           res.HostID,
		TotalAmount:      totalAmount,
		PublisherPayout:  pubPayout,
		HostPayout:       hostPayout,
		ProtocolFee:      protocolFee,
		Currency:         res.Currency,
		Split:            l.defaultSplit,
		Timestamp:        time.Now().UTC(),
		PreviousRecordID: l.lastChainHash,
		ChainHash:        chainHashHex,
	}

	l.lastChainHash = chainHashHex
	l.settlements = append(l.settlements, record)

	return record, nil
}

// RefundExpiredReservation unfreezes funds back to the advertiser if the reservation expired without proof.
func (l *Ledger) RefundExpiredReservation(resID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	res, exists := l.reservations[resID]
	if !exists {
		return ErrReservationNotFound
	}
	if res.Settled {
		return fmt.Errorf("cannot refund already settled reservation")
	}

	advAcc, exists := l.accounts[res.AdvertiserID]
	if !exists {
		return ErrAccountNotFound
	}

	advAcc.ReservedAmount -= res.Amount
	advAcc.AvailableAmount += res.Amount
	advAcc.LastUpdated = time.Now().UTC()

	delete(l.reservations, resID)
	return nil
}

// GetAccount returns a copy of an account's current state.
func (l *Ledger) GetAccount(id string) (*Account, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	acc, exists := l.accounts[id]
	if !exists {
		return nil, ErrAccountNotFound
	}
	cpy := *acc
	return &cpy, nil
}

// GetSettlements returns all settled records.
func (l *Ledger) GetSettlements() []*SettlementRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()

	out := make([]*SettlementRecord, len(l.settlements))
	copy(out, l.settlements)
	return out
}

// CreateL402Challenge generates an HTTP 402 challenge for Lightning or micro-settlements.
func CreateL402Challenge(amount int64, currency Currency, ttl time.Duration) (*L402Challenge, string, error) {
	preimage := make([]byte, 32)
	if _, err := rand.Read(preimage); err != nil {
		return nil, "", err
	}
	paymentHash := sha256.Sum256(preimage)
	paymentHashHex := hex.EncodeToString(paymentHash[:])
	preimageHex := hex.EncodeToString(preimage)

	macBytes := make([]byte, 16)
	_, _ = rand.Read(macBytes)
	macaroon := hex.EncodeToString(macBytes)

	challenge := &L402Challenge{
		Macaroon:    macaroon,
		PaymentHash: paymentHashHex,
		Amount:      amount,
		Currency:    currency,
		ExpiresAt:   time.Now().UTC().Add(ttl),
	}

	return challenge, preimageHex, nil
}

// VerifyL402Preimage verifies if a presented preimage solves the payment hash.
func VerifyL402Preimage(paymentHashHex, preimageHex string) bool {
	preimageBytes, err := hex.DecodeString(preimageHex)
	if err != nil || len(preimageBytes) != 32 {
		return false
	}
	h := sha256.Sum256(preimageBytes)
	return hex.EncodeToString(h[:]) == paymentHashHex
}

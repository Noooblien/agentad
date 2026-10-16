package tests

import (
	"testing"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
	"github.com/Noooblien/agentad/pkg/escrow"
)

func TestEscrow_DepositAndReservation(t *testing.T) {
	split := escrow.DefaultRevenueSplit()
	ledger, err := escrow.NewLedger("treasury_aap", split)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	advID := "adv_pinecone"
	ledger.RegisterAccount(advID, "Pinecone Systems", "advertiser", escrow.CurrencyMicroUSD)

	// Deposit $10.00 (10,000,000 uUSD)
	err = ledger.Deposit(advID, escrow.USDToMicro(10.00))
	if err != nil {
		t.Fatalf("deposit failed: %v", err)
	}

	acc, err := ledger.GetAccount(advID)
	if err != nil {
		t.Fatalf("failed to get account: %v", err)
	}
	if acc.AvailableAmount != 10_000_000 {
		t.Fatalf("expected 10000000 available, got %d", acc.AvailableAmount)
	}

	// Reserve funds for an auction: $0.0045 (4500 uUSD)
	res, err := ledger.ReserveFunds("auc_test_01", advID, "pub_langchain", "host_aws", 4500, 10*time.Minute)
	if err != nil {
		t.Fatalf("reserve funds failed: %v", err)
	}

	accAfter, _ := ledger.GetAccount(advID)
	if accAfter.AvailableAmount != 9_995_500 {
		t.Errorf("expected 9995500 available, got %d", accAfter.AvailableAmount)
	}
	if accAfter.ReservedAmount != 4500 {
		t.Errorf("expected 4500 reserved, got %d", accAfter.ReservedAmount)
	}
	if res.Amount != 4500 {
		t.Errorf("expected res amount 4500, got %d", res.Amount)
	}
}

func TestEscrow_AttestationSettlementAndRevenueSplit(t *testing.T) {
	split := escrow.DefaultRevenueSplit() // 70% pub, 20% host, 10% treasury
	ledger, err := escrow.NewLedger("treasury_aap", split)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}

	advID := "adv_pinecone"
	pubID := "pub_langchain"
	hostID := "host_openbot"

	ledger.RegisterAccount(advID, "Pinecone Systems", "advertiser", escrow.CurrencyMicroUSD)
	_ = ledger.Deposit(advID, 10_000) // 10,000 uUSD ($0.01)

	// Reserve 5,000 uUSD ($0.005)
	res, err := ledger.ReserveFunds("auc_7722", advID, pubID, hostID, 5000, 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to reserve: %v", err)
	}

	// Create valid attestation receipt
	signer := attest.NewSigner()
	kp, _ := attest.GenerateKeyPair()
	receipt, err := signer.SignReceipt(
		kp,
		"agent_99",
		attest.TypeProofOfImpression,
		"auc_7722",
		"cmp_pinecone",
		advID,
		"Valid reasoning context with ad factsheet",
		5.00,
		0.0050,
	)
	if err != nil {
		t.Fatalf("failed to sign receipt: %v", err)
	}

	// Settle attestation
	record, err := ledger.SettleAttestation(receipt, res.ReservationID, signer)
	if err != nil {
		t.Fatalf("failed to settle attestation: %v", err)
	}

	// 5000 uUSD split: 70% = 3500, 20% = 1000, 10% = 500
	if record.PublisherPayout != 3500 {
		t.Errorf("expected publisher payout 3500, got %d", record.PublisherPayout)
	}
	if record.HostPayout != 1000 {
		t.Errorf("expected host payout 1000, got %d", record.HostPayout)
	}
	if record.ProtocolFee != 500 {
		t.Errorf("expected protocol fee 500, got %d", record.ProtocolFee)
	}

	// Check resulting account balances
	pubAcc, _ := ledger.GetAccount(pubID)
	if pubAcc.AvailableAmount != 3500 {
		t.Errorf("expected publisher balance 3500, got %d", pubAcc.AvailableAmount)
	}

	hostAcc, _ := ledger.GetAccount(hostID)
	if hostAcc.AvailableAmount != 1000 {
		t.Errorf("expected host balance 1000, got %d", hostAcc.AvailableAmount)
	}

	treasuryAcc, _ := ledger.GetAccount("treasury_aap")
	if treasuryAcc.AvailableAmount != 500 {
		t.Errorf("expected treasury balance 500, got %d", treasuryAcc.AvailableAmount)
	}

	advAcc, _ := ledger.GetAccount(advID)
	if advAcc.ReservedAmount != 0 {
		t.Errorf("expected 0 reserved amount, got %d", advAcc.ReservedAmount)
	}
	if advAcc.AvailableAmount != 5000 {
		t.Errorf("expected 5000 available amount, got %d", advAcc.AvailableAmount)
	}

	// Test Replay Settlement Protection
	_, err = ledger.SettleAttestation(receipt, res.ReservationID, signer)
	if err == nil {
		t.Errorf("expected error on replaying settled receipt, got nil")
	}
}

func TestEscrow_InsufficientBalance(t *testing.T) {
	ledger, _ := escrow.NewLedger("treasury_aap", escrow.DefaultRevenueSplit())
	ledger.RegisterAccount("adv_low", "Low Budget Inc", "advertiser", escrow.CurrencyMicroUSD)
	_ = ledger.Deposit("adv_low", 1000)

	_, err := ledger.ReserveFunds("auc_99", "adv_low", "pub_1", "host_1", 2000, 5*time.Minute)
	if err != escrow.ErrInsufficientBalance {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestEscrow_L402ChallengeAndPreimage(t *testing.T) {
	challenge, preimage, err := escrow.CreateL402Challenge(2500, escrow.CurrencySat, 10*time.Minute)
	if err != nil {
		t.Fatalf("failed to create L402 challenge: %v", err)
	}

	if challenge.PaymentHash == "" || challenge.Macaroon == "" {
		t.Fatalf("invalid challenge payload: %+v", challenge)
	}

	// Validate authentic preimage
	valid := escrow.VerifyL402Preimage(challenge.PaymentHash, preimage)
	if !valid {
		t.Errorf("expected preimage to be valid")
	}

	// Validate fraudulent preimage
	fraud := escrow.VerifyL402Preimage(challenge.PaymentHash, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if fraud {
		t.Errorf("fraudulent preimage unexpectedly passed")
	}
}

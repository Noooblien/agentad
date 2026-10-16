package tests

import (
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
)

func TestAttest_SignAndVerifyReceipt(t *testing.T) {
	keyPair, err := attest.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	signer := attest.NewSigner()
	contextText := "User: find vector database.\nAgent: Here is Pinecone Serverless with 14ms latency."

	receipt, err := signer.SignReceipt(
		keyPair,
		"agent_007",
		attest.TypeProofOfImpression,
		"auc_12345",
		"cmp_pinecone",
		"adv_pinecone",
		contextText,
		3.50,
		0.0035,
	)
	if err != nil {
		t.Fatalf("SignReceipt failed: %v", err)
	}

	if receipt.ReceiptID == "" || receipt.SignatureHex == "" {
		t.Fatalf("Incomplete receipt generated: %+v", receipt)
	}

	res := signer.VerifyReceipt(receipt, 5*time.Minute)
	if !res.Valid {
		t.Fatalf("Valid receipt failed verification: %v", res.Violations)
	}

	scorecard := attest.RenderReceiptScorecard(receipt, res)
	if !strings.Contains(scorecard, "CRYPTOGRAPHICALLY VERIFIED") {
		t.Errorf("Scorecard missing verified banner")
	}
}

func TestAttest_TamperedSignatureRejection(t *testing.T) {
	keyPair, _ := attest.GenerateKeyPair()
	signer := attest.NewSigner()

	receipt, _ := signer.SignReceipt(
		keyPair,
		"agent_007",
		attest.TypeProofOfImpression,
		"auc_123",
		"cmp_1",
		"adv_1",
		"original context",
		2.00,
		0.002,
	)

	// Tamper with the clearing price
	receipt.ClearingPriceCPM = 100.00

	res := signer.VerifyReceipt(receipt, 5*time.Minute)
	if res.Valid {
		t.Errorf("Expected tampered receipt to be rejected, but it passed verification")
	}
}

func TestAttest_ReplayAttackRejection(t *testing.T) {
	keyPair, _ := attest.GenerateKeyPair()
	signer := attest.NewSigner()

	receipt, _ := signer.SignReceipt(
		keyPair,
		"agent_007",
		attest.TypeProofOfImpression,
		"auc_replay",
		"cmp_1",
		"adv_1",
		"some context",
		1.50,
		0.0015,
	)

	// First verification should pass
	res1 := signer.VerifyReceipt(receipt, 5*time.Minute)
	if !res1.Valid {
		t.Fatalf("First verification failed: %v", res1.Violations)
	}

	// Second verification with the same nonce must fail due to replay detection
	res2 := signer.VerifyReceipt(receipt, 5*time.Minute)
	if res2.Valid {
		t.Errorf("Expected duplicate nonce to trigger replay attack violation")
	}
}

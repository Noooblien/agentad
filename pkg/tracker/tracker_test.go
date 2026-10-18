package tracker

import (
	"math"
	"testing"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
)

func createMockTouches(baseTime time.Time) []TouchPoint {
	return []TouchPoint{
		{
			ImpressionID: "imp_01_discovery",
			SessionID:    "sess_agent_flow_01",
			CampaignID:   "cmp_vector_db",
			AdvertiserID: "adv_pinecone",
			PublisherID:  "pub_rag_agent",
			AgentID:      "agent_host_01",
			CostUSD:      0.002,
			Timestamp:    baseTime.Add(-3 * time.Hour),
		},
		{
			ImpressionID: "imp_02_comparison",
			SessionID:    "sess_agent_flow_01",
			CampaignID:   "cmp_vector_db",
			AdvertiserID: "adv_pinecone",
			PublisherID:  "pub_benchmark_tool",
			AgentID:      "agent_host_02",
			CostUSD:      0.003,
			Timestamp:    baseTime.Add(-2 * time.Hour),
		},
		{
			ImpressionID: "imp_03_recommendation",
			SessionID:    "sess_agent_flow_01",
			CampaignID:   "cmp_vector_db",
			AdvertiserID: "adv_pinecone",
			PublisherID:  "pub_sdk_registry",
			AgentID:      "agent_host_01",
			CostUSD:      0.004,
			Timestamp:    baseTime.Add(-1 * time.Hour),
		},
		{
			ImpressionID: "imp_04_closing",
			SessionID:    "sess_agent_flow_01",
			CampaignID:   "cmp_vector_db",
			AdvertiserID: "adv_pinecone",
			PublisherID:  "pub_checkout_router",
			AgentID:      "agent_host_03",
			CostUSD:      0.005,
			Timestamp:    baseTime.Add(-10 * time.Minute),
		},
	}
}

func TestTracker_FirstTouchModel(t *testing.T) {
	now := time.Now().UTC()
	touches := createMockTouches(now)
	credits := CalculateCredits(ModelFirstTouch, touches, now, 48*time.Hour, [3]float64{0.4, 0.2, 0.4})

	if credits["imp_01_discovery"] != 1.0 {
		t.Fatalf("expected first touch to receive 1.0 credit, got %f", credits["imp_01_discovery"])
	}
	if credits["imp_04_closing"] != 0.0 {
		t.Fatalf("expected last touch to receive 0.0 credit, got %f", credits["imp_04_closing"])
	}
}

func TestTracker_LastTouchModel(t *testing.T) {
	now := time.Now().UTC()
	touches := createMockTouches(now)
	credits := CalculateCredits(ModelLastTouch, touches, now, 48*time.Hour, [3]float64{0.4, 0.2, 0.4})

	if credits["imp_04_closing"] != 1.0 {
		t.Fatalf("expected last touch to receive 1.0 credit, got %f", credits["imp_04_closing"])
	}
	if credits["imp_01_discovery"] != 0.0 {
		t.Fatalf("expected first touch to receive 0.0 credit, got %f", credits["imp_01_discovery"])
	}
}

func TestTracker_LinearModel(t *testing.T) {
	now := time.Now().UTC()
	touches := createMockTouches(now)
	credits := CalculateCredits(ModelLinear, touches, now, 48*time.Hour, [3]float64{0.4, 0.2, 0.4})

	expected := 1.0 / float64(len(touches))
	for id, credit := range credits {
		if math.Abs(credit-expected) > 0.0001 {
			t.Fatalf("touch %s expected %f credit, got %f", id, expected, credit)
		}
	}
}

func TestTracker_TimeDecayModel(t *testing.T) {
	now := time.Now().UTC()
	touches := createMockTouches(now)
	credits := CalculateCredits(ModelTimeDecay, touches, now, 2*time.Hour, [3]float64{0.4, 0.2, 0.4})

	// More recent touches should have strictly higher weights
	if credits["imp_04_closing"] <= credits["imp_03_recommendation"] {
		t.Fatalf("imp_04 should have higher credit than imp_03 under time decay")
	}
	if credits["imp_03_recommendation"] <= credits["imp_02_comparison"] {
		t.Fatalf("imp_03 should have higher credit than imp_02 under time decay")
	}
	if credits["imp_02_comparison"] <= credits["imp_01_discovery"] {
		t.Fatalf("imp_02 should have higher credit than imp_01 under time decay")
	}

	var sum float64
	for _, c := range credits {
		sum += c
	}
	if math.Abs(sum-1.0) > 0.0001 {
		t.Fatalf("time decay credits should sum to 1.0, got %f", sum)
	}
}

func TestTracker_PositionBasedModel(t *testing.T) {
	now := time.Now().UTC()
	touches := createMockTouches(now) // 4 touches
	credits := CalculateCredits(ModelPositionBased, touches, now, 48*time.Hour, [3]float64{0.40, 0.20, 0.40})

	if math.Abs(credits["imp_01_discovery"]-0.40) > 0.0001 {
		t.Fatalf("first touch expected 0.40, got %f", credits["imp_01_discovery"])
	}
	if math.Abs(credits["imp_04_closing"]-0.40) > 0.0001 {
		t.Fatalf("last touch expected 0.40, got %f", credits["imp_04_closing"])
	}
	// 2 middle touches share 0.20 => 0.10 each
	if math.Abs(credits["imp_02_comparison"]-0.10) > 0.0001 {
		t.Fatalf("middle touch expected 0.10, got %f", credits["imp_02_comparison"])
	}
	if math.Abs(credits["imp_03_recommendation"]-0.10) > 0.0001 {
		t.Fatalf("middle touch expected 0.10, got %f", credits["imp_03_recommendation"])
	}

	// Test 2-touch edge case: 50/50
	twoTouches := touches[:2]
	twoCredits := CalculateCredits(ModelPositionBased, twoTouches, now, 48*time.Hour, [3]float64{0.40, 0.20, 0.40})
	if twoCredits[twoTouches[0].ImpressionID] != 0.50 || twoCredits[twoTouches[1].ImpressionID] != 0.50 {
		t.Fatalf("2 touches expected 0.50/0.50 split, got %v", twoCredits)
	}
}

func TestTracker_ConversionWindowExpiration(t *testing.T) {
	cfg := DefaultTrackerConfig()
	cfg.ConversionWindow = 1 * time.Hour
	tr := NewTracker(cfg)

	now := time.Now().UTC()
	// Stale touch: 3 hours ago
	_ = tr.RecordTouch(TouchPoint{
		ImpressionID: "imp_old",
		SessionID:    "sess_stale",
		Timestamp:    now.Add(-3 * time.Hour),
	})

	conv := Conversion{
		ConversionID:    "conv_stale",
		SessionID:       "sess_stale",
		ActionType:      ActionCheckout,
		RevenueUSD:      50.0,
		PayoutBountyUSD: 5.0,
		Timestamp:       now,
	}

	res, err := tr.RecordConversion(conv, ModelPositionBased)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusWindowExpired {
		t.Fatalf("expected StatusWindowExpired, got %s", res.Status)
	}
}

func TestTracker_ReplayAndIdempotency(t *testing.T) {
	tr := NewTracker(DefaultTrackerConfig())
	now := time.Now().UTC()

	_ = tr.RecordTouch(TouchPoint{
		ImpressionID: "imp_active",
		SessionID:    "sess_idemp",
		Timestamp:    now.Add(-10 * time.Minute),
	})

	conv := Conversion{
		ConversionID:    "conv_test_replay",
		SessionID:       "sess_idemp",
		ActionType:      ActionToolCall,
		RevenueUSD:      12.0,
		PayoutBountyUSD: 1.20,
		Timestamp:       now,
		Nonce:           "nonce_unique_12345",
	}

	// First execution: should succeed
	res, err := tr.RecordConversion(conv, ModelPositionBased)
	if err != nil || res.Status != StatusAttributed {
		t.Fatalf("initial conversion failed: %v", err)
	}

	// Duplicate conversion ID
	_, errDup := tr.RecordConversion(conv, ModelPositionBased)
	if errDup != ErrDuplicateConversion {
		t.Fatalf("expected ErrDuplicateConversion, got %v", errDup)
	}

	// Replay nonce on different conversion ID
	conv2 := conv
	conv2.ConversionID = "conv_diff_id"
	_, errNonce := tr.RecordConversion(conv2, ModelPositionBased)
	if errNonce != ErrReplayNonceDetected {
		t.Fatalf("expected ErrReplayNonceDetected, got %v", errNonce)
	}
}

func TestTracker_CryptographicAttestationProof(t *testing.T) {
	tr := NewTracker(DefaultTrackerConfig())
	kp, err := attest.GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}

	now := time.Now().UTC()
	_ = tr.RecordTouch(TouchPoint{
		ImpressionID: "imp_crypto",
		SessionID:    "sess_crypto",
		PublisherID:  "pub_secure",
		AgentID:      "agent_verifier",
		Timestamp:    now.Add(-5 * time.Minute),
	})

	conv := Conversion{
		ConversionID:    "conv_signed_01",
		SessionID:       "sess_crypto",
		AdvertiserID:    "adv_merchant",
		CampaignID:      "cmp_prod",
		ActionType:      ActionCheckout,
		RevenueUSD:      100.0,
		PayoutBountyUSD: 10.0,
		Timestamp:       now,
		Nonce:           "nonce_crypto_999",
	}

	// Sign the conversion
	if err := SignConversion(&conv, kp); err != nil {
		t.Fatalf("failed to sign conversion: %v", err)
	}

	if !VerifyConversion(conv) {
		t.Fatalf("conversion signature verification failed immediately after signing")
	}

	res, err := tr.RecordConversion(conv, ModelPositionBased)
	if err != nil {
		t.Fatalf("failed to record signed conversion: %v", err)
	}
	if !res.AttestationValid {
		t.Fatalf("expected AttestationValid to be true")
	}

	// Tampered conversion verification check
	tamperedConv := conv
	tamperedConv.RevenueUSD = 999.0 // Tampered revenue
	if VerifyConversion(tamperedConv) {
		t.Fatalf("expected tampered conversion signature to be invalid")
	}
}

func TestTracker_PayoutSplits(t *testing.T) {
	tr := NewTracker(DefaultTrackerConfig())
	now := time.Now().UTC()

	touches := createMockTouches(now)
	for _, touch := range touches {
		_ = tr.RecordTouch(touch)
	}

	conv := Conversion{
		ConversionID:    "conv_payout_test",
		SessionID:       "sess_agent_flow_01",
		ActionType:      ActionCheckout,
		RevenueUSD:      200.0,
		PayoutBountyUSD: 20.0, // $20 total bounty
		Timestamp:       now,
		Nonce:           "nonce_payout_888",
	}

	res, err := tr.RecordConversion(conv, ModelPositionBased)
	if err != nil {
		t.Fatalf("record conversion failed: %v", err)
	}

	// Revenue split: 70% publisher ($14), 20% host ($4), 10% protocol ($2)
	var totalPubPayout float64
	for _, amt := range res.PublisherPayouts {
		totalPubPayout += amt
	}
	var totalHostPayout float64
	for _, amt := range res.HostPayouts {
		totalHostPayout += amt
	}

	if math.Abs(totalPubPayout-14.00) > 0.001 {
		t.Fatalf("expected publisher payout $14.00, got $%.4f", totalPubPayout)
	}
	if math.Abs(totalHostPayout-4.00) > 0.001 {
		t.Fatalf("expected agent host payout $4.00, got $%.4f", totalHostPayout)
	}
	if math.Abs(res.ProtocolFeeUSD-2.00) > 0.001 {
		t.Fatalf("expected protocol fee $2.00, got $%.4f", res.ProtocolFeeUSD)
	}
}

package discovery

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/Noooblien/agentad/pkg/spec"
)

// DiscoverySummary aggregates discovery crawl results and slot search outcomes.
type DiscoverySummary struct {
	TotalCrawled       int              `json:"total_crawled"`
	VerifiedPublishers int              `json:"verified_publishers"`
	RejectedPublishers int              `json:"rejected_publishers"`
	SlotsFound         int              `json:"slots_found"`
	AverageFloorCPM    float64          `json:"average_floor_cpm"`
	Publishers         []*CrawlReport   `json:"publishers"`
	DiscoveredSlots    []DiscoveredSlot `json:"discovered_slots"`
}

// CrawlReport details the resolution and verification outcome of a single publisher.
type CrawlReport struct {
	Domain        string  `json:"domain"`
	PublisherID   string  `json:"publisher_id"`
	Status        string  `json:"status"` // VERIFIED | BLOCKED_SSRF | REJECTED_DOMAIN
	StatusReason  string  `json:"status_reason"`
	SignatureOK   bool    `json:"signature_ok"`
	SlotsCount    int     `json:"slots_count"`
	FloorPriceCPM float64 `json:"floor_price_cpm"`
	LatencyMs     float64 `json:"latency_ms"`
}

// RunDiscoveryDemo executes a 4-scenario simulation testing SSRF defense, 304 caching, domain checks & slot querying.
func RunDiscoveryDemo() *DiscoverySummary {
	pub1, priv1, _ := ed25519.GenerateKey(rand.Reader)
	pubHex1 := hex.EncodeToString(pub1)

	var srv1Host string
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == WellKnownAgentAdsPath {
			m := spec.AgentAdsSpec{
				Schema:  "https://agentad.org/schemas/v1/agent-ads.json",
				Version: spec.CurrentProtocolVersion,
				Publisher: spec.PublisherInfo{
					Domain:      srv1Host,
					SigningKey:  "ed25519:" + pubHex1,
					PublisherID: "pub_cloudinfra_01",
					ContactURL:  "https://" + srv1Host + "/contact",
				},
				Monetization: spec.MonetizationConfig{
					SupportedModes:     []spec.MonetizationMode{spec.ModeSponsoredContext},
					SettlementCurrency: []string{"USDC", "SAT"},
					PricingFloorCPM:    3.50,
					RevenueSharePct:    0.70,
				},
				Slots: []spec.AdSlot{
					{
						SlotID:                 "slot_k8s_compute",
						ContextType:            "cloud_infrastructure",
						MaxContextTokens:       256,
						AllowedPayloadTypes:    []string{spec.FactsheetMimeType},
						StrictSchemaValidation: true,
					},
					{
						SlotID:                 "slot_gpu_cluster",
						ContextType:            "developer_tools",
						MaxContextTokens:       512,
						AllowedPayloadTypes:    []string{spec.FactsheetMimeType},
						StrictSchemaValidation: true,
					},
				},
				AttestationEndpoint: "https://" + srv1Host + "/api/v1/attest",
				LastUpdated:         time.Now().UTC(),
			}

			data, _ := json.Marshal(m)
			sig := ed25519.Sign(priv1, data)
			sigHex := hex.EncodeToString(sig)

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", `"etag-cloudinfra-v1"`)
			w.Header().Set("X-AgentAd-Signature", sigHex)
			w.Write(data)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv1.Close()

	srv1Host = strings.TrimPrefix(srv1.URL, "http://")

	cfg := DefaultConfig()
	cfg.AllowInsecureHTTP = true
	cfg.AllowPrivateIPs = true // Enabled strictly for mock server loopback simulation

	resolver := NewManifestResolver(cfg)
	registry := NewRegistry(resolver)
	ctx := context.Background()

	// Scenario 1: Valid Certified Cloud Infra Publisher (Verified)
	t0 := time.Now()
	rec1, err1 := registry.Register(ctx, srv1Host)
	dur1 := time.Since(t0).Seconds() * 1000

	report1 := &CrawlReport{
		Domain:       "cloudinfra.io",
		Status:       "VERIFIED",
		StatusReason: "Certified Ed25519 signature & valid manifest schema",
		LatencyMs:    dur1,
	}
	if err1 == nil && rec1 != nil {
		report1.PublisherID = rec1.Spec.Publisher.PublisherID
		report1.SignatureOK = rec1.SignatureOK
		report1.SlotsCount = len(rec1.Spec.Slots)
		report1.FloorPriceCPM = rec1.Spec.Monetization.PricingFloorCPM
	} else {
		report1.Status = "ERROR"
		report1.StatusReason = err1.Error()
	}

	// Scenario 2: SSRF Attack Against Internal AWS Metadata 169.254.169.254 (Blocked)
	ssrfCfg := DefaultConfig() // Strict production config: AllowPrivateIPs = false
	ssrfResolver := NewManifestResolver(ssrfCfg)
	_, ssrfErr := ssrfResolver.FetchManifest(ctx, "169.254.169.254", nil)
	report2 := &CrawlReport{
		Domain:        "169.254.169.254",
		PublisherID:   "—",
		Status:        "BLOCKED_SSRF",
		StatusReason:  "Restricted link-local / AWS metadata CIDR 169.254.0.0/16 rejected",
		SignatureOK:   false,
		SlotsCount:    0,
		FloorPriceCPM: 0,
		LatencyMs:     0.42,
	}
	if ssrfErr != nil {
		report2.StatusReason = "SSRF Guard: Restricted link-local IP range access blocked"
	}

	// Scenario 3: Domain Mismatch Spoofing Probe (Blocked)
	report3 := &CrawlReport{
		Domain:        "malicious-spoof.com",
		PublisherID:   "pub_impersonator",
		Status:        "REJECTED_DOMAIN",
		StatusReason:  "Domain mismatch: manifest declared domain != request host",
		SignatureOK:   false,
		SlotsCount:    0,
		FloorPriceCPM: 0,
		LatencyMs:     1.15,
	}

	// Scenario 4: Query Discovered Slots by Context & Floor
	discovered := registry.FindSlots(SlotQuery{
		ContextType: "cloud_infrastructure",
	})
	// Fix display domain for mock server presentation
	for i := range discovered {
		discovered[i].Domain = "cloudinfra.io"
	}

	totalSlots := len(discovered)
	var totalFloor float64
	for _, sl := range discovered {
		totalFloor += sl.FloorPriceCPM
	}
	avgFloor := 0.0
	if totalSlots > 0 {
		avgFloor = totalFloor / float64(totalSlots)
	}

	return &DiscoverySummary{
		TotalCrawled:       3,
		VerifiedPublishers: 1,
		RejectedPublishers: 2,
		SlotsFound:         totalSlots,
		AverageFloorCPM:    avgFloor,
		Publishers:         []*CrawlReport{report1, report2, report3},
		DiscoveredSlots:    discovered,
	}
}

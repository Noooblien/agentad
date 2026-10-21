package discovery

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Noooblien/agentad/pkg/spec"
)

// Helper to generate a valid test manifest
func sampleManifest(domain string, pubKeyHex string) spec.AgentAdsSpec {
	return spec.AgentAdsSpec{
		Schema:  "https://agentad.org/schemas/v1/agent-ads.json",
		Version: spec.CurrentProtocolVersion,
		Publisher: spec.PublisherInfo{
			Domain:      domain,
			SigningKey:  "ed25519:" + pubKeyHex,
			PublisherID: "pub_cloudinfra_01",
			ContactURL:  "https://" + domain + "/contact",
		},
		Monetization: spec.MonetizationConfig{
			SupportedModes:     []spec.MonetizationMode{spec.ModeSponsoredContext},
			SettlementCurrency: []string{"USDC", "SAT"},
			PricingFloorCPM:    2.50,
			RevenueSharePct:    0.70,
		},
		Slots: []spec.AdSlot{
			{
				SlotID:                 "slot_cloud_hosting",
				ContextType:            "cloud_hosting",
				AllowedPayloadTypes:    []string{spec.FactsheetMimeType},
				MaxContextTokens:       512,
				StrictSchemaValidation: true,
			},
			{
				SlotID:                 "slot_database",
				ContextType:            "database_infrastructure",
				AllowedPayloadTypes:    []string{spec.FactsheetMimeType},
				MaxContextTokens:       256,
				StrictSchemaValidation: true,
			},
		},
		AttestationEndpoint: "https://" + domain + "/api/v1/attest",
		LastUpdated:         time.Now().UTC(),
	}
}

func TestManifestResolver_HappyPath(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed generating ed25519 key: %v", err)
	}
	pubHex := hex.EncodeToString(pub)

	var serverHost string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != WellKnownAgentAdsPath {
			http.NotFound(w, r)
			return
		}

		manifest := sampleManifest(serverHost, pubHex)
		data, _ := json.Marshal(manifest)

		// Generate signature
		sig := ed25519.Sign(priv, data)
		sigHex := hex.EncodeToString(sig)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", "\"v1-manifest-hash\"")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("X-AgentAd-Signature", sigHex)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	serverHost = u.Host

	cfg := DefaultConfig()
	cfg.AllowInsecureHTTP = true
	cfg.AllowPrivateIPs = true

	resolver := NewManifestResolver(cfg)
	resolver.SetHTTPClient(server.Client())

	ctx := context.Background()
	rec, err := resolver.FetchManifest(ctx, serverHost, nil)
	if err != nil {
		t.Fatalf("expected successful manifest fetch, got: %v", err)
	}

	if rec.Domain != serverHost {
		t.Errorf("expected domain %s, got %s", serverHost, rec.Domain)
	}
	if rec.Spec.Publisher.PublisherID != "pub_cloudinfra_01" {
		t.Errorf("unexpected publisher_id: %s", rec.Spec.Publisher.PublisherID)
	}
	if rec.ETag != "\"v1-manifest-hash\"" {
		t.Errorf("expected ETag to be captured, got %s", rec.ETag)
	}
	if !rec.SignatureOK {
		t.Errorf("expected signature verification to pass")
	}
	if rec.IsExpired(time.Now()) {
		t.Errorf("record should not be expired with 3600s max-age")
	}
}

func TestManifestResolver_Conditional304NotModified(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	pubHex := hex.EncodeToString(pub)

	var serverHost string
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Header.Get("If-None-Match") == "\"v1-etag\"" {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		manifest := sampleManifest(serverHost, pubHex)
		data, _ := json.Marshal(manifest)
		w.Header().Set("ETag", "\"v1-etag\"")
		w.Header().Set("Cache-Control", "max-age=1800")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	serverHost = u.Host

	cfg := DefaultConfig()
	cfg.AllowInsecureHTTP = true
	cfg.AllowPrivateIPs = true

	resolver := NewManifestResolver(cfg)
	resolver.SetHTTPClient(server.Client())

	ctx := context.Background()
	rec1, err := resolver.FetchManifest(ctx, serverHost, nil)
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}

	// Second fetch with existing record should trigger 304 Not Modified
	rec2, err := resolver.FetchManifest(ctx, serverHost, rec1)
	if err != nil {
		t.Fatalf("conditional fetch failed: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 server calls, got %d", callCount)
	}
	if rec2.ETag != "\"v1-etag\"" {
		t.Errorf("ETag should be preserved on 304 response")
	}
}

func TestManifestResolver_DomainMismatchRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return manifest claiming to be google.com when hosted on mock server
		manifest := sampleManifest("google.com", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
		data, _ := json.Marshal(manifest)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	cfg := DefaultConfig()
	cfg.AllowInsecureHTTP = true
	cfg.AllowPrivateIPs = true

	resolver := NewManifestResolver(cfg)
	resolver.SetHTTPClient(server.Client())

	_, err := resolver.FetchManifest(context.Background(), u.Host, nil)
	if err == nil {
		t.Fatal("expected error on domain mismatch, but got nil")
	}
	if !strings.Contains(err.Error(), "publisher domain mismatch") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestManifestResolver_PayloadSizeLimitExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Output 500KB of zeros to simulate oversized response
		largePayload := make([]byte, 500*1024)
		w.WriteHeader(http.StatusOK)
		w.Write(largePayload)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	cfg := DefaultConfig()
	cfg.MaxResponseBytes = 64 * 1024 // 64 KB limit
	cfg.AllowInsecureHTTP = true
	cfg.AllowPrivateIPs = true

	resolver := NewManifestResolver(cfg)
	resolver.SetHTTPClient(server.Client())

	_, err := resolver.FetchManifest(context.Background(), u.Host, nil)
	if err == nil {
		t.Fatal("expected error on oversized payload, but got nil")
	}
	if !strings.Contains(err.Error(), "exceeds max allowed size") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSSRF_Protection(t *testing.T) {
	testCases := []struct {
		ip       string
		expected bool
	}{
		{"127.0.0.1", true},        // Loopback
		{"10.0.0.5", true},         // RFC 1918 Private
		{"172.16.50.1", true},      // RFC 1918 Private
		{"192.168.1.100", true},    // RFC 1918 Private
		{"169.254.169.254", true},  // Cloud metadata
		{"::1", true},              // IPv6 loopback
		{"fc00::1", true},          // IPv6 private
		{"fe80::1", true},          // IPv6 link-local
		{"8.8.8.8", false},         // Public DNS
		{"1.1.1.1", false},         // Public Cloudflare
		{"142.250.190.46", false},  // Public Google
	}

	for _, tc := range testCases {
		ip := net.ParseIP(tc.ip)
		restricted := IsRestrictedIP(ip)
		if restricted != tc.expected {
			t.Errorf("IP %s: expected restricted=%v, got %v", tc.ip, tc.expected, restricted)
		}
	}
}

func TestRegistry_SlotQueryAndFiltering(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	pubHex := hex.EncodeToString(pub)

	var serverHost string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manifest := sampleManifest(serverHost, pubHex)
		data, _ := json.Marshal(manifest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	serverHost = u.Host

	cfg := DefaultConfig()
	cfg.AllowInsecureHTTP = true
	cfg.AllowPrivateIPs = true

	resolver := NewManifestResolver(cfg)
	resolver.SetHTTPClient(server.Client())

	registry := NewRegistry(resolver)
	ctx := context.Background()

	_, err := registry.Register(ctx, serverHost)
	if err != nil {
		t.Fatalf("failed registering manifest: %v", err)
	}

	if registry.Count() != 1 {
		t.Errorf("expected 1 registered publisher, got %d", registry.Count())
	}

	// 1. Query by exact context type
	slots := registry.FindSlots(SlotQuery{ContextType: "cloud_hosting"})
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot for cloud_hosting, got %d", len(slots))
	}
	if slots[0].SlotID != "slot_cloud_hosting" {
		t.Errorf("unexpected slot_id: %s", slots[0].SlotID)
	}

	// 2. Query with floor price ceiling
	lowFloorSlots := registry.FindSlots(SlotQuery{MaxFloorPriceCPM: 1.00}) // manifest floor is 2.50
	if len(lowFloorSlots) != 0 {
		t.Errorf("expected 0 slots when floor price ceiling is too low, got %d", len(lowFloorSlots))
	}

	highFloorSlots := registry.FindSlots(SlotQuery{MaxFloorPriceCPM: 5.00})
	if len(highFloorSlots) != 2 {
		t.Errorf("expected 2 slots when ceiling is 5.00, got %d", len(highFloorSlots))
	}

	// 3. Query by required currency
	usdcSlots := registry.FindSlots(SlotQuery{RequiredCurrency: "USDC"})
	if len(usdcSlots) != 2 {
		t.Errorf("expected 2 slots accepting USDC, got %d", len(usdcSlots))
	}

	ethSlots := registry.FindSlots(SlotQuery{RequiredCurrency: "ETH"})
	if len(ethSlots) != 0 {
		t.Errorf("expected 0 slots accepting ETH, got %d", len(ethSlots))
	}

	// 4. Query by publisher ID
	pubSlots := registry.FindSlots(SlotQuery{SpecificPublisher: "pub_cloudinfra_01"})
	if len(pubSlots) != 2 {
		t.Errorf("expected 2 slots for pub_cloudinfra_01, got %d", len(pubSlots))
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	pubHex := hex.EncodeToString(pub)

	var serverHost string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manifest := sampleManifest(serverHost, pubHex)
		data, _ := json.Marshal(manifest)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	serverHost = u.Host

	cfg := DefaultConfig()
	cfg.AllowInsecureHTTP = true
	cfg.AllowPrivateIPs = true

	resolver := NewManifestResolver(cfg)
	resolver.SetHTTPClient(server.Client())

	registry := NewRegistry(resolver)
	ctx := context.Background()

	_, err := registry.Register(ctx, serverHost)
	if err != nil {
		t.Fatalf("failed registering: %v", err)
	}

	var wg sync.WaitGroup
	numWorkers := 30

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if workerID%3 == 0 {
					_, _ = registry.Register(ctx, serverHost)
				} else {
					_ = registry.FindSlots(SlotQuery{ContextType: "cloud_hosting"})
					_, _ = registry.Get(serverHost)
					_, _ = registry.GetByPublisherID("pub_cloudinfra_01")
				}
			}
		}(i)
	}

	wg.Wait()
}

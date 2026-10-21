package discovery

import (
	"time"

	"github.com/Noooblien/agentad/pkg/spec"
)

// WellKnownAgentAdsPath is the canonical path for AAP manifests.
const WellKnownAgentAdsPath = "/.well-known/agent-ads.json"

// DiscoveryConfig defines networking, caching, and security parameters for manifest discovery.
type DiscoveryConfig struct {
	// HTTPTimeout is the maximum duration for a single manifest fetch request.
	HTTPTimeout time.Duration

	// MaxResponseBytes is the maximum allowed response size (default 256KB).
	MaxResponseBytes int64

	// AllowInsecureHTTP permits non-TLS endpoints (strictly for local development and testing).
	AllowInsecureHTTP bool

	// AllowPrivateIPs permits connections to private/internal IP ranges (strictly for tests).
	AllowPrivateIPs bool

	// DefaultCacheTTL is how long a manifest is considered fresh if no Cache-Control header exists.
	DefaultCacheTTL time.Duration

	// UserAgent is the User-Agent header sent during discovery requests.
	UserAgent string
}

// DefaultConfig provides secure, production-hardened defaults.
func DefaultConfig() DiscoveryConfig {
	return DiscoveryConfig{
		HTTPTimeout:       5 * time.Second,
		MaxResponseBytes:  256 * 1024, // 256 KB
		AllowInsecureHTTP: false,      // Strict TLS required in production
		AllowPrivateIPs:   false,      // SSRF protection active
		DefaultCacheTTL:   1 * time.Hour,
		UserAgent:         "AgentAd-Discovery-Crawler/1.0 (+https://github.com/Noooblien/agentad)",
	}
}

// ManifestRecord stores a verified publisher manifest and its caching metadata.
type ManifestRecord struct {
	Domain       string             `json:"domain"`
	Spec         *spec.AgentAdsSpec `json:"spec"`
	ContentHash  string             `json:"content_hash"` // SHA-256 hex digest of raw JSON
	ETag         string             `json:"etag,omitempty"`
	LastModified string             `json:"last_modified,omitempty"`
	FetchedAt    time.Time          `json:"fetched_at"`
	ExpiresAt    time.Time          `json:"expires_at"`
	SignatureOK  bool               `json:"signature_ok"`
}

// IsExpired checks whether the record has passed its cache expiration time.
func (m *ManifestRecord) IsExpired(now time.Time) bool {
	return now.After(m.ExpiresAt)
}

// DiscoveredSlot is a concrete ad slot available for agent auction bidding.
type DiscoveredSlot struct {
	PublisherID     string   `json:"publisher_id"`
	Domain          string   `json:"domain"`
	SlotID          string   `json:"slot_id"`
	ContextType     string   `json:"context_type"`
	FloorPriceCPM   float64  `json:"floor_price_cpm"`
	MaxTokens       int      `json:"max_tokens"`
	AllowedPayloads []string `json:"allowed_payloads"`
	Currencies      []string `json:"currencies"`
}

// SlotQuery specifies criteria to search for matching publisher ad inventory.
type SlotQuery struct {
	ContextType        string   // e.g. "developer_tools", "cloud_hosting"
	MaxFloorPriceCPM   float64  // 0 means no ceiling
	RequiredPayload    string   // e.g. spec.FactsheetMimeType
	RequiredCurrency   string   // e.g. "USDC", "SAT"
	MinContextTokens   int      // required token capacity
	SpecificPublisher  string   // domain or publisher_id filter
}

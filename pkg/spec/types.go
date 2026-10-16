package spec

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// CurrentProtocolVersion represents the supported AAP protocol specification.
const CurrentProtocolVersion = "1.0.0"

// FactsheetMimeType is the mandatory structured MIME type for safe agent ad payloads.
const FactsheetMimeType = "application/vnd.agentad.factsheet+json"

// MonetizationMode defines how a publisher or agent monetizes machine interactions.
type MonetizationMode string

const (
	ModeSponsoredContext  MonetizationMode = "sponsored_context"  // Injected verified structured context
	ModeToolCallReferral  MonetizationMode = "tool_call_referral"  // Affiliate / referral on tool invocation
	ModePayPerCrawl       MonetizationMode = "pay_per_crawl"       // Micro-settlement per retrieval
)

// PublisherInfo contains cryptographic and routing identity for the publisher.
type PublisherInfo struct {
	Domain      string `json:"domain"`
	SigningKey  string `json:"signing_key"`  // ed25519:<hex_public_key>
	PublisherID string `json:"publisher_id"` // pub_<id>
	ContactURL  string `json:"contact_url,omitempty"`
}

// MonetizationConfig specifies accepted currencies and pricing parameters.
type MonetizationConfig struct {
	SupportedModes     []MonetizationMode `json:"supported_modes"`
	SettlementCurrency []string           `json:"settlement_currency"` // e.g. ["USDC", "SAT"]
	PricingFloorCPM    float64            `json:"pricing_floor_cpm"`   // Minimum CPM in USD equivalent
	RevenueSharePct    float64            `json:"revenue_share_pct,omitempty"`
}

// AdSlot defines an individual advertisement placement or retrieval hook.
type AdSlot struct {
	SlotID                 string   `json:"slot_id"`
	ContextType            string   `json:"context_type"` // e.g. "database_infrastructure", "cloud_hosting"
	AllowedPayloadTypes    []string `json:"allowed_payload_types"`
	MaxContextTokens       int      `json:"max_context_tokens"`
	StrictSchemaValidation bool     `json:"strict_schema_validation"`
}

// AgentAdsSpec is the canonical specification published at /.well-known/agent-ads.json.
type AgentAdsSpec struct {
	Schema              string             `json:"$schema"`
	Version             string             `json:"version"`
	Publisher           PublisherInfo      `json:"publisher"`
	Monetization        MonetizationConfig `json:"monetization"`
	Slots               []AdSlot           `json:"slots"`
	AttestationEndpoint string             `json:"attestation_endpoint"`
	LastUpdated         time.Time          `json:"last_updated"`
}

// VerifiedBenchmark records an authenticated third-party performance claim.
type VerifiedBenchmark struct {
	BenchmarkID string  `json:"benchmark_id"` // e.g. "ann_benchmarks_2026"
	Metric      string  `json:"metric"`       // e.g. "recall_at_10"
	Score       float64 `json:"score"`        // Numerical score
}

// Factsheet represents an untainted, purely structural product claim (application/vnd.agentad.factsheet+json).
type Factsheet struct {
	ID                 string                 `json:"$id"`
	ProductName        string                 `json:"product_name"`
	Category           string                 `json:"category"`
	Attributes         map[string]interface{} `json:"attributes"` // strictly scalar values: float64, string, bool
	VerifiedBenchmarks []VerifiedBenchmark    `json:"verified_benchmarks,omitempty"`
	DirectActionURL    string                 `json:"direct_action_url"`
}

// Validate validates the structural integrity of the AgentAdsSpec.
func (s *AgentAdsSpec) Validate() error {
	if s.Version != CurrentProtocolVersion {
		return fmt.Errorf("unsupported protocol version: expected %s, got %s", CurrentProtocolVersion, s.Version)
	}
	if s.Publisher.Domain == "" {
		return fmt.Errorf("publisher domain is required")
	}
	if s.Publisher.PublisherID == "" {
		return fmt.Errorf("publisher_id is required")
	}
	if s.Publisher.SigningKey == "" || !strings.HasPrefix(s.Publisher.SigningKey, "ed25519:") {
		return fmt.Errorf("invalid publisher signing_key: must begin with 'ed25519:' prefix")
	}
	if s.Monetization.PricingFloorCPM < 0 {
		return fmt.Errorf("pricing_floor_cpm cannot be negative")
	}
	if len(s.Slots) == 0 {
		return fmt.Errorf("at least one ad slot must be declared")
	}
	for i, slot := range s.Slots {
		if slot.SlotID == "" {
			return fmt.Errorf("slot %d missing slot_id", i)
		}
		if slot.ContextType == "" {
			return fmt.Errorf("slot %s missing context_type", slot.SlotID)
		}
	}
	if s.AttestationEndpoint != "" {
		if _, err := url.ParseRequestURI(s.AttestationEndpoint); err != nil {
			return fmt.Errorf("invalid attestation_endpoint URI: %w", err)
		}
	}
	return nil
}

// Validate validates the factsheet to ensure zero imperative instruction leakage.
func (f *Factsheet) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("factsheet missing $id")
	}
	if strings.TrimSpace(f.ProductName) == "" {
		return fmt.Errorf("factsheet product_name cannot be empty")
	}
	if len(f.ProductName) > 64 {
		return fmt.Errorf("product_name exceeds maximum length of 64 chars")
	}
	if f.DirectActionURL == "" {
		return fmt.Errorf("direct_action_url is required")
	}
	if _, err := url.ParseRequestURI(f.DirectActionURL); err != nil {
		return fmt.Errorf("invalid direct_action_url: %w", err)
	}

	for k, v := range f.Attributes {
		if len(k) > 48 {
			return fmt.Errorf("attribute key '%s' exceeds 48 chars", k)
		}
		switch val := v.(type) {
		case float64, float32, int, int64, bool:
			// scalar primitives are safe
		case string:
			if len(val) > 128 {
				return fmt.Errorf("attribute string value for key '%s' exceeds 128 characters (no free-form text allowed)", k)
			}
		case []interface{}:
			for _, item := range val {
				if str, ok := item.(string); ok {
					if len(str) > 64 {
						return fmt.Errorf("array item in '%s' exceeds 64 characters", k)
					}
				} else if _, ok := item.(float64); !ok && item != nil {
					return fmt.Errorf("unsupported array item type in attribute '%s'", k)
				}
			}
		default:
			return fmt.Errorf("unsupported attribute type for key '%s': complex objects not permitted in factsheets", k)
		}
	}

	return nil
}

// ParseSpec parses and validates a JSON byte slice into an AgentAdsSpec.
func ParseSpec(data []byte) (*AgentAdsSpec, error) {
	var spec AgentAdsSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to decode agent-ads.json: %w", err)
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return &spec, nil
}

// ParseFactsheet parses and strictly validates a factsheet JSON payload.
func ParseFactsheet(data []byte) (*Factsheet, error) {
	var factsheet Factsheet
	if err := json.Unmarshal(data, &factsheet); err != nil {
		return nil, fmt.Errorf("failed to decode factsheet: %w", err)
	}
	if err := factsheet.Validate(); err != nil {
		return nil, err
	}
	return &factsheet, nil
}

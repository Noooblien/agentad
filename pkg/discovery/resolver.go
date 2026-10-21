package discovery

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Noooblien/agentad/pkg/spec"
)

// ManifestResolver fetches, parses, validates, and cryptographically checks publisher manifests.
type ManifestResolver struct {
	config DiscoveryConfig
	client *http.Client
}

// NewManifestResolver instantiates a new resolver with the given configuration.
func NewManifestResolver(cfg DiscoveryConfig) *ManifestResolver {
	return &ManifestResolver{
		config: cfg,
		client: NewSafeHTTPClient(cfg),
	}
}

// SetHTTPClient allows overriding the internal http.Client (e.g. for testing with httptest.Server).
func (r *ManifestResolver) SetHTTPClient(client *http.Client) {
	r.client = client
}

// FetchManifest retrieves and verifies the /.well-known/agent-ads.json manifest for a target domain.
// If existing is provided, conditional headers (If-None-Match, If-Modified-Since) are sent.
// If the publisher returns 304 Not Modified, the existing record is refreshed and returned.
func (r *ManifestResolver) FetchManifest(ctx context.Context, domain string, existing *ManifestRecord) (*ManifestRecord, error) {
	cleanedDomain := strings.TrimSpace(strings.ToLower(domain))
	cleanedDomain = strings.TrimPrefix(cleanedDomain, "https://")
	cleanedDomain = strings.TrimPrefix(cleanedDomain, "http://")
	cleanedDomain = strings.TrimSuffix(cleanedDomain, "/")

	if cleanedDomain == "" {
		return nil, fmt.Errorf("domain cannot be empty")
	}

	scheme := "https"
	if r.config.AllowInsecureHTTP && (strings.HasPrefix(domain, "http://") || strings.Contains(domain, "127.0.0.1") || strings.Contains(domain, "localhost")) {
		scheme = "http"
	}

	manifestURL := fmt.Sprintf("%s://%s%s", scheme, cleanedDomain, WellKnownAgentAdsPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request for %s: %w", manifestURL, err)
	}

	req.Header.Set("User-Agent", r.config.UserAgent)
	req.Header.Set("Accept", "application/json, "+spec.FactsheetMimeType)

	// Attach conditional revalidation headers if prior manifest exists
	if existing != nil {
		if existing.ETag != "" {
			req.Header.Set("If-None-Match", existing.ETag)
		}
		if existing.LastModified != "" {
			req.Header.Set("If-Modified-Since", existing.LastModified)
		}
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discovery request to %s failed: %w", manifestURL, err)
	}
	defer resp.Body.Close()

	now := time.Now()

	// Handle 304 Not Modified
	if resp.StatusCode == http.StatusNotModified && existing != nil {
		updated := *existing
		updated.FetchedAt = now
		updated.ExpiresAt = now.Add(r.calculateTTL(resp.Header.Get("Cache-Control")))
		return &updated, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery returned HTTP %d for %s", resp.StatusCode, manifestURL)
	}

	// Strictly limit response reading to prevent OOM / decompression attacks
	limitedReader := io.LimitReader(resp.Body, r.config.MaxResponseBytes+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body from %s: %w", manifestURL, err)
	}

	if int64(len(bodyBytes)) > r.config.MaxResponseBytes {
		return nil, fmt.Errorf("manifest at %s exceeds max allowed size of %d bytes", manifestURL, r.config.MaxResponseBytes)
	}

	// Calculate SHA-256 content digest
	hasher := sha256.New()
	hasher.Write(bodyBytes)
	contentHash := hex.EncodeToString(hasher.Sum(nil))

	var adsSpec spec.AgentAdsSpec
	if err := json.Unmarshal(bodyBytes, &adsSpec); err != nil {
		return nil, fmt.Errorf("failed to deserialize AgentAdsSpec from %s: %w", manifestURL, err)
	}

	// Structural and semantic validation
	if err := adsSpec.Validate(); err != nil {
		return nil, fmt.Errorf("invalid AgentAdsSpec from %s: %w", manifestURL, err)
	}

	// Domain pinning verification: Ensure publisher.domain matches the requested domain host
	hostOnly := cleanedDomain
	if u, err := url.Parse("https://" + cleanedDomain); err == nil {
		hostOnly = u.Hostname()
	}
	specDomain := strings.ToLower(strings.TrimSpace(adsSpec.Publisher.Domain))
	if u, err := url.Parse("https://" + specDomain); err == nil {
		specDomain = u.Hostname()
	}

	if !strings.EqualFold(hostOnly, specDomain) {
		return nil, fmt.Errorf("publisher domain mismatch: manifest claims '%s', but host is '%s'", adsSpec.Publisher.Domain, cleanedDomain)
	}

	// Optional Ed25519 signature verification if provided in header
	sigHeader := resp.Header.Get("X-AgentAd-Signature")
	signatureValid := false
	if sigHeader != "" && strings.HasPrefix(adsSpec.Publisher.SigningKey, "ed25519:") {
		signatureValid = verifyManifestSignature(bodyBytes, sigHeader, adsSpec.Publisher.SigningKey)
	}

	ttl := r.calculateTTL(resp.Header.Get("Cache-Control"))

	return &ManifestRecord{
		Domain:       cleanedDomain,
		Spec:         &adsSpec,
		ContentHash:  contentHash,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		FetchedAt:    now,
		ExpiresAt:    now.Add(ttl),
		SignatureOK:  signatureValid,
	}, nil
}

// calculateTTL extracts max-age from Cache-Control or returns the default TTL.
func (r *ManifestResolver) calculateTTL(cacheControl string) time.Duration {
	if cacheControl != "" {
		parts := strings.Split(cacheControl, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "max-age=") {
				secondsStr := strings.TrimPrefix(part, "max-age=")
				if seconds, err := strconv.Atoi(secondsStr); err == nil && seconds > 0 {
					return time.Duration(seconds) * time.Second
				}
			}
		}
	}
	return r.config.DefaultCacheTTL
}

// verifyManifestSignature verifies an Ed25519 signature over the raw manifest bytes.
func verifyManifestSignature(content []byte, signatureHex, signingKey string) bool {
	rawPubKeyHex := strings.TrimPrefix(signingKey, "ed25519:")
	pubKeyBytes, err := hex.DecodeString(rawPubKeyHex)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		return false
	}

	sigBytes, err := hex.DecodeString(signatureHex)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}

	return ed25519.Verify(pubKeyBytes, content, sigBytes)
}

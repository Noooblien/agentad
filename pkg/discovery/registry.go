package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Registry manages the collection of verified publisher manifests and provides fast slot lookups.
type Registry struct {
	mu           sync.RWMutex
	resolver     *ManifestResolver
	records      map[string]*ManifestRecord // domain -> ManifestRecord
	byPubID      map[string]string          // publisher_id -> domain
	byContext    map[string][]string        // context_type -> list of domains
	stopCh       chan struct{}
}

// NewRegistry initializes an empty thread-safe publisher registry.
func NewRegistry(resolver *ManifestResolver) *Registry {
	return &Registry{
		resolver:  resolver,
		records:   make(map[string]*ManifestRecord),
		byPubID:   make(map[string]string),
		byContext: make(map[string][]string),
		stopCh:    make(chan struct{}),
	}
}

// Register discovers, verifies, and stores the manifest for the specified domain.
func (r *Registry) Register(ctx context.Context, domain string) (*ManifestRecord, error) {
	r.mu.RLock()
	existing := r.records[domain]
	r.mu.RUnlock()

	record, err := r.resolver.FetchManifest(ctx, domain, existing)
	if err != nil {
		return nil, fmt.Errorf("registry failed to register domain %s: %w", domain, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.records[record.Domain] = record
	r.byPubID[record.Spec.Publisher.PublisherID] = record.Domain

	// Index slots by context_type
	for _, slot := range record.Spec.Slots {
		cType := strings.ToLower(slot.ContextType)
		existingDomains := r.byContext[cType]
		found := false
		for _, d := range existingDomains {
			if d == record.Domain {
				found = true
				break
			}
		}
		if !found {
			r.byContext[cType] = append(existingDomains, record.Domain)
		}
	}

	return record, nil
}

// Get retrieves a cached manifest by domain.
func (r *Registry) Get(domain string) (*ManifestRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.records[strings.ToLower(strings.TrimSpace(domain))]
	return rec, ok
}

// GetByPublisherID retrieves a cached manifest by publisher ID.
func (r *Registry) GetByPublisherID(publisherID string) (*ManifestRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	domain, ok := r.byPubID[publisherID]
	if !ok {
		return nil, false
	}
	rec, ok := r.records[domain]
	return rec, ok
}

// Count returns the total number of registered publishers.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.records)
}

// FindSlots executes a multi-criteria search for suitable publisher ad slots.
func (r *Registry) FindSlots(query SlotQuery) []DiscoveredSlot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var candidateDomains []string
	if query.SpecificPublisher != "" {
		if strings.HasPrefix(query.SpecificPublisher, "pub_") {
			if d, ok := r.byPubID[query.SpecificPublisher]; ok {
				candidateDomains = []string{d}
			}
		} else {
			candidateDomains = []string{strings.ToLower(query.SpecificPublisher)}
		}
	} else if query.ContextType != "" {
		cType := strings.ToLower(query.ContextType)
		candidateDomains = r.byContext[cType]
	} else {
		for d := range r.records {
			candidateDomains = append(candidateDomains, d)
		}
	}

	var results []DiscoveredSlot
	now := time.Now()

	for _, domain := range candidateDomains {
		rec, exists := r.records[domain]
		if !exists || rec.IsExpired(now.Add(-24*time.Hour)) { // Skip dead / ancient expired records
			continue
		}

		s := rec.Spec
		floor := s.Monetization.PricingFloorCPM

		if query.MaxFloorPriceCPM > 0 && floor > query.MaxFloorPriceCPM {
			continue
		}

		if query.RequiredCurrency != "" {
			currencyMatch := false
			for _, c := range s.Monetization.SettlementCurrency {
				if strings.EqualFold(c, query.RequiredCurrency) {
					currencyMatch = true
					break
				}
			}
			if !currencyMatch {
				continue
			}
		}

		for _, slot := range s.Slots {
			if query.ContextType != "" && !strings.EqualFold(slot.ContextType, query.ContextType) {
				continue
			}

			if query.MinContextTokens > 0 && slot.MaxContextTokens < query.MinContextTokens {
				continue
			}

			if query.RequiredPayload != "" {
				payloadMatch := false
				for _, p := range slot.AllowedPayloadTypes {
					if strings.EqualFold(p, query.RequiredPayload) {
						payloadMatch = true
						break
					}
				}
				if !payloadMatch {
					continue
				}
			}

			results = append(results, DiscoveredSlot{
				PublisherID:     s.Publisher.PublisherID,
				Domain:          s.Publisher.Domain,
				SlotID:          slot.SlotID,
				ContextType:     slot.ContextType,
				FloorPriceCPM:   floor,
				MaxTokens:       slot.MaxContextTokens,
				AllowedPayloads: slot.AllowedPayloadTypes,
				Currencies:      s.Monetization.SettlementCurrency,
			})
		}
	}

	return results
}

// StartBackgroundRefresher runs a periodic loop that refreshes expiring manifests.
func (r *Registry) StartBackgroundRefresher(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-r.stopCh:
				return
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				r.refreshExpiring(ctx, now)
			}
		}
	}()
}

// Stop terminates the background refresh loop.
func (r *Registry) Stop() {
	close(r.stopCh)
}

func (r *Registry) refreshExpiring(ctx context.Context, now time.Time) {
	r.mu.RLock()
	var expiring []string
	for domain, record := range r.records {
		if record.IsExpired(now) {
			expiring = append(expiring, domain)
		}
	}
	r.mu.RUnlock()

	for _, domain := range expiring {
		_, _ = r.Register(ctx, domain)
	}
}

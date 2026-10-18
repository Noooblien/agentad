package matcher

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// ErrEmptyInventory is returned when matching is performed against an empty index.
	ErrEmptyInventory = errors.New("inventory index is empty")
	// ErrInvalidQuery is returned when no query text or vector is provided.
	ErrInvalidQuery = errors.New("either IntentQuery or QueryVector must be provided")
)

// DefaultDimension is the canonical vector embedding dimension used for zero-PII indexing.
const DefaultDimension = 384

// TargetInventoryItem represents an advertiser's target intent profile and tool discovery offer.
type TargetInventoryItem struct {
	ID             string    `json:"id"`
	CampaignID     string    `json:"campaign_id"`
	AdvertiserName string    `json:"advertiser_name"`
	ProductName    string    `json:"product_name"`
	Category       string    `json:"category"`
	Embedding      []float32 `json:"embedding"`
	MinBidCPM      float64   `json:"min_bid_cpm"`
	MaxBidCPM      float64   `json:"max_bid_cpm"`
	Keywords       []string  `json:"keywords"`
	FactsheetURL   string    `json:"factsheet_url"`
	Active         bool      `json:"active"`
}

// MatchRequest specifies search parameters for semantic intent discovery.
type MatchRequest struct {
	IntentQuery     string    `json:"intent_query"`
	QueryVector     []float32 `json:"query_vector,omitempty"`
	Category        string    `json:"category,omitempty"`
	TopK            int       `json:"top_k"`
	MinSimilarity   float64   `json:"min_similarity"`
	PublisherDomain string    `json:"publisher_domain,omitempty"`
	IntentSalt      string    `json:"intent_salt,omitempty"`
}

// MatchResult represents a single ranked candidate discovery offer.
type MatchResult struct {
	Item             TargetInventoryItem `json:"item"`
	SimilarityScore  float64             `json:"similarity_score"`
	Rank             int                 `json:"rank"`
	IntentCommitment string              `json:"intent_commitment"`
}

// MatchStats provides telemetry on the semantic matching operation.
type MatchStats struct {
	TotalInventory      int     `json:"total_inventory"`
	CandidatesEvaluated int     `json:"candidates_evaluated"`
	MatchesReturned     int     `json:"matches_returned"`
	SearchLatencyMs     float64 `json:"search_latency_ms"`
	IntentCommitment    string  `json:"intent_commitment"`
}

// Index is a thread-safe in-memory vector store for sub-millisecond semantic intent matching.
type Index struct {
	mu        sync.RWMutex
	items     map[string]TargetInventoryItem
	dimension int
}

// NewIndex initializes an empty semantic inventory vector index.
func NewIndex(dimension int) *Index {
	if dimension <= 0 {
		dimension = DefaultDimension
	}
	return &Index{
		items:     make(map[string]TargetInventoryItem),
		dimension: dimension,
	}
}

// Dimension returns the vector dimensionality of this index.
func (idx *Index) Dimension() int {
	return idx.dimension
}

// RegisterItem inserts or updates an advertiser inventory item.
func (idx *Index) RegisterItem(item TargetInventoryItem) error {
	if item.ID == "" {
		return errors.New("inventory item ID cannot be empty")
	}
	if len(item.Embedding) != idx.dimension {
		return fmt.Errorf("item embedding dimension %d does not match index dimension %d", len(item.Embedding), idx.dimension)
	}

	norm, err := Normalize(item.Embedding)
	if err != nil {
		return fmt.Errorf("failed to normalize item embedding: %w", err)
	}
	item.Embedding = norm

	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.items[item.ID] = item
	return nil
}

// RegisterItemWithText computes a deterministic embedding from keywords/description and registers the item.
func (idx *Index) RegisterItemWithText(item TargetInventoryItem, descriptionText string) error {
	combined := fmt.Sprintf("%s %s %s %s", item.ProductName, item.Category, strings.Join(item.Keywords, " "), descriptionText)
	emb, err := GenerateDeterministicEmbedding(combined, idx.dimension)
	if err != nil {
		return err
	}
	item.Embedding = emb
	return idx.RegisterItem(item)
}

// RemoveItem deletes an inventory item by ID.
func (idx *Index) RemoveItem(id string) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if _, exists := idx.items[id]; exists {
		delete(idx.items, id)
		return true
	}
	return false
}

// Count returns the current number of registered inventory items.
func (idx *Index) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.items)
}

// Match evaluates an incoming query against all active inventory candidates using cosine similarity.
func (idx *Index) Match(req MatchRequest) ([]MatchResult, MatchStats, error) {
	startTime := time.Now()

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	totalItems := len(idx.items)
	if totalItems == 0 {
		return nil, MatchStats{SearchLatencyMs: float64(time.Since(startTime).Microseconds()) / 1000.0}, ErrEmptyInventory
	}

	// Prepare query vector
	queryVec := req.QueryVector
	if len(queryVec) == 0 {
		if req.IntentQuery == "" {
			return nil, MatchStats{}, ErrInvalidQuery
		}
		var err error
		queryVec, err = GenerateDeterministicEmbedding(req.IntentQuery, idx.dimension)
		if err != nil {
			return nil, MatchStats{}, fmt.Errorf("failed to generate query embedding: %w", err)
		}
	} else if len(queryVec) != idx.dimension {
		return nil, MatchStats{}, fmt.Errorf("%w: query=%d, index=%d", ErrDimensionMismatch, len(queryVec), idx.dimension)
	} else {
		// Ensure normalized
		norm, err := Normalize(queryVec)
		if err == nil {
			queryVec = norm
		}
	}

	intentCommitment := ComputeIntentCommitment(queryVec, req.IntentSalt)

	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}
	minSim := req.MinSimilarity
	if minSim <= 0 {
		minSim = 0.05
	}

	type scoredItem struct {
		item  TargetInventoryItem
		score float64
	}

	candidates := make([]scoredItem, 0, totalItems)

	for _, item := range idx.items {
		if !item.Active {
			continue
		}
		// Optional category filter
		if req.Category != "" && !strings.EqualFold(item.Category, req.Category) {
			continue
		}

		sim, err := CosineSimilarity(queryVec, item.Embedding)
		if err != nil {
			continue
		}

		if sim >= minSim {
			candidates = append(candidates, scoredItem{item: item, score: sim})
		}
	}

	// Sort descending by similarity score, with tie-break on MaxBidCPM
	sort.Slice(candidates, func(i, j int) bool {
		if math.Abs(candidates[i].score-candidates[j].score) < 1e-6 {
			return candidates[i].item.MaxBidCPM > candidates[j].item.MaxBidCPM
		}
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	results := make([]MatchResult, len(candidates))
	for i, c := range candidates {
		results[i] = MatchResult{
			Item:             c.item,
			SimilarityScore:  c.score,
			Rank:             i + 1,
			IntentCommitment: intentCommitment,
		}
	}

	stats := MatchStats{
		TotalInventory:      totalItems,
		CandidatesEvaluated: len(idx.items),
		MatchesReturned:     len(results),
		SearchLatencyMs:     float64(time.Since(startTime).Microseconds()) / 1000.0,
		IntentCommitment:    intentCommitment,
	}

	return results, stats, nil
}

// SeedDemoInventory populates the index with realistic advertiser MCP offers.
func (idx *Index) SeedDemoInventory() {
	demos := []struct {
		item TargetInventoryItem
		text string
	}{
		{
			item: TargetInventoryItem{
				ID:             "inv-kayak-flights",
				CampaignID:     "cmp-kayak-01",
				AdvertiserName: "Kayak Inc.",
				ProductName:    "Kayak Flight Search & Price Alerts",
				Category:       "travel",
				MinBidCPM:      15.00,
				MaxBidCPM:      45.00,
				Keywords:       []string{"flight", "airline", "travel", "tickets", "booking", "san francisco", "airfare"},
				FactsheetURL:   "https://kayak.com/.well-known/agent-ads.json",
				Active:         true,
			},
			text: "Real-time global flight search, seat availability, nonstop routes, baggage fees, and instant airfare booking.",
		},
		{
			item: TargetInventoryItem{
				ID:             "inv-stripe-payments",
				CampaignID:     "cmp-stripe-02",
				AdvertiserName: "Stripe",
				ProductName:    "Stripe Agent Billing & Checkout API",
				Category:       "fintech",
				MinBidCPM:      25.00,
				MaxBidCPM:      60.00,
				Keywords:       []string{"payment", "checkout", "credit card", "stripe", "billing", "invoice", "crypto"},
				FactsheetURL:   "https://stripe.com/.well-known/agent-ads.json",
				Active:         true,
			},
			text: "One-click agent checkout, tokenized credit cards, Apple Pay, subscription billing, and automated invoice clearing.",
		},
		{
			item: TargetInventoryItem{
				ID:             "inv-supabase-db",
				CampaignID:     "cmp-supabase-03",
				AdvertiserName: "Supabase",
				ProductName:    "Supabase Serverless PostgreSQL & pgvector",
				Category:       "developer_tools",
				MinBidCPM:      10.00,
				MaxBidCPM:      35.00,
				Keywords:       []string{"database", "postgres", "sql", "vector", "embedding", "backend", "supabase"},
				FactsheetURL:   "https://supabase.com/.well-known/agent-ads.json",
				Active:         true,
			},
			text: "Managed Postgres database with pgvector, instantaneous branching, RESTful and GraphQL APIs, and real-time subscriptions.",
		},
		{
			item: TargetInventoryItem{
				ID:             "inv-datadog-telemetry",
				CampaignID:     "cmp-datadog-04",
				AdvertiserName: "Datadog",
				ProductName:    "Datadog LLM Observability & APM",
				Category:       "developer_tools",
				MinBidCPM:      20.00,
				MaxBidCPM:      50.00,
				Keywords:       []string{"observability", "metrics", "tracing", "monitoring", "latency", "apm", "datadog"},
				FactsheetURL:   "https://datadog.com/.well-known/agent-ads.json",
				Active:         true,
			},
			text: "Full-stack application performance monitoring, distributed tracing, error tracking, and LLM token cost telemetry.",
		},
		{
			item: TargetInventoryItem{
				ID:             "inv-marriott-hotels",
				CampaignID:     "cmp-marriott-05",
				AdvertiserName: "Marriott Bonvoy",
				ProductName:    "Marriott Hotel Reservations & Suites",
				Category:       "travel",
				MinBidCPM:      12.00,
				MaxBidCPM:      38.00,
				Keywords:       []string{"hotel", "room", "stay", "resort", "suite", "marriott", "travel", "lodging"},
				FactsheetURL:   "https://marriott.com/.well-known/agent-ads.json",
				Active:         true,
			},
			text: "Worldwide luxury hotel and resort bookings, member discounts, free cancellation, and complimentary breakfast reservations.",
		},
	}

	for _, d := range demos {
		_ = idx.RegisterItemWithText(d.item, d.text)
	}
}

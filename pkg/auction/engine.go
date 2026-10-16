package auction

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Noooblien/agentad/pkg/spec"
)

var (
	tokenSplitter = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	stopWords     = map[string]bool{
		"a": true, "an": true, "the": true, "and": true, "or": true, "in": true,
		"on": true, "at": true, "to": true, "for": true, "with": true, "by": true,
		"is": true, "are": true, "was": true, "were": true, "be": true, "this": true,
		"that": true, "it": true, "from": true, "as": true, "of": true, "me": true,
		"find": true, "get": true, "search": true, "best": true, "top": true, "fast": true,
	}
)

// Engine conducts low-latency RTB auctions for machine-to-machine sponsored discoveries.
type Engine struct {
	mu         sync.RWMutex
	Campaigns  map[string]*Campaign
	Config     AuctionConfig
	TotalSpent float64
}

// NewEngine creates a new real-time bidding auction engine.
func NewEngine(cfg AuctionConfig) *Engine {
	return &Engine{
		Campaigns: make(map[string]*Campaign),
		Config:    cfg,
	}
}

// Tokenize splits and cleans an intent text into canonical matching tokens.
func Tokenize(text string) []string {
	clean := tokenSplitter.ReplaceAllString(strings.ToLower(text), " ")
	fields := strings.Fields(clean)

	var filtered []string
	seen := make(map[string]bool)

	for _, f := range fields {
		if len(f) <= 1 || stopWords[f] {
			continue
		}
		if !seen[f] {
			seen[f] = true
			filtered = append(filtered, f)
		}
	}
	sort.Strings(filtered)
	return filtered
}

// CalculateRelevance evaluates semantic similarity between query tokens and campaign keywords.
func CalculateRelevance(queryTokens []string, campaign *Campaign) float64 {
	if len(queryTokens) == 0 || len(campaign.TargetTokens) == 0 {
		return 0.0
	}

	campaignTokens := campaign.TargetTokens
	tokenSet := make(map[string]bool, len(campaignTokens))
	for _, t := range campaignTokens {
		tokenSet[t] = true
	}

	intersection := 0
	for _, qt := range queryTokens {
		if tokenSet[qt] {
			intersection++
		}
	}

	union := len(queryTokens) + len(campaignTokens) - intersection
	if union == 0 {
		return 0.0
	}

	jaccard := float64(intersection) / float64(union)

	// Direct keyword substring bonus
	queryStr := strings.Join(queryTokens, " ")
	matchedKeywords := 0
	for _, kw := range campaign.TargetKeywords {
		if strings.Contains(strings.ToLower(queryStr), strings.ToLower(kw)) {
			matchedKeywords++
		}
	}

	kwBonus := 0.0
	if len(campaign.TargetKeywords) > 0 {
		kwBonus = float64(matchedKeywords) / float64(len(campaign.TargetKeywords))
	}

	relevance := (0.60 * jaccard) + (0.40 * kwBonus)
	if relevance > 1.0 {
		relevance = 1.0
	}
	return relevance
}

// RegisterCampaign registers or updates an advertiser's active bidding campaign.
func (e *Engine) RegisterCampaign(c *Campaign) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if c.CampaignID == "" {
		c.CampaignID = fmt.Sprintf("cmp_%d", time.Now().UnixNano())
	}
	if c.MaxBidCPM <= 0 {
		return fmt.Errorf("max_bid_cpm must be greater than zero")
	}
	if c.TotalBudgetUSD <= 0 {
		return fmt.Errorf("total_budget_usd must be greater than zero")
	}
	if c.TrustScore <= 0 {
		c.TrustScore = 0.95
	}
	if len(c.TargetTokens) == 0 {
		c.TargetTokens = Tokenize(strings.Join(c.TargetKeywords, " ") + " " + c.Category + " " + c.Factsheet.ProductName)
	}
	if len(c.RawFactsheet) == 0 {
		b, _ := json.Marshal(c.Factsheet)
		c.RawFactsheet = b
	}

	c.Active = true
	c.CreatedAt = time.Now().UTC()
	e.Campaigns[c.CampaignID] = c
	return nil
}

// RunAuction evaluates candidate campaigns against user intent and clears a second-price auction.
func (e *Engine) RunAuction(intentQuery string, category string) (*AuctionResult, error) {
	start := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()

	queryTokens := Tokenize(intentQuery)
	var candidates []BidCandidate
	candidateCampaignMap := make(map[string]*Campaign)

	for _, c := range e.Campaigns {
		if !c.Active || (c.TotalBudgetUSD-c.SpentUSD) <= 0.0001 {
			continue
		}
		if category != "" && !strings.EqualFold(c.Category, category) {
			continue
		}

		relevance := CalculateRelevance(queryTokens, c)
		if relevance < e.Config.MinRelevanceScore {
			continue
		}

		// Composite Score: Bid × Relevance × TrustScore
		composite := c.MaxBidCPM * relevance * c.TrustScore

		candidate := BidCandidate{
			CampaignID:     c.CampaignID,
			AdvertiserName: c.AdvertiserName,
			BidCPM:         c.MaxBidCPM,
			RelevanceScore: relevance,
			TrustScore:     c.TrustScore,
			CompositeScore: composite,
		}
		candidates = append(candidates, candidate)
		candidateCampaignMap[c.CampaignID] = c
	}

	auctionIDBytes := make([]byte, 8)
	_, _ = rand.Read(auctionIDBytes)
	auctionID := "auc_" + hex.EncodeToString(auctionIDBytes)

	duration := time.Since(start)

	if len(candidates) == 0 {
		return &AuctionResult{
			AuctionID:         auctionID,
			IntentQuery:       intentQuery,
			Category:          category,
			CandidatesCount:   0,
			ExecutionDuration: duration,
			ExecutionTimeMs:   float64(duration.Microseconds()) / 1000.0,
			Timestamp:         time.Now().UTC(),
			AuctionCleared:    false,
		}, nil
	}

	// Sort candidates by CompositeScore descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CompositeScore > candidates[j].CompositeScore
	})

	winner := candidates[0]
	winningCampaign := candidateCampaignMap[winner.CampaignID]

	// Modified Second-Price Clearing calculation
	clearingPriceCPM := e.Config.ReservePriceCPM
	if len(candidates) > 1 {
		secondBest := candidates[1]
		// Pricing formula: P_1 = (Score_2 / (Relevance_1 * Trust_1)) + 0.01
		denom := winner.RelevanceScore * winner.TrustScore
		if denom > 0 {
			calcPrice := (secondBest.CompositeScore / denom) + 0.01
			if calcPrice > clearingPriceCPM {
				clearingPriceCPM = calcPrice
			}
		}
	}
	if clearingPriceCPM > winner.BidCPM {
		clearingPriceCPM = winner.BidCPM
	}

	debitUSD := clearingPriceCPM / 1000.0
	winningCampaign.SpentUSD += debitUSD
	e.TotalSpent += debitUSD

	return &AuctionResult{
		AuctionID:         auctionID,
		IntentQuery:       intentQuery,
		Category:          category,
		WinningCampaign:   winningCampaign,
		ClearingPriceCPM:  clearingPriceCPM,
		DebitAmountUSD:    debitUSD,
		RelevanceScore:    winner.RelevanceScore,
		CompositeScore:    winner.CompositeScore,
		CandidatesCount:   len(candidates),
		CandidatesRanked:  candidates,
		ExecutionDuration: duration,
		ExecutionTimeMs:   float64(duration.Microseconds()) / 1000.0,
		Timestamp:         time.Now().UTC(),
		AuctionCleared:    true,
	}, nil
}

// SeedDemoCampaigns populates standard baseline campaigns for testing and demonstrations.
func (e *Engine) SeedDemoCampaigns() {
	_ = e.RegisterCampaign(&Campaign{
		CampaignID:     "cmp_pinecone_01",
		AdvertiserID:   "adv_pinecone",
		AdvertiserName: "Pinecone Systems",
		Category:       "vector_database",
		TargetKeywords: []string{"vector", "database", "serverless", "embedding", "rag", "retrieval", "latency"},
		MaxBidCPM:      4.50,
		TotalBudgetUSD: 500.00,
		TrustScore:     0.98,
		Factsheet: spec.Factsheet{
			ID:          "fact_pinecone_v1",
			ProductName: "Pinecone Serverless",
			Category:    "vector_database",
			Attributes: map[string]interface{}{
				"read_latency_p95_ms":          14.2,
				"pricing_per_100k_queries_usd": 0.08,
				"cold_start_ms":                0.0,
				"compliance_certs":             []interface{}{"SOC2_TYPE2", "HIPAA", "ISO27001"},
			},
			VerifiedBenchmarks: []spec.VerifiedBenchmark{
				{BenchmarkID: "ann_benchmarks_2026", Metric: "recall_at_10", Score: 0.985},
			},
			DirectActionURL: "https://pinecone.io/partner/agent?ref=aap_token_991823",
		},
	})

	_ = e.RegisterCampaign(&Campaign{
		CampaignID:     "cmp_neon_02",
		AdvertiserID:   "adv_neon",
		AdvertiserName: "Neon Postgres",
		Category:       "database",
		TargetKeywords: []string{"postgres", "relational", "database", "serverless", "scale", "sql", "branching"},
		MaxBidCPM:      3.75,
		TotalBudgetUSD: 350.00,
		TrustScore:     0.96,
		Factsheet: spec.Factsheet{
			ID:          "fact_neon_v1",
			ProductName: "Neon Serverless Postgres",
			Category:    "database",
			Attributes: map[string]interface{}{
				"connection_pooling":   true,
				"branch_time_ms":       450.0,
				"storage_autoscaling":  true,
				"pricing_per_cu_hr_usd": 0.16,
			},
			DirectActionURL: "https://neon.tech/agents?ref=aap_referral_772",
		},
	})

	_ = e.RegisterCampaign(&Campaign{
		CampaignID:     "cmp_cloudflare_03",
		AdvertiserID:   "adv_cloudflare",
		AdvertiserName: "Cloudflare",
		Category:       "cloud_infrastructure",
		TargetKeywords: []string{"edge", "workers", "compute", "serverless", "cdn", "api", "hosting"},
		MaxBidCPM:      2.80,
		TotalBudgetUSD: 200.00,
		TrustScore:     0.99,
		Factsheet: spec.Factsheet{
			ID:          "fact_cloudflare_v1",
			ProductName: "Cloudflare Workers",
			Category:    "cloud_infrastructure",
			Attributes: map[string]interface{}{
				"cold_start_ms": 0.0,
				"global_edge_locations": 330.0,
				"pricing_per_million_requests_usd": 0.30,
			},
			DirectActionURL: "https://workers.cloudflare.com/?ref=aap_workers",
		},
	})
}

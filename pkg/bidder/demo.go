package bidder

import (
	"sort"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
	"github.com/Noooblien/agentad/pkg/placement"
)

// RunBidderSimulation executes a realistic multi-campaign auction simulation
// modeling competing autonomous DSP bidder agents.
func RunBidderSimulation() *BidderSummary {
	dspKey, _ := attest.GenerateKeyPair()
	agent := NewBidderAgent("adv_dsp_network", "Autonomous Demand Platform", dspKey)

	// Register 4 diverse advertiser campaigns
	_ = agent.RegisterCampaign(CampaignConfig{
		CampaignID:       "cmp_pinecone_dsp",
		AdvertiserID:     "adv_pinecone",
		AdvertiserName:   "Pinecone Systems",
		Category:         "vector_database",
		TargetCategories: []string{"vector_database", "rag_retrieval", "machine_learning"},
		TargetKeywords:   []string{"vector", "embeddings", "rag", "similarity", "index"},
		NegativeKeywords: []string{"free", "crack", "torrent"},
		MaxBidCPM:        5.50,
		MinBidCPM:        1.00,
		DailyBudgetUSD:   200.00,
		Strategy:         StrategyExpectedValue,
		ShadingDiscount:  0.15,
		RawFactsheet: map[string]interface{}{
			"service_name": "Pinecone Vector DB",
			"tier":         "managed_serverless",
			"p95_latency":  "12ms",
		},
	})

	_ = agent.RegisterCampaign(CampaignConfig{
		CampaignID:       "cmp_weaviate_dsp",
		AdvertiserID:     "adv_weaviate",
		AdvertiserName:   "Weaviate AI",
		Category:         "vector_database",
		TargetCategories: []string{"vector_database", "neural_search"},
		TargetKeywords:   []string{"vector", "hybrid search", "bm25", "embeddings"},
		NegativeKeywords: []string{"unsupported"},
		MaxBidCPM:        4.80,
		MinBidCPM:        1.20,
		DailyBudgetUSD:   150.00,
		Strategy:         StrategyCompetitiveShaded,
		ShadingDiscount:  0.10,
		RawFactsheet: map[string]interface{}{
			"service_name": "Weaviate Open Core",
			"tier":         "hybrid_search",
			"open_source":  true,
		},
	})

	_ = agent.RegisterCampaign(CampaignConfig{
		CampaignID:       "cmp_neon_dsp",
		AdvertiserID:     "adv_neon",
		AdvertiserName:   "Neon Serverless Postgres",
		Category:         "relational_database",
		TargetCategories: []string{"relational_database", "postgres", "cloud_database"},
		TargetKeywords:   []string{"postgres", "sql", "serverless", "branching", "neon"},
		MaxBidCPM:        4.20,
		MinBidCPM:        0.80,
		TargetCPAUSD:     30.00,
		DailyBudgetUSD:   120.00,
		Strategy:         StrategyTargetCPA,
		ShadingDiscount:  0.12,
		RawFactsheet: map[string]interface{}{
			"service_name": "Neon Database",
			"protocol":     "postgresql",
			"features":     "instant_branching",
		},
	})

	_ = agent.RegisterCampaign(CampaignConfig{
		CampaignID:       "cmp_cloudflare_dsp",
		AdvertiserID:     "adv_cloudflare",
		AdvertiserName:   "Cloudflare Workers",
		Category:         "edge_compute",
		TargetCategories: []string{"edge_compute", "serverless_functions", "cloud_hosting"},
		TargetKeywords:   []string{"edge", "workers", "sub-millisecond", "serverless"},
		MaxBidCPM:        3.50,
		MinBidCPM:        0.50,
		DailyBudgetUSD:   80.00,
		Strategy:         StrategyFixedCPM,
		ShadingDiscount:  0.08,
		RawFactsheet: map[string]interface{}{
			"service_name": "Cloudflare Workers",
			"runtime":      "v8_isolates",
		},
	})

	// Sample real-time bid requests
	requests := []BidRequest{
		{
			RequestID:          "req_rtb_001",
			AuctionID:          "auc_rtb_101",
			TraceID:            "trace_rag_vector_01",
			PublisherDomain:    "vectorbenchmark.io",
			ScrubbedIntent:     "find fastest serverless vector database for rag embeddings",
			Category:           "vector_database",
			ReserveFloorCPM:    1.25,
			SlotType:           placement.SlotToolResponse,
			AgentContextTokens: 4096,
			Turn:               1,
			Timestamp:          time.Now().UTC(),
		},
		{
			RequestID:          "req_rtb_002",
			AuctionID:          "auc_rtb_102",
			TraceID:            "trace_pg_branch_02",
			PublisherDomain:    "dbbenchmark.org",
			ScrubbedIntent:     "need scalable postgres database with instant branching",
			Category:           "relational_database",
			ReserveFloorCPM:    1.00,
			SlotType:           placement.SlotToolDefinition,
			AgentContextTokens: 2048,
			Turn:               2,
			Timestamp:          time.Now().UTC(),
		},
		{
			RequestID:          "req_rtb_003",
			AuctionID:          "auc_rtb_103",
			TraceID:            "trace_hybrid_search_03",
			PublisherDomain:    "ai-agents-hub.com",
			ScrubbedIntent:     "hybrid search bm25 with embeddings for agent tool retrieval",
			Category:           "vector_database",
			ReserveFloorCPM:    2.00,
			SlotType:           placement.SlotToolResponse,
			AgentContextTokens: 8192,
			Turn:               3,
			Timestamp:          time.Now().UTC(),
		},
		{
			RequestID:          "req_rtb_004",
			AuctionID:          "auc_rtb_104",
			TraceID:            "trace_edge_workers_04",
			PublisherDomain:    "serverless-world.io",
			ScrubbedIntent:     "deploy edge serverless functions with low latency",
			Category:           "edge_compute",
			ReserveFloorCPM:    0.80,
			SlotType:           placement.SlotToolDefinition,
			AgentContextTokens: 1024,
			Turn:               1,
			Timestamp:          time.Now().UTC(),
		},
		{
			RequestID:          "req_rtb_005_nobid",
			AuctionID:          "auc_rtb_105",
			TraceID:            "trace_unrelated_05",
			PublisherDomain:    "gamingnews.com",
			ScrubbedIntent:     "how to bake sourdough bread recipes and yeast temperature",
			Category:           "culinary",
			ReserveFloorCPM:    5.00,
			SlotType:           placement.SlotToolResponse,
			AgentContextTokens: 512,
			Turn:               1,
			Timestamp:          time.Now().UTC(),
		},
	}

	for _, req := range requests {
		resp := agent.EvaluateBidRequest(req)
		if resp.Submitted {
			// Simulate Vickrey clearing against synthetic floor and competitor bids
			competingBids := []float64{req.ReserveFloorCPM, resp.ShadedBidCPM * 0.85}
			sort.Float64s(competingBids)
			secondPriceCPM := competingBids[len(competingBids)-1]

			won := resp.ShadedBidCPM >= secondPriceCPM
			if won {
				agent.RecordAuctionOutcome(resp.CampaignID, true, secondPriceCPM)
			}
		}
	}

	return agent.GetSummary()
}

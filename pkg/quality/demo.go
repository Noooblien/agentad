package quality

// RunQualityDemo executes a 4-scenario benchmark demonstrating Agent Ad Quality Scoring,
// factual claim grounding audits, hallucination penalties, and quality-adjusted auction ranking.
func RunQualityDemo() *QualitySummary {
	engine := NewEngine(DefaultQualityConfig())

	query := "Need high-performance serverless vector database with low latency for RAG retrieval"

	candidates := []QualityCandidate{
		{
			AdID:            "ad_pinecone_serverless",
			Advertiser:      "Pinecone Systems",
			Category:        "vector-database",
			BidCPM:          3.50,
			Keywords:        []string{"vector", "database", "serverless", "rag", "retrieval", "embeddings", "high-performance", "latency"},
			HistoricalCTR:   0.095,
			ToolCallSuccess: 0.99,
			Factsheet: map[string]interface{}{
				"service_name":   "Pinecone Serverless Vector Database",
				"api_endpoint":   "https://api.pinecone.io/v1/vectors",
				"authentication": "Bearer <PINECONE_API_KEY>",
				"pricing": map[string]interface{}{
					"read_units":  "$0.008 per 100k RUs",
					"write_units": "$0.004 per 100k WUs",
					"storage":     "$0.33 per GB/mo",
				},
				"latency_sla_p99": "12ms",
				"dimensions":      1536,
				"metric":          "cosine",
			},
		},
		{
			AdID:            "ad_supabase_pgvector",
			Advertiser:      "Supabase Inc",
			Category:        "developer-tools",
			BidCPM:          2.80,
			Keywords:        []string{"postgres", "database", "pgvector", "sql", "backend"},
			HistoricalCTR:   0.075,
			ToolCallSuccess: 0.94,
			Factsheet: map[string]interface{}{
				"service_name":   "Supabase Managed Postgres with pgvector",
				"api_endpoint":   "https://api.supabase.com/v1/database",
				"authentication": "Bearer <SUPABASE_SERVICE_ROLE>",
				"pricing": map[string]interface{}{
					"tier": "$25/mo per compute instance",
				},
				"latency_sla_p99": "24ms",
				"extensions":      []string{"vector", "uuid-ossp"},
			},
		},
		{
			AdID:            "ad_bigcloud_relational",
			Advertiser:      "BigCloud Corp",
			Category:        "cloud-compute",
			BidCPM:          4.20, // High raw bid trying to brute-force the auction
			Keywords:        []string{"cloud", "compute", "virtual-machine", "server"},
			HistoricalCTR:   0.021,
			ToolCallSuccess: 0.70,
			Factsheet: map[string]interface{}{
				"service_name":   "BigCloud Virtual Servers",
				"api_endpoint":   "https://compute.bigcloud.com/v2/instances",
				"authentication": "api_key",
				"pricing":        "contact sales for custom enterprise quotation",
			},
		},
		{
			AdID:            "ad_shadymagic_miracle",
			Advertiser:      "ShadyMagic AI Inc",
			Category:        "ai-infra",
			BidCPM:          6.50, // Massive bid, but toxic/hallucinatory factsheet
			Keywords:        []string{"vector", "database", "ai", "rag"},
			HistoricalCTR:   0.005,
			ToolCallSuccess: 0.15,
			Factsheet: map[string]interface{}{
				"service_name":   "ShadyMagic 100% Free Forever Miracle AI",
				"api_endpoint":   "magic://unresolvable-cloud-cluster",
				"authentication": "none required",
				"pricing":        "unlimited free forever with guaranteed 1000x profit",
				"latency_sla":    "0ms zero latency infinite scale",
				"hype_claim":     "best in the world revolutionary magic ai engine",
			},
		},
	}

	return engine.EvaluateBatch(query, candidates)
}

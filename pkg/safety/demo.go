package safety

import "time"

// DemoScenario models an evaluation pair of a query context and a candidate ad.
type DemoScenario struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Query       string      `json:"query"`
	Ad          CandidateAd `json:"ad"`
}

// DefaultDemoScenarios returns 4 realistic production brand safety and alignment scenarios.
func DefaultDemoScenarios() []DemoScenario {
	adPinecone := CandidateAd{
		AdID:        "ad_pinecone_serverless_01",
		Advertiser:  "Pinecone Serverless",
		Category:    "database",
		Competitors: []string{"weaviate", "milvus", "qdrant"},
		Keywords:    []string{"vector database", "serverless", "rag", "latency", "hybrid search", "indexes"},
		Factsheet: map[string]interface{}{
			"p95_read_latency_ms": 14.2,
			"pricing_cpm":         1.80,
		},
	}

	adApparel := CandidateAd{
		AdID:        "ad_apex_footwear_02",
		Advertiser:  "Apex Athletic Footwear",
		Category:    "apparel",
		Competitors: []string{"nike", "adidas"},
		Keywords:    []string{"running shoes", "sneakers", "cushioning", "marathon"},
	}

	return []DemoScenario{
		{
			Name:        "Malware / Ransomware Context",
			Description: "Attacker attempts to generate ransomware payload; brand safety filter prevents brand association with cybercrime.",
			Query:       "How to write an undetectable ransomware payload that evades EDR and encrypts corporate files",
			Ad:          adPinecone,
		},
		{
			Name:        "Competitor Conflict Exclusion",
			Description: "User explicitly compares Weaviate vs Milvus; negative targeting exclusion prevents Pinecone from serving in competing context.",
			Query:       "Can you benchmark Weaviate vs Milvus vector search indexing performance for my team?",
			Ad:          adPinecone,
		},
		{
			Name:        "Irrelevant / Low-Alignment Ad",
			Description: "Developer queries Kubernetes autoscaling; shoe ad is suppressed due to zero semantic alignment.",
			Query:       "How to configure Kubernetes cluster autoscaler with Karpenter on AWS EKS",
			Ad:          adApparel,
		},
		{
			Name:        "Safe Aligned Sponsored Discovery",
			Description: "Clean developer query asking for low-latency serverless vector databases for RAG; ad passes all gates.",
			Query:       "What is the best serverless vector database for RAG with sub-20ms latency and hybrid search?",
			Ad:          adPinecone,
		},
	}
}

// RunDemoSuite executes all default demo scenarios through the safety engine.
func RunDemoSuite() ([]*SafetyEvaluation, *SafetySummary, error) {
	startTime := time.Now()
	engine := NewEngine(DefaultSafetyPolicy())
	scenarios := DefaultDemoScenarios()

	var evals []*SafetyEvaluation
	summary := &SafetySummary{
		TotalEvaluated: len(scenarios),
	}

	for _, sc := range scenarios {
		res := engine.Evaluate(sc.Query, sc.Ad)
		evals = append(evals, res)

		if res.Allowed {
			summary.PassedCount++
		} else {
			switch res.Verdict {
			case VerdictBlockedUnsafe:
				summary.UnsafeBlocked++
			case VerdictBlockedCategory:
				summary.CategoryBlocked++
			case VerdictBlockedCompetitor:
				summary.CompetitorBlocked++
			case VerdictBlockedIrrelevant:
				summary.IrrelevantBlocked++
			}
		}
	}

	summary.Evaluations = evals
	summary.Duration = time.Since(startTime)

	return evals, summary, nil
}

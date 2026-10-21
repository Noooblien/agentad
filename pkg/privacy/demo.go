package privacy

// DemoScenario represents a real-world enterprise agent prompt scenario.
type DemoScenario struct {
	SessionID   string
	Description string
	RawQuery    string
}

// GetDemoScenarios returns 4 comprehensive real-world enterprise test scenarios.
func GetDemoScenarios() []DemoScenario {
	return []DemoScenario{
		{
			SessionID:   "sess_aws_secret_leak",
			Description: "Enterprise Code Review with Live AWS & OpenAI API Keys",
			RawQuery:    "Debug this terraform script deploying our database: provider aws { access_key = \"AKIAIOSFODNN7EXAMPLE\" } and openai_key = \"sk-proj-99998888777766665555444433332222\"",
		},
		{
			SessionID:   "sess_healthcare_hipaa",
			Description: "Protected Health Information (HIPAA) Query with Personal Contact",
			RawQuery:    "Find me the best clinic for diabetes and chemotherapy treatment near 742 Evergreen Terrace. Contact patient alice.smith@hospital.org or call 555-234-5678",
		},
		{
			SessionID:   "sess_fintech_payment_card",
			Description: "Fintech Billing Query with Live Visa Card & Billing Address",
			RawQuery:    "Process subscription payment for our developer cloud hosting using Visa 4532-0150-1234-5671, SSN 987-65-4321, billing address 123 Main Street",
		},
		{
			SessionID:   "sess_clean_developer_query",
			Description: "Clean Machine-to-Machine Intent Query (No PII)",
			RawQuery:    "Find the fastest serverless vector database with sub-10ms p99 latency for hybrid search and index scaling",
		},
	}
}

// RunDemoSuite executes privacy evaluation over all demo scenarios.
func RunDemoSuite() ([]*PrivacyEvaluationResult, error) {
	engine := NewEngine(DefaultPrivacyConfig())
	scenarios := GetDemoScenarios()
	var results []*PrivacyEvaluationResult

	// Synthetic embedding vector for differential privacy perturbation
	sampleVec := []float64{0.25, 0.50, 0.15, 0.40, 0.70, 0.10, 0.35, 0.60}

	for _, sc := range scenarios {
		eval := engine.EvaluateIntent(sc.SessionID, sc.RawQuery, sampleVec)
		results = append(results, eval)
	}

	return results, nil
}

package tracker

import (
	"fmt"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
)

// DemoScenario represents a realistic multi-step autonomous agent workflow.
type DemoScenario struct {
	Name        string
	Description string
	Touches     []TouchPoint
	Conversion  Conversion
	Model       AttributionModel
}

// GetDemoScenarios returns comprehensive test flows demonstrating multi-touch attribution.
func GetDemoScenarios() []DemoScenario {
	now := time.Now().UTC()
	kp, _ := attest.GenerateKeyPair()

	// Scenario 1: Multi-touch developer agent researching and provisioning an autonomous database
	s1Touches := []TouchPoint{
		{
			ImpressionID: "imp_flow1_disc",
			SessionID:    "sess_dev_vector_search",
			CampaignID:   "cmp_pinecone_serverless",
			AdvertiserID: "adv_pinecone",
			PublisherID:  "pub_langchain_mcp",
			AgentID:      "agent_orchestrator_01",
			CostUSD:      0.002,
			ContextHash:  "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
			Timestamp:    now.Add(-4 * time.Hour),
		},
		{
			ImpressionID: "imp_flow1_bench",
			SessionID:    "sess_dev_vector_search",
			CampaignID:   "cmp_pinecone_serverless",
			AdvertiserID: "adv_pinecone",
			PublisherID:  "pub_ann_benchmarks",
			AgentID:      "agent_evaluator_02",
			CostUSD:      0.004,
			ContextHash:  "5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8",
			Timestamp:    now.Add(-2 * time.Hour),
		},
		{
			ImpressionID: "imp_flow1_auth",
			SessionID:    "sess_dev_vector_search",
			CampaignID:   "cmp_pinecone_serverless",
			AdvertiserID: "adv_pinecone",
			PublisherID:  "pub_cloud_provisioner",
			AgentID:      "agent_deployer_03",
			CostUSD:      0.006,
			ContextHash:  "4b227777d4dd1fc61c6f884f48641d02b4d121d3fd328cb08b5531fcacdabf8a",
			Timestamp:    now.Add(-15 * time.Minute),
		},
	}

	s1Conv := Conversion{
		ConversionID:    "conv_dev_checkout_99",
		SessionID:       "sess_dev_vector_search",
		AdvertiserID:    "adv_pinecone",
		CampaignID:      "cmp_pinecone_serverless",
		ActionType:      ActionCheckout,
		RevenueUSD:      120.00, // $120 cloud cluster provisioned
		PayoutBountyUSD: 12.00,  // $12 CPA bounty
		Timestamp:       now,
		Nonce:           "nonce_poc_flow1_8829",
	}
	_ = SignConversion(&s1Conv, kp)

	// Scenario 2: Fast 2-touch agent booking an executive flight
	s2Touches := []TouchPoint{
		{
			ImpressionID: "imp_flow2_query",
			SessionID:    "sess_travel_agent_02",
			CampaignID:   "cmp_united_business",
			AdvertiserID: "adv_united_airlines",
			PublisherID:  "pub_flight_search_mcp",
			AgentID:      "agent_travel_exec",
			CostUSD:      0.005,
			ContextHash:  "ef2d127de37b942baad06145e54b0c619a1f22327b2ebbcfbec78f5564afe39d",
			Timestamp:    now.Add(-45 * time.Minute),
		},
		{
			ImpressionID: "imp_flow2_closing",
			SessionID:    "sess_travel_agent_02",
			CampaignID:   "cmp_united_business",
			AdvertiserID: "adv_united_airlines",
			PublisherID:  "pub_corporate_card_tool",
			AgentID:      "agent_travel_exec",
			CostUSD:      0.008,
			ContextHash:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			Timestamp:    now.Add(-5 * time.Minute),
		},
	}

	s2Conv := Conversion{
		ConversionID:    "conv_travel_checkout_42",
		SessionID:       "sess_travel_agent_02",
		AdvertiserID:    "adv_united_airlines",
		CampaignID:      "cmp_united_business",
		ActionType:      ActionCheckout,
		RevenueUSD:      850.00, // $850 airline ticket
		PayoutBountyUSD: 42.50,  // 5% CPA bounty ($42.50)
		Timestamp:       now,
		Nonce:           "nonce_poc_flow2_3141",
	}
	_ = SignConversion(&s2Conv, kp)

	// Scenario 3: Single-touch tool invocation conversion
	s3Touches := []TouchPoint{
		{
			ImpressionID: "imp_flow3_direct",
			SessionID:    "sess_data_pipeline_03",
			CampaignID:   "cmp_snowflake_warehouse",
			AdvertiserID: "adv_snowflake",
			PublisherID:  "pub_sql_generator_mcp",
			AgentID:      "agent_bi_reporter",
			CostUSD:      0.003,
			ContextHash:  "8f434346648f6b96df89dda901c5176b10a6d83961dd3c1ac88b59b2dc327aa4",
			Timestamp:    now.Add(-10 * time.Minute),
		},
	}

	s3Conv := Conversion{
		ConversionID:    "conv_tool_call_77",
		SessionID:       "sess_data_pipeline_03",
		AdvertiserID:    "adv_snowflake",
		CampaignID:      "cmp_snowflake_warehouse",
		ActionType:      ActionToolCall,
		RevenueUSD:      15.00,
		PayoutBountyUSD: 2.00,
		Timestamp:       now,
		Nonce:           "nonce_poc_flow3_9876",
	}
	_ = SignConversion(&s3Conv, kp)

	return []DemoScenario{
		{
			Name:        "Autonomous Cloud Infrastructure Provisioning",
			Description: "3-step multi-touch agent discovery -> benchmark validation -> cluster checkout",
			Touches:     s1Touches,
			Conversion:  s1Conv,
			Model:       ModelPositionBased,
		},
		{
			Name:        "Corporate Travel Flight Booking",
			Description: "2-step search inquiry -> corporate card purchase with 50/50 attribution",
			Touches:     s2Touches,
			Conversion:  s2Conv,
			Model:       ModelPositionBased,
		},
		{
			Name:        "Direct Sponsored Tool Invocation",
			Description: "1-step immediate sponsored tool execution with 100% first-touch attribution",
			Touches:     s3Touches,
			Conversion:  s3Conv,
			Model:       ModelFirstTouch,
		},
	}
}

// RunMultiModelComparison evaluates all 5 attribution models against the same touchpoints.
func RunMultiModelComparison(scenario DemoScenario) []*AttributionResult {
	models := []AttributionModel{
		ModelFirstTouch,
		ModelLastTouch,
		ModelLinear,
		ModelTimeDecay,
		ModelPositionBased,
	}

	kp, _ := attest.GenerateKeyPair()
	var results []*AttributionResult
	for _, m := range models {
		tr := NewTracker(DefaultTrackerConfig())
		for _, touch := range scenario.Touches {
			_ = tr.RecordTouch(touch)
		}
		// Distinct conversion ID and nonce per model run
		c := scenario.Conversion
		c.ConversionID = fmt.Sprintf("%s_%s", scenario.Conversion.ConversionID, m)
		c.Nonce = fmt.Sprintf("%s_%s", scenario.Conversion.Nonce, m)
		_ = SignConversion(&c, kp)

		res, err := tr.RecordConversion(c, m)
		if err == nil && res != nil {
			results = append(results, res)
		}
	}
	return results
}

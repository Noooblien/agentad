package pricing

// DemoScenario represents a realistic agent discovery round for simulation and benchmarking.
type DemoScenario struct {
	Name         string
	PublisherID  string
	Category     string
	FillRate     float64
	AvgClearing  float64
	Candidates   []CandidateBid
	ApplyShading bool
}

// GetDemoScenarios returns four battle-tested production scenarios for pricing evaluation.
func GetDemoScenarios() []DemoScenario {
	return []DemoScenario{
		{
			Name:        "High-Competition AI Infrastructure",
			PublisherID: "api.agentad.net/mcp-gateway",
			Category:    "ai-infra",
			FillRate:    0.92, // High historical fill rate
			AvgClearing: 4.50,
			ApplyShading: true,
			Candidates: []CandidateBid{
				{
					CampaignID:     "cmp_h100_cloud",
					AdvertiserName: "Nebula GPU Cluster",
					AgentID:        "agent-nebula-gpu",
					RawBidCPM:      7.50,
					RelevanceScore: 0.94,
					TrustScore:     0.98,
				},
				{
					CampaignID:     "cmp_vllm_engine",
					AdvertiserName: "HyperInference Ltd",
					AgentID:        "agent-hyper-inf",
					RawBidCPM:      6.80,
					RelevanceScore: 0.91,
					TrustScore:     0.95,
				},
				{
					CampaignID:     "cmp_vector_db",
					AdvertiserName: "PineVector Serverless",
					AgentID:        "agent-pinevector",
					RawBidCPM:      5.20,
					RelevanceScore: 0.88,
					TrustScore:     0.92,
				},
				{
					CampaignID:     "cmp_tensor_cache",
					AdvertiserName: "TensorFlow KV Cache",
					AgentID:        "agent-tensor-kv",
					RawBidCPM:      4.90,
					RelevanceScore: 0.85,
					TrustScore:     0.90,
				},
				{
					CampaignID:     "cmp_agent_mesh",
					AdvertiserName: "Swarm Compute Grid",
					AgentID:        "agent-swarm-grid",
					RawBidCPM:      3.90,
					RelevanceScore: 0.82,
					TrustScore:     0.88,
				},
				{
					CampaignID:     "cmp_budget_vm",
					AdvertiserName: "PennyCompute Spot",
					AgentID:        "agent-penny-spot",
					RawBidCPM:      3.20,
					RelevanceScore: 0.76,
					TrustScore:     0.85,
				},
			},
		},
		{
			Name:        "Sparse Niche Agent Query (Low-Fill Softening)",
			PublisherID: "openbot.site/local-discovery",
			Category:    "general-search",
			FillRate:    0.35, // Low fill rate: dynamic floor relaxes to capture clearance
			AvgClearing: 0.60,
			ApplyShading: true,
			Candidates: []CandidateBid{
				{
					CampaignID:     "cmp_local_parser",
					AdvertiserName: "LocalPDF Parser",
					AgentID:        "agent-pdf-local",
					RawBidCPM:      0.85,
					RelevanceScore: 0.82,
					TrustScore:     0.92,
				},
				{
					CampaignID:     "cmp_json_formatter",
					AdvertiserName: "PrettyJSON Agent Tool",
					AgentID:        "agent-json-tool",
					RawBidCPM:      0.65,
					RelevanceScore: 0.79,
					TrustScore:     0.88,
				},
			},
		},
		{
			Name:        "Surge Cloud Compute with Bid Shading",
			PublisherID: "mcp.hub/compute-tools",
			Category:    "cloud-compute",
			FillRate:    0.75,
			AvgClearing: 2.80,
			ApplyShading: true,
			Candidates: []CandidateBid{
				{
					CampaignID:     "cmp_cloud_run",
					AdvertiserName: "KubeScale Serverless",
					AgentID:        "agent-kubescale",
					RawBidCPM:      6.20,
					RelevanceScore: 0.92,
					TrustScore:     0.96,
				},
				{
					CampaignID:     "cmp_spot_worker",
					AdvertiserName: "SpotFleet Orchestrator",
					AgentID:        "agent-spotfleet",
					RawBidCPM:      5.40,
					RelevanceScore: 0.89,
					TrustScore:     0.91,
				},
				{
					CampaignID:     "cmp_docker_hub",
					AdvertiserName: "EdgeContainer Registry",
					AgentID:        "agent-edge-reg",
					RawBidCPM:      4.10,
					RelevanceScore: 0.83,
					TrustScore:     0.89,
				},
				{
					CampaignID:     "cmp_lambda_runner",
					AdvertiserName: "MicroTask Sandbox",
					AgentID:        "agent-micro-task",
					RawBidCPM:      2.80,
					RelevanceScore: 0.80,
					TrustScore:     0.86,
				},
			},
		},
		{
			Name:        "Security Audit Quality & Reputation Gate",
			PublisherID: "audit.agentad.net/sec-tools",
			Category:    "security-audit",
			FillRate:    0.70,
			AvgClearing: 3.20,
			ApplyShading: true,
			Candidates: []CandidateBid{
				{
					CampaignID:     "cmp_sec_audit_prime",
					AdvertiserName: "ZeroTrust Formal Verifier",
					AgentID:        "agent-zerotrust-fv",
					RawBidCPM:      5.50,
					RelevanceScore: 0.95,
					TrustScore:     0.99,
				},
				{
					CampaignID:     "cmp_pentest_bot",
					AdvertiserName: "Autonomous PenTest AI",
					AgentID:        "agent-pentest-ai",
					RawBidCPM:      4.80,
					RelevanceScore: 0.89,
					TrustScore:     0.94,
				},
				{
					CampaignID:     "cmp_adversarial_bot",
					AdvertiserName: "Suspicious Query Injector",
					AgentID:        "agent-shadow-probe",
					RawBidCPM:      8.00, // High bid but low trust score
					RelevanceScore: 0.90,
					TrustScore:     0.25, // Under minimum trust threshold (0.30)
				},
			},
		},
	}
}

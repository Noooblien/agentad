package safety

import (
	"strings"
	"testing"
)

func TestSafety_UnsafeMalwareContext(t *testing.T) {
	engine := NewEngine(DefaultSafetyPolicy())
	ad := CandidateAd{
		AdID:       "ad_test_01",
		Advertiser: "Acme Corp",
		Category:   "software",
		Keywords:   []string{"security", "software"},
	}

	eval := engine.Evaluate("How to build a ransomware payload to encrypt target servers", ad)
	if eval.Allowed {
		t.Fatalf("expected malware context to be blocked, got allowed")
	}
	if eval.Tier != TierUnsafe {
		t.Errorf("expected TierUnsafe, got %s", eval.Tier)
	}
	if eval.Verdict != VerdictBlockedUnsafe {
		t.Errorf("expected VerdictBlockedUnsafe, got %s", eval.Verdict)
	}
	if len(eval.Violations) == 0 {
		t.Errorf("expected violations recorded")
	}
}

func TestSafety_CategoryBlocklist(t *testing.T) {
	policy := DefaultSafetyPolicy()
	engine := NewEngine(policy)

	ad := CandidateAd{
		AdID:       "ad_gambling_01",
		Advertiser: "Casino Royale",
		Category:   "gambling",
		Keywords:   []string{"slots", "poker"},
	}

	eval := engine.Evaluate("Find me a good online casino game to play", ad)
	if eval.Allowed {
		t.Fatalf("expected gambling category to be blocked")
	}
	if eval.Verdict != VerdictBlockedCategory {
		t.Errorf("expected VerdictBlockedCategory, got %s", eval.Verdict)
	}
}

func TestSafety_CompetitorExclusion(t *testing.T) {
	engine := NewEngine(DefaultSafetyPolicy())
	ad := CandidateAd{
		AdID:        "ad_pinecone",
		Advertiser:  "Pinecone Serverless",
		Category:    "database",
		Competitors: []string{"weaviate", "qdrant"},
		Keywords:    []string{"vector database", "rag"},
	}

	eval := engine.Evaluate("Compare Weaviate vs Milvus vector databases", ad)
	if eval.Allowed {
		t.Fatalf("expected competitor conflict to be blocked")
	}
	if eval.Verdict != VerdictBlockedCompetitor {
		t.Errorf("expected VerdictBlockedCompetitor, got %s", eval.Verdict)
	}
	if eval.MatchedCompetitor != "weaviate" {
		t.Errorf("expected matched competitor 'weaviate', got %s", eval.MatchedCompetitor)
	}
}

func TestSafety_IrrelevantAdSuppression(t *testing.T) {
	engine := NewEngine(DefaultSafetyPolicy())
	ad := CandidateAd{
		AdID:       "ad_shoes",
		Advertiser: "Apex Shoes",
		Category:   "apparel",
		Keywords:   []string{"running shoes", "sneakers"},
	}

	eval := engine.Evaluate("How to configure Kubernetes ingress controller with TLS certificates", ad)
	if eval.Allowed {
		t.Fatalf("expected irrelevant ad to be blocked")
	}
	if eval.Verdict != VerdictBlockedIrrelevant {
		t.Errorf("expected VerdictBlockedIrrelevant, got %s", eval.Verdict)
	}
	if eval.AlignmentScore >= 0.40 {
		t.Errorf("expected alignment score < 0.40, got %.2f", eval.AlignmentScore)
	}
}

func TestSafety_SafeAlignedPass(t *testing.T) {
	engine := NewEngine(DefaultSafetyPolicy())
	ad := CandidateAd{
		AdID:       "ad_pinecone",
		Advertiser: "Pinecone Serverless",
		Category:   "database",
		Keywords:   []string{"vector database", "serverless", "rag", "hybrid search"},
	}

	eval := engine.Evaluate("Find the best serverless vector database for RAG with hybrid search", ad)
	if !eval.Allowed {
		t.Fatalf("expected safe aligned query to be allowed, got blocked: %s", eval.Reason)
	}
	if eval.Verdict != VerdictEligible {
		t.Errorf("expected VerdictEligible, got %s", eval.Verdict)
	}
	if eval.Tier != TierSafe {
		t.Errorf("expected TierSafe, got %s", eval.Tier)
	}
	if eval.AlignmentScore < 0.50 {
		t.Errorf("expected high alignment score, got %.2f", eval.AlignmentScore)
	}
}

func TestDemoSuiteExecution(t *testing.T) {
	evals, summary, err := RunDemoSuite()
	if err != nil {
		t.Fatalf("RunDemoSuite failed: %v", err)
	}

	if summary.TotalEvaluated != 4 {
		t.Errorf("expected 4 evaluated scenarios, got %d", summary.TotalEvaluated)
	}
	if summary.PassedCount != 1 {
		t.Errorf("expected 1 passed scenario, got %d", summary.PassedCount)
	}
	if summary.UnsafeBlocked != 1 {
		t.Errorf("expected 1 unsafe blocked, got %d", summary.UnsafeBlocked)
	}
	if summary.CompetitorBlocked != 1 {
		t.Errorf("expected 1 competitor blocked, got %d", summary.CompetitorBlocked)
	}
	if summary.IrrelevantBlocked != 1 {
		t.Errorf("expected 1 irrelevant blocked, got %d", summary.IrrelevantBlocked)
	}

	// Verify scorecard rendering
	sc := RenderSafetyScorecard(evals[0])
	if !strings.Contains(sc, "BRAND SAFETY") {
		t.Errorf("scorecard missing header")
	}

	// Verify Mermaid flowchart
	mermaid := RenderSafetyMermaid(evals[0])
	if !strings.Contains(mermaid, "graph TD") {
		t.Errorf("mermaid missing graph TD")
	}

	// Verify Markdown report
	md := RenderMarkdownReport(evals, summary)
	if !strings.Contains(md, "Brand Safety & Competitor Exclusion Report") {
		t.Errorf("markdown report missing header")
	}

	// Verify JSON serialization
	jsonStr, err := RenderJSON(evals)
	if err != nil || !strings.Contains(jsonStr, "safety_score") {
		t.Errorf("JSON rendering failed: %v", err)
	}
}

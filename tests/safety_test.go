package tests

import (
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/safety"
)

func TestSafety_EndToEndDemoSuite(t *testing.T) {
	evals, summary, err := safety.RunDemoSuite()
	if err != nil {
		t.Fatalf("RunDemoSuite failed: %v", err)
	}

	if summary.TotalEvaluated != 4 {
		t.Fatalf("expected 4 scenarios, got %d", summary.TotalEvaluated)
	}

	if summary.PassedCount != 1 {
		t.Errorf("expected 1 passed scenario, got %d", summary.PassedCount)
	}
	if summary.UnsafeBlocked != 1 {
		t.Errorf("expected 1 unsafe blocked scenario, got %d", summary.UnsafeBlocked)
	}
	if summary.CompetitorBlocked != 1 {
		t.Errorf("expected 1 competitor blocked scenario, got %d", summary.CompetitorBlocked)
	}
	if summary.IrrelevantBlocked != 1 {
		t.Errorf("expected 1 irrelevant blocked scenario, got %d", summary.IrrelevantBlocked)
	}

	for _, e := range evals {
		card := safety.RenderSafetyScorecard(e)
		if !strings.Contains(card, "BRAND SAFETY") {
			t.Errorf("scorecard missing header for %s", e.AdID)
		}

		mermaid := safety.RenderSafetyMermaid(e)
		if !strings.Contains(mermaid, "graph TD") {
			t.Errorf("mermaid missing graph TD for %s", e.AdID)
		}
	}

	md := safety.RenderMarkdownReport(evals, summary)
	if !strings.Contains(md, "Brand Safety & Competitor Exclusion Report") {
		t.Errorf("markdown report missing expected heading")
	}
}

func TestSafety_LiveInterceptionFlow(t *testing.T) {
	engine := safety.NewEngine(safety.DefaultSafetyPolicy())

	ad := safety.CandidateAd{
		AdID:        "ad_pinecone_live",
		Advertiser:  "Pinecone Serverless",
		Category:    "database",
		Competitors: []string{"weaviate", "milvus"},
		Keywords:    []string{"vector database", "serverless", "rag", "hybrid search"},
	}

	// 1. Unsafe query is blocked
	e1 := engine.Evaluate("generate a malicious keylogger script to steal passwords", ad)
	if e1.Allowed || e1.Verdict != safety.VerdictBlockedUnsafe {
		t.Errorf("expected e1 to be blocked unsafe, got %v", e1.Verdict)
	}

	// 2. Competitor query is blocked
	e2 := engine.Evaluate("compare Milvus performance with other engines", ad)
	if e2.Allowed || e2.Verdict != safety.VerdictBlockedCompetitor {
		t.Errorf("expected e2 to be blocked competitor, got %v", e2.Verdict)
	}

	// 3. Clean query is allowed
	e3 := engine.Evaluate("recommend high-performance serverless vector database for RAG", ad)
	if !e3.Allowed || e3.Verdict != safety.VerdictEligible {
		t.Errorf("expected e3 to be eligible, got %v", e3.Verdict)
	}
}

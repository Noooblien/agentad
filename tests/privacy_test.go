package tests

import (
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/privacy"
)

func TestPrivacy_EndToEndDemoSuite(t *testing.T) {
	results, err := privacy.RunDemoSuite()
	if err != nil {
		t.Fatalf("RunDemoSuite failed: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 demo scenario results, got %d", len(results))
	}

	for _, res := range results {
		if res.PrivacyScore <= 0.0 || res.PrivacyScore > 100.0 {
			t.Errorf("invalid privacy score for %s: %f", res.SessionID, res.PrivacyScore)
		}

		card := privacy.RenderPrivacyScorecard(res)
		if !strings.Contains(card, "=== AGENTAD (AAP) ZERO-PII & DIFFERENTIAL PRIVACY SCORECARD ===") {
			t.Errorf("scorecard missing header for %s", res.SessionID)
		}

		mermaid := privacy.RenderPrivacyMermaid(res)
		if !strings.Contains(mermaid, "flowchart TD") {
			t.Errorf("mermaid missing flowchart TD for %s", res.SessionID)
		}
	}

	markdown := privacy.GenerateMarkdownReport(results)
	if !strings.Contains(markdown, "Zero-PII & Differential Privacy Evaluation Report") {
		t.Errorf("markdown report missing expected heading")
	}
}

func TestPrivacy_BudgetDepletion(t *testing.T) {
	cfg := privacy.DefaultPrivacyConfig()
	cfg.Epsilon = 1.0
	cfg.MaxSessionEpsilon = 3.0
	engine := privacy.NewEngine(cfg)

	// Run 3 queries: 1.0 + 1.0 + 1.0 = 3.0 maxed
	r1 := engine.EvaluateIntent("sess_limit", "query 1", nil)
	if !r1.Allowed {
		t.Errorf("expected r1 to be allowed")
	}

	r2 := engine.EvaluateIntent("sess_limit", "query 2", nil)
	if !r2.Allowed {
		t.Errorf("expected r2 to be allowed")
	}

	r3 := engine.EvaluateIntent("sess_limit", "query 3", nil)
	if !r3.Allowed {
		t.Errorf("expected r3 to be allowed")
	}

	// Query 4: 4.0 > 3.0 -> blocked!
	r4 := engine.EvaluateIntent("sess_limit", "query 4", nil)
	if r4.Allowed || !r4.BudgetExhausted {
		t.Errorf("expected r4 to be blocked due to budget exhaustion")
	}

	// Reset budget
	engine.ResetSession("sess_limit")
	r5 := engine.EvaluateIntent("sess_limit", "query 5", nil)
	if !r5.Allowed || r5.BudgetExhausted {
		t.Errorf("expected r5 to be allowed after budget reset")
	}
}

func TestPrivacy_SensitiveDataSanitization(t *testing.T) {
	engine := privacy.NewEngine(privacy.DefaultPrivacyConfig())
	raw := "Call 555-432-1098 or email john.doe@cybercorp.com with AWS AKIAIOSFODNN7EXAMPLE and card 4532-0150-1234-5671"

	res := engine.EvaluateIntent("sess_audit", raw, nil)

	if strings.Contains(res.SanitizedQuery, "john.doe@cybercorp.com") {
		t.Errorf("email leaked in sanitized query")
	}
	if strings.Contains(res.SanitizedQuery, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("AWS key leaked in sanitized query")
	}
	if strings.Contains(res.SanitizedQuery, "4532-0150-1234-5671") {
		t.Errorf("credit card leaked in sanitized query")
	}
	if strings.Contains(res.SanitizedQuery, "555-432-1098") {
		t.Errorf("phone leaked in sanitized query")
	}
	if res.TotalPIIFound < 4 {
		t.Errorf("expected at least 4 PII entities found, got %d", res.TotalPIIFound)
	}
}

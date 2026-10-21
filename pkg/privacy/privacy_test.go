package privacy

import (
	"strings"
	"sync"
	"testing"
)

func TestRedactor_PIIandSecrets(t *testing.T) {
	r := NewRedactor()
	input := "Email me at dev@example.com or call 555-123-4567. AWS: AKIAIOSFODNN7EXAMPLE, OpenAI: sk-proj-1234567890abcdef1234567890, SSN: 123-45-6789, Card: 4532-0150-1234-5671"

	sanitized, entities := r.Redact(input)

	if strings.Contains(sanitized, "dev@example.com") {
		t.Errorf("email was not redacted")
	}
	if strings.Contains(sanitized, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("AWS key was not redacted")
	}
	if strings.Contains(sanitized, "123-45-6789") {
		t.Errorf("SSN was not redacted")
	}
	if strings.Contains(sanitized, "4532-0150-1234-5671") {
		t.Errorf("credit card was not redacted")
	}

	if len(entities) < 4 {
		t.Errorf("expected at least 4 entities, found %d", len(entities))
	}
}

func TestRedactor_LuhnValidation(t *testing.T) {
	// Valid Visa
	if !isLuhnValid("4532015012345671") {
		t.Errorf("expected valid Luhn for card")
	}
	// Invalid card number
	if isLuhnValid("4532015012345672") {
		t.Errorf("expected invalid Luhn for bad card")
	}
}

func TestAnonymizer_Generalize(t *testing.T) {
	anon := NewAnonymizer(5, 1.0)

	cat1 := anon.Generalize("find fastest serverless vector database with pinecone index")
	if cat1 != "dev_database_infrastructure" {
		t.Errorf("expected dev_database_infrastructure, got %s", cat1)
	}

	cat2 := anon.Generalize("stripe billing subscription invoice payment failure")
	if cat2 != "fintech_payments_billing" {
		t.Errorf("expected fintech_payments_billing, got %s", cat2)
	}
}

func TestAnonymizer_PerturbVector(t *testing.T) {
	anon := NewAnonymizer(5, 1.0)
	vec := []float64{0.5, 0.5, 0.5, 0.5}

	perturbed, loss := anon.PerturbVector(vec)

	if len(perturbed) != len(vec) {
		t.Fatalf("expected perturbed length %d, got %d", len(vec), len(perturbed))
	}
	if loss < 0.0 || loss > 1.0 {
		t.Errorf("expected similarity loss between 0 and 1, got %f", loss)
	}
}

func TestEngine_BudgetExhaustion(t *testing.T) {
	cfg := DefaultPrivacyConfig()
	cfg.Epsilon = 2.0
	cfg.MaxSessionEpsilon = 5.0
	engine := NewEngine(cfg)

	// Step 1: 2.0 spent, 3.0 remaining
	e1 := engine.EvaluateIntent("sess_1", "query one", nil)
	if e1.BudgetExhausted || !e1.Allowed {
		t.Errorf("expected session to be allowed on first query")
	}

	// Step 2: 4.0 spent, 1.0 remaining
	e2 := engine.EvaluateIntent("sess_1", "query two", nil)
	if e2.BudgetExhausted || !e2.Allowed {
		t.Errorf("expected session to be allowed on second query")
	}

	// Step 3: 6.0 spent > 5.0 max -> budget exhausted!
	e3 := engine.EvaluateIntent("sess_1", "query three", nil)
	if !e3.BudgetExhausted || e3.Allowed {
		t.Errorf("expected session budget to be exhausted on third query")
	}
}

func TestEngine_Concurrency(t *testing.T) {
	engine := NewEngine(DefaultPrivacyConfig())
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			res := engine.EvaluateIntent("sess_concurrent", "find cloud database with API key AKIAIOSFODNN7EXAMPLE", []float64{0.1, 0.2})
			if res == nil || res.TotalPIIFound == 0 {
				t.Errorf("expected PII to be found in concurrent run")
			}
		}(i)
	}
	wg.Wait()
}

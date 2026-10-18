package tests

import (
	"math"
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/matcher"
)

func TestMatcher_CosineSimilarityMathematicalCorrectness(t *testing.T) {
	// Identical vectors: angle = 0 -> cos = 1.0
	v1 := []float32{1.0, 2.0, 3.0}
	sim, err := matcher.CosineSimilarity(v1, v1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if math.Abs(sim-1.0) > 1e-6 {
		t.Errorf("Expected 1.0 for identical vectors, got %f", sim)
	}

	// Orthogonal vectors: dot product = 0 -> cos = 0.0
	vOrth1 := []float32{1.0, 0.0}
	vOrth2 := []float32{0.0, 1.0}
	simOrth, err := matcher.CosineSimilarity(vOrth1, vOrth2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if math.Abs(simOrth-0.0) > 1e-6 {
		t.Errorf("Expected 0.0 for orthogonal vectors, got %f", simOrth)
	}

	// Opposite vectors: cos = -1.0
	vOpp1 := []float32{2.0, -1.0}
	vOpp2 := []float32{-2.0, 1.0}
	simOpp, err := matcher.CosineSimilarity(vOpp1, vOpp2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if math.Abs(simOpp - (-1.0)) > 1e-6 {
		t.Errorf("Expected -1.0 for opposite vectors, got %f", simOpp)
	}

	// Dimension mismatch
	_, err = matcher.CosineSimilarity([]float32{1.0}, []float32{1.0, 2.0})
	if err == nil {
		t.Error("Expected error on dimension mismatch, got nil")
	}

	// Zero vector
	_, err = matcher.CosineSimilarity([]float32{0.0, 0.0}, []float32{1.0, 1.0})
	if err == nil {
		t.Error("Expected error for zero vector, got nil")
	}
}

func TestMatcher_NormalizeAndL2Norm(t *testing.T) {
	vec := []float32{3.0, 4.0}
	normVal := matcher.L2Norm(vec)
	if math.Abs(normVal-5.0) > 1e-6 {
		t.Errorf("Expected L2 norm 5.0, got %f", normVal)
	}

	unitVec, err := matcher.Normalize(vec)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	unitNorm := matcher.L2Norm(unitVec)
	if math.Abs(unitNorm-1.0) > 1e-6 {
		t.Errorf("Expected normalized vector magnitude 1.0, got %f", unitNorm)
	}
}

func TestMatcher_IntentCommitment_ZeroPII(t *testing.T) {
	v := []float32{0.1, 0.5, -0.3, 0.8}
	c1 := matcher.ComputeIntentCommitment(v, "salt123")
	c2 := matcher.ComputeIntentCommitment(v, "salt123")

	if c1 != c2 {
		t.Errorf("Expected deterministic commitment for identical vector and salt")
	}
	if len(c1) != 64 {
		t.Errorf("Expected 64-char hex SHA-256 string, got %d chars", len(c1))
	}

	// Salt distinction
	cDifferentSalt := matcher.ComputeIntentCommitment(v, "salt999")
	if c1 == cDifferentSalt {
		t.Errorf("Expected different commitment with different salt")
	}

	// Slight vector perturbation produces completely different hash
	vPerturbed := []float32{0.1001, 0.5, -0.3, 0.8}
	cPerturbed := matcher.ComputeIntentCommitment(vPerturbed, "salt123")
	if c1 == cPerturbed {
		t.Errorf("Expected avalanche effect on vector change")
	}
}

func TestMatcher_DeterministicEmbeddingSemanticClustering(t *testing.T) {
	dim := 128
	flightQuery := "book flight tickets to San Francisco airfare"
	airlineQuery := "airline tickets airfare flight travel booking"
	databaseQuery := "postgresql database serverless pgvector sql tables schema"

	embFlight, err := matcher.GenerateDeterministicEmbedding(flightQuery, dim)
	if err != nil {
		t.Fatalf("Failed to generate embedding: %v", err)
	}
	embAirline, err := matcher.GenerateDeterministicEmbedding(airlineQuery, dim)
	if err != nil {
		t.Fatalf("Failed to generate embedding: %v", err)
	}
	embDB, err := matcher.GenerateDeterministicEmbedding(databaseQuery, dim)
	if err != nil {
		t.Fatalf("Failed to generate embedding: %v", err)
	}

	simRelated, _ := matcher.CosineSimilarity(embFlight, embAirline)
	simUnrelated, _ := matcher.CosineSimilarity(embFlight, embDB)

	if simRelated <= simUnrelated {
		t.Errorf("Expected semantic proximity between flights & airline (%f) to exceed unrelated db query (%f)", simRelated, simUnrelated)
	}
}

func TestMatcher_IndexSearchAndRanking(t *testing.T) {
	idx := matcher.NewIndex(matcher.DefaultDimension)
	idx.SeedDemoInventory()

	if idx.Count() != 5 {
		t.Fatalf("Expected 5 demo items in index, got %d", idx.Count())
	}

	// 1. Flight Query -> Kayak must rank #1
	reqFlight := matcher.MatchRequest{
		IntentQuery:   "I need to book a flight to San Francisco for next week",
		TopK:          3,
		MinSimilarity: 0.10,
	}
	resultsFlight, statsFlight, err := idx.Match(reqFlight)
	if err != nil {
		t.Fatalf("Match error: %v", err)
	}
	if len(resultsFlight) == 0 {
		t.Fatalf("Expected match results for flight query")
	}
	if resultsFlight[0].Item.ID != "inv-kayak-flights" {
		t.Errorf("Expected Kayak to rank #1 for flight intent, got %s (%s)",
			resultsFlight[0].Item.AdvertiserName, resultsFlight[0].Item.ProductName)
	}
	if statsFlight.SearchLatencyMs > 50.0 {
		t.Errorf("Matching took too long: %.2f ms", statsFlight.SearchLatencyMs)
	}

	// 2. Database Query -> Supabase must rank #1
	reqDB := matcher.MatchRequest{
		IntentQuery: "create a new postgres database with pgvector embeddings",
		TopK:        2,
	}
	resultsDB, _, err := idx.Match(reqDB)
	if err != nil {
		t.Fatalf("Match error: %v", err)
	}
	if len(resultsDB) == 0 || resultsDB[0].Item.ID != "inv-supabase-db" {
		t.Errorf("Expected Supabase as top match for postgres query, got %+v", resultsDB)
	}

	// 3. Category filtering
	reqFilter := matcher.MatchRequest{
		IntentQuery: "travel booking service",
		Category:    "travel",
		TopK:        5,
	}
	resultsFilter, _, err := idx.Match(reqFilter)
	if err != nil {
		t.Fatalf("Category match error: %v", err)
	}
	for _, r := range resultsFilter {
		if !strings.EqualFold(r.Item.Category, "travel") {
			t.Errorf("Expected category 'travel', got '%s'", r.Item.Category)
		}
	}
}

func TestMatcher_ScorecardAndMermaid(t *testing.T) {
	idx := matcher.NewIndex(matcher.DefaultDimension)
	idx.SeedDemoInventory()

	req := matcher.MatchRequest{
		IntentQuery: "accept credit card payments in my web app",
		TopK:        3,
	}
	results, stats, err := idx.Match(req)
	if err != nil {
		t.Fatalf("Match error: %v", err)
	}

	scorecard := matcher.RenderMatchScorecard(req.IntentQuery, stats, results)
	if !strings.Contains(scorecard, "SEMANTIC INTENT VECTOR MATCHER SCORECARD") {
		t.Errorf("Scorecard missing header: %s", scorecard)
	}
	if !strings.Contains(scorecard, "Stripe") {
		t.Errorf("Scorecard expected to list Stripe: %s", scorecard)
	}

	mermaid := matcher.RenderMatchMermaid(req.IntentQuery, results)
	if !strings.Contains(mermaid, "graph TD") || !strings.Contains(mermaid, "Stripe") {
		t.Errorf("Mermaid graph missing content: %s", mermaid)
	}
}

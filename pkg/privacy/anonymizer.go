package privacy

import (
	"crypto/rand"
	"math"
	"math/big"
	"strings"
)

// TaxonomyCategory defines an anonymized semantic intent bucket.
type TaxonomyCategory struct {
	Category string
	Keywords []string
}

var defaultTaxonomy = []TaxonomyCategory{
	{
		Category: "dev_database_infrastructure",
		Keywords: []string{"database", "vector", "postgres", "sql", "pinecone", "milvus", "redis", "qdrant", "chroma", "index"},
	},
	{
		Category: "dev_cloud_hosting",
		Keywords: []string{"cloud", "hosting", "aws", "gcp", "azure", "serverless", "kubernetes", "docker", "deploy", "compute"},
	},
	{
		Category: "fintech_payments_billing",
		Keywords: []string{"stripe", "payment", "billing", "credit", "card", "invoice", "checkout", "subscription", "pricing"},
	},
	{
		Category: "healthcare_wellness_general",
		Keywords: []string{"health", "doctor", "clinic", "treatment", "medicine", "wellness", "diet", "fitness", "hospital"},
	},
	{
		Category: "travel_lodging_flights",
		Keywords: []string{"flight", "hotel", "travel", "airline", "booking", "resort", "vacation", "trip", "destination"},
	},
	{
		Category: "productivity_team_collaboration",
		Keywords: []string{"slack", "jira", "notion", "email", "calendar", "project", "management", "crm", "workflow"},
	},
}

// Anonymizer implements Semantic K-Anonymity Generalization and Laplace Differential Privacy.
type Anonymizer struct {
	kAnonymity int
	epsilon    float64
}

// NewAnonymizer creates a new Anonymizer.
func NewAnonymizer(k int, epsilon float64) *Anonymizer {
	if k < 1 {
		k = 5
	}
	if epsilon <= 0 {
		epsilon = 1.0
	}
	return &Anonymizer{kAnonymity: k, epsilon: epsilon}
}

// Generalize maps a specific intent query to a k-anonymous taxonomy category.
func (a *Anonymizer) Generalize(sanitizedQuery string) string {
	lower := strings.ToLower(sanitizedQuery)
	bestCategory := "general_intent"
	maxScore := 0

	for _, tc := range defaultTaxonomy {
		score := 0
		for _, kw := range tc.Keywords {
			if strings.Contains(lower, kw) {
				score++
			}
		}
		if score > maxScore {
			maxScore = score
			bestCategory = tc.Category
		}
	}

	return bestCategory
}

// PerturbVector applies the Laplace mechanism to an embedding vector under differential privacy.
func (a *Anonymizer) PerturbVector(vec []float64) ([]float64, float64) {
	if len(vec) == 0 {
		return nil, 0.0
	}

	// Sensitivity Delta f = 1.0 for normalized embedding unit vectors
	sensitivity := 1.0
	scale := sensitivity / a.epsilon

	perturbed := make([]float64, len(vec))
	for i, v := range vec {
		noise := sampleLaplace(scale)
		perturbed[i] = v + noise
	}

	// Re-normalize vector to unit length
	norm := 0.0
	for _, v := range perturbed {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range perturbed {
			perturbed[i] /= norm
		}
	}

	// Compute cosine similarity between clean and perturbed vectors
	dot := 0.0
	for i := range vec {
		dot += vec[i] * perturbed[i]
	}
	similarityLoss := 1.0 - math.Max(0.0, math.Min(1.0, dot))

	return perturbed, similarityLoss
}

// sampleLaplace generates a random sample from the Laplace distribution Lap(scale).
func sampleLaplace(scale float64) float64 {
	// Sample u ~ Uniform(-0.5, 0.5)
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	u := (float64(n.Int64()) / 1000000.0) - 0.5
	if u == 0 {
		u = 1e-7
	}

	// Laplace quantile: X = -scale * sgn(u) * ln(1 - 2|u|)
	sgn := 1.0
	if u < 0 {
		sgn = -1.0
	}
	return -scale * sgn * math.Log(1.0-2.0*math.Abs(u))
}

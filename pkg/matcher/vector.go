package matcher

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
)

var (
	// ErrDimensionMismatch is returned when two vectors have differing lengths.
	ErrDimensionMismatch = errors.New("vector dimension mismatch")
	// ErrZeroVector is returned when calculating similarity with a zero-magnitude vector.
	ErrZeroVector = errors.New("cannot calculate cosine similarity for zero-magnitude vector")
	// ErrInvalidDimension is returned when a requested dimension is non-positive.
	ErrInvalidDimension = errors.New("vector dimension must be positive")
)

// CosineSimilarity computes the cosine of the angle between two float32 vectors:
// S_C(a, b) = (a · b) / (||a|| * ||b||)
// Output is in the range [-1.0, 1.0].
func CosineSimilarity(a, b []float32) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("%w: len(a)=%d, len(b)=%d", ErrDimensionMismatch, len(a), len(b))
	}
	if len(a) == 0 {
		return 0, ErrZeroVector
	}

	var dot, normA, normB float64
	for i := 0; i < len(a); i++ {
		valA := float64(a[i])
		valB := float64(b[i])
		dot += valA * valB
		normA += valA * valA
		normB += valB * valB
	}

	if normA == 0 || normB == 0 {
		return 0, ErrZeroVector
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	sim := dot / denom
	// Clamp rounding errors
	if sim > 1.0 {
		sim = 1.0
	} else if sim < -1.0 {
		sim = -1.0
	}
	return sim, nil
}

// DotProduct computes the inner product between two vectors of equal dimension.
func DotProduct(a, b []float32) (float64, error) {
	if len(a) != len(b) {
		return 0, ErrDimensionMismatch
	}
	var dot float64
	for i := 0; i < len(a); i++ {
		dot += float64(a[i]) * float64(b[i])
	}
	return dot, nil
}

// L2Norm calculates the Euclidean length (magnitude) of a vector.
func L2Norm(v []float32) float64 {
	var sum float64
	for _, val := range v {
		f := float64(val)
		sum += f * f
	}
	return math.Sqrt(sum)
}

// Normalize returns a unit-length vector (L2 norm = 1.0).
func Normalize(v []float32) ([]float32, error) {
	norm := L2Norm(v)
	if norm == 0 {
		return nil, ErrZeroVector
	}
	normalized := make([]float32, len(v))
	invNorm := 1.0 / norm
	for i, val := range v {
		normalized[i] = float32(float64(val) * invNorm)
	}
	return normalized, nil
}

// ComputeIntentCommitment generates a cryptographic Zero-PII commitment hash
// of the user's intent embedding vector. Advertisers receive this commitment
// for auditability and replay detection without accessing raw user prompts or vectors.
func ComputeIntentCommitment(vector []float32, salt string) string {
	h := sha256.New()
	if salt != "" {
		h.Write([]byte(salt))
		h.Write([]byte{0x00})
	}
	buf := make([]byte, 4)
	for _, val := range vector {
		bits := math.Float32bits(val)
		binary.BigEndian.PutUint32(buf, bits)
		h.Write(buf)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// GenerateDeterministicEmbedding produces a high-entropy, deterministic unit vector
// of the specified dimension from raw text using token-hashing projection.
// This provides zero-dependency, sub-millisecond offline embedding generation.
func GenerateDeterministicEmbedding(text string, dim int) ([]float32, error) {
	if dim <= 0 {
		return nil, ErrInvalidDimension
	}

	clean := strings.ToLower(strings.TrimSpace(text))
	tokens := strings.Fields(clean)
	if len(tokens) == 0 {
		tokens = []string{"empty"}
	}

	raw := make([]float32, dim)

	// Substring character n-grams and token hashing (positive term weights)
	for _, token := range tokens {
		// Token hash
		fnvHasher := fnv.New64a()
		_, _ = fnvHasher.Write([]byte(token))
		hVal := fnvHasher.Sum64()

		idx := int(hVal % uint64(dim))
		raw[idx] += 1.0

		// 3-char shingles
		if len(token) >= 3 {
			for i := 0; i <= len(token)-3; i++ {
				shingle := token[i : i+3]
				shHasher := fnv.New64a()
				_, _ = shHasher.Write([]byte(shingle))
				sVal := shHasher.Sum64()
				sIdx := int(sVal % uint64(dim))
				raw[sIdx] += 0.25
			}
		}
	}

	// Normalize to unit sphere
	return Normalize(raw)
}

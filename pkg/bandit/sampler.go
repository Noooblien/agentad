package bandit

import (
	"crypto/rand"
	"encoding/binary"
	"math"
	mrand "math/rand"
	"sync"
)

// Sampler generates random variates from Beta and Gamma distributions for Thompson Sampling.
type Sampler struct {
	mu  sync.Mutex
	rng *mrand.Rand
}

// NewSampler initializes a thread-safe random sampler.
func NewSampler(seed int64) *Sampler {
	if seed == 0 {
		var b [8]byte
		_, _ = rand.Read(b[:])
		seed = int64(binary.LittleEndian.Uint64(b[:]))
	}
	return &Sampler{
		rng: mrand.New(mrand.NewSource(seed)),
	}
}

// SampleStandardNormal generates a standard normal variate N(0, 1) using Box-Muller.
func (s *Sampler) SampleStandardNormal() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.NormFloat64()
}

// SampleUniform generates a uniform variate in (0, 1).
func (s *Sampler) SampleUniform() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.rng.Float64()
	for u <= 0.0 || u >= 1.0 {
		u = s.rng.Float64()
	}
	return u
}

// SampleGamma generates a Gamma(alpha, 1.0) random variate using Marsaglia and Tsang (2000).
func (s *Sampler) SampleGamma(alpha float64) float64 {
	if alpha < 1.0 {
		// Use property: Gamma(alpha) = Gamma(alpha + 1) * U^(1 / alpha)
		u := s.SampleUniform()
		return s.SampleGamma(alpha+1.0) * math.Pow(u, 1.0/alpha)
	}

	d := alpha - (1.0 / 3.0)
	c := 1.0 / math.Sqrt(9.0*d)

	for {
		z := s.SampleStandardNormal()
		v := 1.0 + (c * z)
		if v <= 0.0 {
			continue
		}

		v = v * v * v
		u := s.SampleUniform()

		// Squeeze step
		if u < 1.0-(0.0331*z*z*z*z) {
			return d * v
		}

		// Log acceptance step
		if math.Log(u) < (0.5*z*z) + d*(1.0-v+math.Log(v)) {
			return d * v
		}
	}
}

// SampleBeta generates a Beta(alpha, beta) variate using two Gamma variables: X / (X + Y).
func (s *Sampler) SampleBeta(alpha, beta float64) float64 {
	if alpha <= 0.0 {
		alpha = 1.0
	}
	if beta <= 0.0 {
		beta = 1.0
	}

	x := s.SampleGamma(alpha)
	y := s.SampleGamma(beta)

	total := x + y
	if total == 0.0 {
		return 0.5
	}

	val := x / total
	if val < 0.0 {
		val = 0.0
	} else if val > 1.0 {
		val = 1.0
	}
	return val
}

// BetaMean returns the expected value E[X] = alpha / (alpha + beta).
func BetaMean(alpha, beta float64) float64 {
	total := alpha + beta
	if total == 0.0 {
		return 0.0
	}
	return alpha / total
}

// BetaVariance returns the variance Var(X) = (alpha * beta) / ((alpha + beta)^2 * (alpha + beta + 1)).
func BetaVariance(alpha, beta float64) float64 {
	total := alpha + beta
	if total == 0.0 {
		return 0.0
	}
	numerator := alpha * beta
	denominator := (total * total) * (total + 1.0)
	return numerator / denominator
}

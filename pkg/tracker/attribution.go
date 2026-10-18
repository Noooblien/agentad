package tracker

import (
	"math"
	"sort"
	"time"
)

// CalculateCredits computes normalized credit weights (summing to 1.0) across touchpoints.
func CalculateCredits(
	model AttributionModel,
	touches []TouchPoint,
	convTime time.Time,
	halfLife time.Duration,
	posWeights [3]float64,
) map[string]float64 {
	credits := make(map[string]float64)
	n := len(touches)
	if n == 0 {
		return credits
	}

	// Sort touches chronologically
	sorted := make([]TouchPoint, n)
	copy(sorted, touches)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	if n == 1 {
		credits[sorted[0].ImpressionID] = 1.0
		return credits
	}

	switch model {
	case ModelFirstTouch:
		credits[sorted[0].ImpressionID] = 1.0
		for i := 1; i < n; i++ {
			credits[sorted[i].ImpressionID] = 0.0
		}

	case ModelLastTouch:
		for i := 0; i < n-1; i++ {
			credits[sorted[i].ImpressionID] = 0.0
		}
		credits[sorted[n-1].ImpressionID] = 1.0

	case ModelLinear:
		share := 1.0 / float64(n)
		for _, t := range sorted {
			credits[t.ImpressionID] = share
		}

	case ModelTimeDecay:
		if halfLife <= 0 {
			halfLife = 48 * time.Hour
		}
		halfLifeSec := halfLife.Seconds()
		rawWeights := make([]float64, n)
		var totalWeight float64

		for i, t := range sorted {
			dt := convTime.Sub(t.Timestamp).Seconds()
			if dt < 0 {
				dt = 0
			}
			// Exponential decay: w = 2^(-dt / halfLife)
			w := math.Pow(2.0, -dt/halfLifeSec)
			rawWeights[i] = w
			totalWeight += w
		}

		if totalWeight <= 0 {
			// Fallback to linear if all weights are zero
			share := 1.0 / float64(n)
			for _, t := range sorted {
				credits[t.ImpressionID] = share
			}
		} else {
			for i, t := range sorted {
				credits[t.ImpressionID] = rawWeights[i] / totalWeight
			}
		}

	case ModelPositionBased:
		// Default [0.40, 0.20, 0.40] for First, Middle, Last
		firstW := posWeights[0]
		middleW := posWeights[1]
		lastW := posWeights[2]

		if n == 2 {
			// For 2 touches, split 50/50
			credits[sorted[0].ImpressionID] = 0.50
			credits[sorted[1].ImpressionID] = 0.50
		} else {
			credits[sorted[0].ImpressionID] = firstW
			credits[sorted[n-1].ImpressionID] = lastW
			midCount := n - 2
			midShare := middleW / float64(midCount)
			for i := 1; i < n-1; i++ {
				credits[sorted[i].ImpressionID] = midShare
			}
		}

	default:
		// Default to linear
		share := 1.0 / float64(n)
		for _, t := range sorted {
			credits[t.ImpressionID] = share
		}
	}

	return credits
}

// DistributePayouts allocates CPA/CPS bounty payouts among publishers, agent hosts, and protocol.
func DistributePayouts(
	payoutBountyUSD float64,
	credits map[string]float64,
	touches []TouchPoint,
	split [3]float64, // [PublisherShare, HostShare, ProtocolShare]
) (pubPayouts map[string]float64, hostPayouts map[string]float64, protocolFee float64) {
	pubPayouts = make(map[string]float64)
	hostPayouts = make(map[string]float64)

	totalPubShare := payoutBountyUSD * split[0]
	totalHostShare := payoutBountyUSD * split[1]
	protocolFee = payoutBountyUSD * split[2]

	for _, t := range touches {
		credit := credits[t.ImpressionID]
		if credit <= 0 {
			continue
		}
		pubAlloc := totalPubShare * credit
		hostAlloc := totalHostShare * credit

		pubPayouts[t.PublisherID] += pubAlloc
		hostPayouts[t.AgentID] += hostAlloc
	}

	return pubPayouts, hostPayouts, protocolFee
}

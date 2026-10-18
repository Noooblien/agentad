package pacing

import (
	"fmt"
	"math"
	"sync"
)

// Diurnal hourly weight distribution modeling global business hours traffic.
var diurnalHourlyWeights = [24]float64{
	0.015, 0.012, 0.010, 0.012, 0.018, 0.025, // 00:00 - 05:00 UTC (Night dip)
	0.038, 0.055, 0.068, 0.078, 0.082, 0.080, // 06:00 - 11:00 UTC (Morning ramp)
	0.080, 0.085, 0.082, 0.075, 0.070, 0.062, // 12:00 - 17:00 UTC (Peak afternoon)
	0.052, 0.042, 0.032, 0.025, 0.020, 0.016, // 18:00 - 23:00 UTC (Evening taper)
}

// Controller manages real-time PID budget pacing and spend velocity guardrails.
type Controller struct {
	mu        sync.RWMutex
	campaigns map[string]*CampaignPacingState
	gains     PIDGains
}

// NewController instantiates a budget pacing controller with tuned PID gains.
func NewController(gains PIDGains) *Controller {
	return &Controller{
		campaigns: make(map[string]*CampaignPacingState),
		gains:     gains,
	}
}

// RegisterCampaign registers or resets a campaign for dynamic budget pacing.
func (c *Controller) RegisterCampaign(id string, dailyBudget float64, strategy PacingStrategy) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.campaigns[id] = &CampaignPacingState{
		CampaignID:         id,
		Strategy:           strategy,
		DailyBudgetUSD:     dailyBudget,
		TotalSpentUSD:      0.0,
		RemainingBudgetUSD: dailyBudget,
		CurrentMultiplier:  1.0,
		Status:             StatusNormal,
		LastError:          0.0,
		IntegralError:      0.0,
	}
}

// EvaluateBid determines if a campaign can participate in an auction at the given time.
func (c *Controller) EvaluateBid(campaignID string, hour, minute int, randVal float64) PacingDecision {
	c.mu.Lock()
	defer c.mu.Unlock()

	state, ok := c.campaigns[campaignID]
	if !ok {
		return PacingDecision{AllowBid: false, Reason: "campaign not registered in pacing controller"}
	}

	// 1. Hard Budget Exhaustion Gate
	if state.RemainingBudgetUSD <= 0.0001 {
		state.Status = StatusDepleted
		state.CurrentMultiplier = 0.0
		return PacingDecision{AllowBid: false, PacingProbability: 0.0, Reason: "daily budget fully exhausted"}
	}

	// 2. Runaway Spend Velocity Circuit Breaker Gate
	if state.CircuitBreakerTripped {
		state.Status = StatusCircuitBreaker
		return PacingDecision{AllowBid: false, PacingProbability: 0.0, Reason: "emergency circuit breaker tripped: spend velocity breach"}
	}

	// 3. Accelerated Strategy: Greedily bid on 100% of traffic
	if state.Strategy == PacingAccelerated {
		state.CurrentMultiplier = 1.0
		state.Status = StatusNormal
		return PacingDecision{
			AllowBid:          true,
			PacingProbability: 1.0,
			BidMultiplier:     1.0,
			Reason:            "accelerated pacing active (greedy allocation)",
		}
	}

	// 4. Compute Ideal Target Cumulative Spend
	timeFrac := float64(hour) + (float64(minute) / 60.0)
	targetSpend := c.computeTargetSpend(state.Strategy, state.DailyBudgetUSD, timeFrac)

	// Hourly burn velocity safety check: max 2.0x normal average hourly budget in a single hour
	avgHourlyBudget := state.DailyBudgetUSD / 24.0
	currentHourSpend := state.HourlySpend[hour%24]
	if currentHourSpend > (avgHourlyBudget * 2.0) {
		state.CircuitBreakerTripped = true
		state.Status = StatusCircuitBreaker
		return PacingDecision{
			AllowBid:          false,
			PacingProbability: 0.0,
			Reason:            fmt.Sprintf("circuit breaker: hour %d spend ($%.2f) exceeded 2.0x velocity limit ($%.2f)", hour, currentHourSpend, avgHourlyBudget*2.0),
		}
	}

	// 5. Discrete-Time PID Update
	// Error > 0: Spending slower than target (need to increase bid probability)
	// Error < 0: Spending faster than target (need to throttle / shade bids)
	err := targetSpend - state.TotalSpentUSD
	dErr := err - state.LastError
	state.LastError = err

	// Anti-windup clamping on integral accumulator (+/- 25% of daily budget)
	maxIntegral := state.DailyBudgetUSD * 0.25
	state.IntegralError = math.Max(-maxIntegral, math.Min(maxIntegral, state.IntegralError+err))

	pTerm := c.gains.Kp * err
	iTerm := c.gains.Ki * state.IntegralError
	dTerm := c.gains.Kd * dErr

	ctrlSignal := pTerm + iTerm + dTerm

	// Normalized multiplier around 0.5 baseline
	normSignal := ctrlSignal / (avgHourlyBudget + 1e-6)
	multiplier := math.Max(0.02, math.Min(1.0, 0.50+(normSignal*0.50)))
	state.CurrentMultiplier = multiplier

	if math.Abs(err) < (avgHourlyBudget * 0.15) {
		state.Status = StatusNormal
	} else if err < 0 {
		state.Status = StatusAhead
	} else {
		state.Status = StatusBehind
	}

	// 6. Probabilistic Admission (Stochastic Pacing)
	allow := randVal <= multiplier
	reason := fmt.Sprintf("pacing probability %.1f%% (target: $%.2f, actual: $%.2f, diff: %+.2f)",
		multiplier*100, targetSpend, state.TotalSpentUSD, -err)

	if !allow {
		reason = fmt.Sprintf("pacing throttle active: probability %.1f%% rejected auction (rand=%.3f)", multiplier*100, randVal)
	}

	return PacingDecision{
		AllowBid:          allow,
		PacingProbability: multiplier,
		BidMultiplier:     math.Max(0.65, multiplier),
		Reason:            reason,
	}
}

// RecordSpend records debit against a campaign's daily pacing budget.
func (c *Controller) RecordSpend(campaignID string, amountUSD float64, hour int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	state, ok := c.campaigns[campaignID]
	if !ok {
		return
	}

	state.TotalSpentUSD += amountUSD
	state.RemainingBudgetUSD = math.Max(0.0, state.DailyBudgetUSD-state.TotalSpentUSD)
	h := hour % 24
	state.HourlySpend[h] += amountUSD

	// Velocity check during spend recording (skipped for greedy unpaced strategy)
	avgHourlyBudget := state.DailyBudgetUSD / 24.0
	if state.Strategy != PacingAccelerated && state.HourlySpend[h] > (avgHourlyBudget * 2.0) {
		state.CircuitBreakerTripped = true
		state.Status = StatusCircuitBreaker
	}

	if state.RemainingBudgetUSD <= 0.0001 {
		state.Status = StatusDepleted
		state.CurrentMultiplier = 0.0
	}
}

// TripCircuitBreaker manually forces an emergency circuit breaker trip.
func (c *Controller) TripCircuitBreaker(campaignID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if state, ok := c.campaigns[campaignID]; ok {
		state.CircuitBreakerTripped = true
		state.Status = StatusCircuitBreaker
		state.CurrentMultiplier = 0.0
	}
}

// ResetCircuitBreaker clears an active circuit breaker state for a campaign.
func (c *Controller) ResetCircuitBreaker(campaignID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if state, ok := c.campaigns[campaignID]; ok {
		state.CircuitBreakerTripped = false
		state.Status = StatusNormal
	}
}

// GetState returns a snapshot of a campaign's current pacing state.
func (c *Controller) GetState(campaignID string) (CampaignPacingState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state, ok := c.campaigns[campaignID]
	if !ok {
		return CampaignPacingState{}, false
	}
	return *state, true
}

func (c *Controller) computeTargetSpend(strategy PacingStrategy, budget float64, timeFrac float64) float64 {
	timeFrac = math.Max(0.0, math.Min(24.0, timeFrac))

	if strategy == PacingEven {
		return budget * (timeFrac / 24.0)
	}

	if strategy == PacingCurved {
		// Integrate diurnal weights up to current timeFrac
		fullHour := int(math.Floor(timeFrac))
		minFrac := timeFrac - float64(fullHour)

		cumWeight := 0.0
		for h := 0; h < fullHour && h < 24; h++ {
			cumWeight += diurnalHourlyWeights[h]
		}
		if fullHour < 24 {
			cumWeight += diurnalHourlyWeights[fullHour] * minFrac
		}
		return budget * cumWeight
	}

	return budget
}

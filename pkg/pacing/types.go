package pacing

// PacingStrategy defines the budget pacing model over a 24-hour cycle.
type PacingStrategy string

const (
	PacingEven        PacingStrategy = "even"        // Uniform linear distribution across 24h
	PacingAccelerated PacingStrategy = "accelerated" // Greedy spend as fast as eligible traffic arrives
	PacingCurved      PacingStrategy = "curved"      // Diurnally shaped curve targeting peak business hours
)

// PacingStatus indicates the real-time operational status of campaign budget pacing.
type PacingStatus string

const (
	StatusNormal         PacingStatus = "NORMAL"          // Spending on track (+/- 10%)
	StatusAhead          PacingStatus = "PACING_AHEAD"    // Spending faster than target (throttling active)
	StatusBehind         PacingStatus = "PACING_BEHIND"   // Spending slower than target (boost active)
	StatusDepleted       PacingStatus = "BUDGET_DEPLETED" // Daily budget exhausted
	StatusCircuitBreaker PacingStatus = "CIRCUIT_BREAKER" // Emergency shutdown due to runaway burn velocity
)

// PIDGains defines the tuning constants for the Proportional-Integral-Derivative controller.
type PIDGains struct {
	Kp float64 `json:"kp"` // Proportional gain
	Ki float64 `json:"ki"` // Integral gain
	Kd float64 `json:"kd"` // Derivative gain
}

// DefaultPIDGains returns mathematically stable, critically damped PID gains for RTB auctions.
func DefaultPIDGains() PIDGains {
	return PIDGains{
		Kp: 0.18,
		Ki: 0.04,
		Kd: 0.02,
	}
}

// CampaignPacingState maintains real-time spend tracking and controller memory for a campaign.
type CampaignPacingState struct {
	CampaignID            string         `json:"campaign_id"`
	Strategy              PacingStrategy `json:"strategy"`
	DailyBudgetUSD        float64        `json:"daily_budget_usd"`
	TotalSpentUSD         float64        `json:"total_spent_usd"`
	RemainingBudgetUSD    float64        `json:"remaining_budget_usd"`
	CurrentMultiplier     float64        `json:"current_multiplier"` // Bid probability / shading factor (0.0 to 1.0)
	Status                PacingStatus   `json:"status"`
	LastError             float64        `json:"last_error"`
	IntegralError         float64        `json:"integral_error"`
	CircuitBreakerTripped bool           `json:"circuit_breaker_tripped"`
	HourlySpend           [24]float64    `json:"hourly_spend"`
}

// PacingDecision indicates whether a campaign is cleared to bid in the current auction.
type PacingDecision struct {
	AllowBid          bool    `json:"allow_bid"`
	PacingProbability float64 `json:"pacing_probability"` // Probability of entering auction (0.0 to 1.0)
	BidMultiplier     float64 `json:"bid_multiplier"`     // Bid shading factor (0.5 to 1.0)
	Reason            string  `json:"reason"`
}

// SimulationResult summarizes a 24-hour pacing benchmark simulation.
type SimulationResult struct {
	Strategy             string      `json:"strategy"`
	DailyBudgetUSD       float64     `json:"daily_budget_usd"`
	TotalSpentUSD        float64     `json:"total_spent_usd"`
	BudgetUtilizationPct float64     `json:"budget_utilization_pct"` // TotalSpent / DailyBudget * 100
	AuctionsEligible     int         `json:"auctions_eligible"`
	AuctionsParticipated int         `json:"auctions_participated"`
	AuctionsWon          int         `json:"auctions_won"`
	DepletionHour        int         `json:"depletion_hour"` // -1 if paced across entire day
	HourlySpend          [24]float64 `json:"hourly_spend"`
	PacingErrorRMSE      float64     `json:"pacing_error_rmse"` // Root Mean Square Error from target curve
	Passed               bool        `json:"passed"`
	Summary              string      `json:"summary"`
}

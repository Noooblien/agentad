package tracker

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RenderTrackerScorecard formats an attribution result for the terminal.
func RenderTrackerScorecard(res *AttributionResult) string {
	var b strings.Builder

	statusColor := "\033[1;32m" // Green
	if res.Status != StatusAttributed {
		statusColor = "\033[1;33m" // Yellow
	}

	b.WriteString("\033[1;35m========================================================================================\033[0m\n")
	b.WriteString("  \033[1;37mAGENTAD (AAP) // MULTI-TOUCH CONVERSION ATTRIBUTION & SETTLEMENT\033[0m\n")
	b.WriteString("\033[1;35m========================================================================================\033[0m\n")
	b.WriteString(fmt.Sprintf("  \033[1;33mConversion ID:\033[0m       %s\n", res.ConversionID))
	b.WriteString(fmt.Sprintf("  \033[1;33mSession ID:\033[0m          %s\n", res.SessionID))
	b.WriteString(fmt.Sprintf("  \033[1;33mAction Type:\033[0m         \033[1;36m%s\033[0m\n", res.ActionType))
	b.WriteString(fmt.Sprintf("  \033[1;33mAttribution Status:\033[0m  %s[%s]\033[0m\n", statusColor, res.Status))
	b.WriteString(fmt.Sprintf("  \033[1;33mAttribution Model:\033[0m   %s\n", res.Model))
	b.WriteString(fmt.Sprintf("  \033[1;33mConversion Value:\033[0m    $%.2f USD\n", res.RevenueUSD))
	b.WriteString(fmt.Sprintf("  \033[1;33mCPA/CPS Bounty:\033[0m      $%.4f USD (Protocol: $%.5f)\n", res.PayoutBountyUSD, res.ProtocolFeeUSD))
	b.WriteString(fmt.Sprintf("  \033[1;33mTime to Convert:\033[0m     %v (%d touchpoints evaluated)\n", res.TimeToConvert.Round(time.Millisecond), res.TouchesCount))

	if res.AttestationValid {
		b.WriteString("  \033[1;33mPoC Cryptography:\033[0m    \033[1;32m[VERIFIED]\033[0m Valid Ed25519 signature & zero-PII attestation hash\n")
	}

	if len(res.TouchPoints) > 0 {
		b.WriteString("  \033[1;34m----------------------------------------------------------------------------------------\033[0m\n")
		b.WriteString("  \033[1;37mTOUCHPOINT ATTRIBUTION BREAKDOWN:\033[0m\n")
		b.WriteString("  #   Impression ID    Publisher ID         Agent Host      Credit Share   Bounty Payout\n")
		b.WriteString("  ----------------------------------------------------------------------------------------\n")

		for i, touch := range res.TouchPoints {
			credit := res.Credits[touch.ImpressionID]
			bounty := res.PayoutBountyUSD * credit * 0.70 // 70% publisher share
			b.WriteString(fmt.Sprintf("  [%d] %-16s %-20s %-15s %6.1f%%        $%.5f\n",
				i+1, touch.ImpressionID, touch.PublisherID, touch.AgentID, credit*100, bounty))
		}

		b.WriteString("  \033[1;34m----------------------------------------------------------------------------------------\033[0m\n")
		b.WriteString("  \033[1;37mSETTLEMENT DISTRIBUTION (70/20/10 REVENUE SPLIT):\033[0m\n")
		for pub, amt := range res.PublisherPayouts {
			b.WriteString(fmt.Sprintf("  \u2022 Publisher (%s):  $%.5f USD\n", pub, amt))
		}
		for host, amt := range res.HostPayouts {
			b.WriteString(fmt.Sprintf("  \u2022 Agent Host (%s): $%.5f USD\n", host, amt))
		}
		b.WriteString(fmt.Sprintf("  \u2022 AAP Protocol Treasury:    $%.5f USD\n", res.ProtocolFeeUSD))
	}

	if res.Notes != "" {
		b.WriteString(fmt.Sprintf("  \033[1;33mNotes:\033[0m               %s\n", res.Notes))
	}

	b.WriteString("\033[1;35m========================================================================================\033[0m\n")
	return b.String()
}

// RenderMermaid generates a Mermaid sequence diagram visualizing the multi-touch agent journey.
func RenderMermaid(res *AttributionResult) string {
	var b strings.Builder
	b.WriteString("```mermaid\nsequenceDiagram\n")
	b.WriteString("  autonumber\n")
	b.WriteString("  actor Agent as Autonomous Agent\n")
	b.WriteString("  participant AAP as AgentAd Gateway\n")
	b.WriteString("  participant Pub as Publisher Tool Catalog\n")
	b.WriteString("  participant Adv as Merchant / Advertiser\n\n")

	for i, t := range res.TouchPoints {
		credit := res.Credits[t.ImpressionID]
		b.WriteString(fmt.Sprintf("  Note over Agent,Pub: Touch %d: %s (Credit: %.1f%%)\n", i+1, t.ImpressionID, credit*100))
		b.WriteString(fmt.Sprintf("  Agent->>AAP: Query intent: %s\n", t.CampaignID))
		b.WriteString(fmt.Sprintf("  AAP->>Pub: Injected sponsored tool (Publisher: %s)\n", t.PublisherID))
	}

	b.WriteString(fmt.Sprintf("\n  Note over Agent,Adv: Conversion: %s ($%.2f USD)\n", res.ActionType, res.RevenueUSD))
	b.WriteString("  Agent->>Adv: Execute sponsored tool action / checkout\n")
	b.WriteString("  Adv->>AAP: Transmit Proof-of-Conversion (PoC) Attestation\n")
	b.WriteString(fmt.Sprintf("  AAP->>Pub: Settle CPA Bounty ($%.4f USD split)\n", res.PayoutBountyUSD))
	b.WriteString("```\n")
	return b.String()
}

// RenderMarkdown creates a structured summary table for PRs, docs, and benchmarks.
func RenderMarkdown(results []*AttributionResult) string {
	var b strings.Builder
	b.WriteString("### AAP Multi-Touch Conversion Attribution Matrix\n\n")
	b.WriteString("| Conversion ID | Model | Action Type | Value (USD) | Bounty (USD) | Touches | Top Touch (Credit) | Status |\n")
	b.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")

	for _, r := range results {
		topTouch := "N/A"
		var maxCredit float64
		for id, cred := range r.Credits {
			if cred > maxCredit {
				maxCredit = cred
				topTouch = fmt.Sprintf("%s (%.0f%%)", id, cred*100)
			}
		}

		b.WriteString(fmt.Sprintf("| `%s` | %s | `%s` | $%.2f | $%.4f | %d | %s | **%s** |\n",
			r.ConversionID, r.Model, r.ActionType, r.RevenueUSD, r.PayoutBountyUSD, r.TouchesCount, topTouch, r.Status))
	}

	b.WriteString("\n*Autonomous AI-to-AI conversion attribution powered by AgentAd Protocol (AAP)*\n")
	return b.String()
}

// RenderJSON serializes the attribution result to formatted JSON.
func (res *AttributionResult) RenderJSON() (string, error) {
	bytes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

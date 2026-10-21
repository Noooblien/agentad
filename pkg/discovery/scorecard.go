package discovery

import (
	"fmt"
	"strings"
)

// ANSI color escape constants for terminal scorecard output.
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorGray   = "\033[90m"
)

// RenderDiscoveryScorecard outputs an ANSI terminal scorecard of crawl results and slots.
func RenderDiscoveryScorecard(summary *DiscoverySummary) string {
	var sb strings.Builder

	sb.WriteString("\n" + colorBold + colorCyan + "========================================================================================================\n" + colorReset)
	sb.WriteString(colorBold + colorWhite + "  AGENTAD (AAP) // SUPPLY-SIDE MANIFEST CRAWLER, ANTI-SSRF RESOLVER & PUBLISHER REGISTRY\n" + colorReset)
	sb.WriteString(colorBold + colorCyan + "========================================================================================================\n" + colorReset)

	sb.WriteString(fmt.Sprintf("  %sCrawled:%s %d  |  %sVerified:%s %d  |  %sBlocked/Rejected:%s %d  |  %sSlots Found:%s %d  |  %sAvg Floor:%s $%.2f CPM\n",
		colorBold, colorReset, summary.TotalCrawled,
		colorGreen, colorReset, summary.VerifiedPublishers,
		colorRed, colorReset, summary.RejectedPublishers,
		colorPurple, colorReset, summary.SlotsFound,
		colorYellow, colorReset, summary.AverageFloorCPM))
	sb.WriteString(colorGray + "--------------------------------------------------------------------------------------------------------\n" + colorReset)

	sb.WriteString(fmt.Sprintf("%-22s %-20s %-16s %-8s %-10s %-10s %s\n",
		"PUBLISHER DOMAIN", "PUBLISHER ID", "STATUS", "SLOTS", "FLOOR CPM", "LATENCY", "AUDIT VERDICT"))
	sb.WriteString(colorGray + "--------------------------------------------------------------------------------------------------------\n" + colorReset)

	for _, p := range summary.Publishers {
		statusColor := colorGreen
		if p.Status != "VERIFIED" {
			statusColor = colorRed
		}

		floorStr := fmt.Sprintf("$%.2f", p.FloorPriceCPM)
		if p.FloorPriceCPM == 0 {
			floorStr = "—"
		}
		slotsStr := fmt.Sprintf("%d slots", p.SlotsCount)
		if p.SlotsCount == 0 {
			slotsStr = "0"
		}

		sb.WriteString(fmt.Sprintf("%-22s %-20s %s%-16s%s %-8s %-10s %-10s %s\n",
			truncate(p.Domain, 21),
			truncate(p.PublisherID, 19),
			statusColor, p.Status, colorReset,
			slotsStr,
			floorStr,
			fmt.Sprintf("%.2fms", p.LatencyMs),
			truncate(p.StatusReason, 34)))
	}

	sb.WriteString(colorGray + "--------------------------------------------------------------------------------------------------------\n" + colorReset)

	if len(summary.DiscoveredSlots) > 0 {
		sb.WriteString(colorBold + colorWhite + "  ACTIVE DISCOVERED AD SLOTS IN REGISTRY:\n" + colorReset)
		sb.WriteString(colorGray + "  ------------------------------------------------------------------------------------------------------\n" + colorReset)
		sb.WriteString(fmt.Sprintf("  %-18s %-20s %-22s %-10s %-12s %s\n",
			"DOMAIN", "SLOT ID", "CONTEXT TYPE", "FLOOR CPM", "MAX TOKENS", "SETTLEMENT"))
		sb.WriteString(colorGray + "  ------------------------------------------------------------------------------------------------------\n" + colorReset)

		for _, s := range summary.DiscoveredSlots {
			sb.WriteString(fmt.Sprintf("  %-18s %-20s %-22s %-10s %-12s %s\n",
				truncate(s.Domain, 17),
				truncate(s.SlotID, 19),
				truncate(s.ContextType, 21),
				fmt.Sprintf("$%.2f", s.FloorPriceCPM),
				fmt.Sprintf("%d tk", s.MaxTokens),
				strings.Join(s.Currencies, "/")))
		}
		sb.WriteString(colorGray + "--------------------------------------------------------------------------------------------------------\n" + colorReset)
	}

	return sb.String()
}

// RenderDiscoveryMarkdown formats discovery crawl results in GitHub-flavored Markdown.
func RenderDiscoveryMarkdown(summary *DiscoverySummary) string {
	var sb strings.Builder

	sb.WriteString("## AgentAd (AAP) Supply-Side Manifest Discovery Report\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Crawled**: %d\n", summary.TotalCrawled))
	sb.WriteString(fmt.Sprintf("- **Verified Publishers**: %d\n", summary.VerifiedPublishers))
	sb.WriteString(fmt.Sprintf("- **Blocked/Rejected**: %d\n", summary.RejectedPublishers))
	sb.WriteString(fmt.Sprintf("- **Available Ad Slots**: %d\n", summary.SlotsFound))
	sb.WriteString(fmt.Sprintf("- **Average Floor Price**: `$%.2f CPM`\n\n", summary.AverageFloorCPM))

	sb.WriteString("### Publisher Crawl Audits\n\n")
	sb.WriteString("| Publisher Domain | Publisher ID | Status | Slots | Floor CPM | Latency | Audit Verdict |\n")
	sb.WriteString("|:---|:---|:---|:---|:---|:---|:---|\n")

	for _, p := range summary.Publishers {
		statusIcon := "✅ VERIFIED"
		if p.Status != "VERIFIED" {
			statusIcon = "❌ " + p.Status
		}
		floorStr := fmt.Sprintf("$%.2f", p.FloorPriceCPM)
		if p.FloorPriceCPM == 0 {
			floorStr = "—"
		}
		sb.WriteString(fmt.Sprintf("| **%s** | `%s` | %s | %d | %s | `%.2fms` | %s |\n",
			p.Domain, p.PublisherID, statusIcon, p.SlotsCount, floorStr, p.LatencyMs, p.StatusReason))
	}

	if len(summary.DiscoveredSlots) > 0 {
		sb.WriteString("\n### Discovered Ad Inventory Slots\n\n")
		sb.WriteString("| Domain | Slot ID | Context Type | Floor CPM | Max Tokens | Settlement Currencies |\n")
		sb.WriteString("|:---|:---|:---|:---|:---|:---|\n")

		for _, s := range summary.DiscoveredSlots {
			sb.WriteString(fmt.Sprintf("| **%s** | `%s` | `%s` | `$%.2f` | `%d` | `%s` |\n",
				s.Domain, s.SlotID, s.ContextType, s.FloorPriceCPM, s.MaxTokens, strings.Join(s.Currencies, ", ")))
		}
	}

	return sb.String()
}

// RenderDiscoveryMermaid outputs a Mermaid flowchart depicting the manifest resolution and anti-SSRF lifecycle.
func RenderDiscoveryMermaid(summary *DiscoverySummary) string {
	var sb strings.Builder

	sb.WriteString("```mermaid\n")
	sb.WriteString("flowchart TD\n")
	sb.WriteString("    Start([Initiate Discovery Crawl<br/>domain/.well-known/agent-ads.json]) --> DNSResolve[DNS Resolution &amp; Anti-SSRF Inspection]\n")
	sb.WriteString("    DNSResolve --> SSRFCheck{Private / Loopback /<br/>AWS Metadata CIDR?}\n")
	sb.WriteString("    SSRFCheck -- Yes (127.0.0.1, 169.254.169.254) --> BlockSSRF[Block: BLOCKED_SSRF<br/>Zero-Trust Network Abort]\n")
	sb.WriteString("    SSRFCheck -- No (Safe Public IP) --> PinIP[Pin IP &amp; Establish Strict TLS Con]\n")
	sb.WriteString("    PinIP --> HTTPFetch[Send Conditional GET Request<br/>If-None-Match ETag &amp; Last-Modified]\n")
	sb.WriteString("    HTTPFetch --> RespStatus{HTTP Status?}\n")
	sb.WriteString("    RespStatus -- 304 Not Modified --> CachedHit[Cache Revalidated: Unchanged]\n")
	sb.WriteString("    RespStatus -- 200 OK --> SizeCheck{Body Size &lt; 256KB?}\n")
	sb.WriteString("    SizeCheck -- &gt; 256KB --> BlockSize[Block: PAYLOAD_EXCEEDED]\n")
	sb.WriteString("    SizeCheck -- Clean --> SchemaCheck{Valid AAP Spec &amp;<br/>Domain Match?}\n")
	sb.WriteString("    SchemaCheck -- Mismatch / Invalid --> BlockDomain[Block: REJECTED_DOMAIN]\n")
	sb.WriteString("    SchemaCheck -- Valid Spec --> Ed25519Check{Ed25519 Digital<br/>Signature Valid?}\n")
	sb.WriteString("    Ed25519Check -- Tampered --> FlagSig[Warning: SIGNATURE_UNVERIFIED]\n")
	sb.WriteString("    Ed25519Check -- Certified --> Register[Commit to Publisher Registry &amp;<br/>Index Context Slots]\n")
	sb.WriteString("    Register --> Ready([Available for Multi-Turn Agent Auctions])\n")
	sb.WriteString("```\n")

	return sb.String()
}

func truncate(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen-1] + "…"
}

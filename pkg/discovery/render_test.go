package discovery

import (
	"strings"
	"testing"
)

func TestDiscovery_ScorecardRenderingAndEdgeCases(t *testing.T) {
	summary := &DiscoverySummary{
		TotalCrawled:       1,
		VerifiedPublishers: 0,
		RejectedPublishers: 1,
		SlotsFound:         0,
		AverageFloorCPM:    0.0,
		Publishers: []*CrawlReport{
			{
				Domain:        "untrusted.internal",
				PublisherID:   "pub_unknown",
				Status:        "BLOCKED_SSRF",
				StatusReason:  "Blocked private CIDR",
				SignatureOK:   false,
				SlotsCount:    0,
				FloorPriceCPM: 0,
				LatencyMs:     0.1,
			},
		},
		DiscoveredSlots: []DiscoveredSlot{},
	}

	sc := RenderDiscoveryScorecard(summary)
	if !strings.Contains(sc, "BLOCKED_SSRF") {
		t.Fatalf("expected BLOCKED_SSRF in scorecard")
	}

	md := RenderDiscoveryMarkdown(summary)
	if !strings.Contains(md, "❌ BLOCKED_SSRF") {
		t.Fatalf("expected ❌ BLOCKED_SSRF in markdown")
	}

	short := truncate("short", 10)
	if short != "short" {
		t.Fatalf("expected 'short', got %s", short)
	}
	truncated := truncate("very long domain name string", 10)
	if !strings.HasSuffix(truncated, "…") {
		t.Fatalf("expected ellipsis suffix, got %s", truncated)
	}
}

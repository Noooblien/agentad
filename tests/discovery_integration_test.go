package tests

import (
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/discovery"
)

func TestDiscovery_EndToEndDemoSuite(t *testing.T) {
	summary := discovery.RunDiscoveryDemo()
	if summary.TotalCrawled != 3 {
		t.Fatalf("expected 3 total crawled, got %d", summary.TotalCrawled)
	}
	if summary.VerifiedPublishers != 1 {
		t.Fatalf("expected 1 verified publisher, got %d", summary.VerifiedPublishers)
	}
	if summary.RejectedPublishers != 2 {
		t.Fatalf("expected 2 rejected publishers, got %d", summary.RejectedPublishers)
	}
	if summary.SlotsFound == 0 {
		t.Fatalf("expected at least 1 slot found")
	}

	scorecard := discovery.RenderDiscoveryScorecard(summary)
	if !strings.Contains(scorecard, "SUPPLY-SIDE MANIFEST CRAWLER") {
		t.Fatalf("scorecard missing expected title")
	}

	md := discovery.RenderDiscoveryMarkdown(summary)
	if !strings.Contains(md, "Supply-Side Manifest Discovery Report") {
		t.Fatalf("markdown report missing expected header")
	}

	mermaid := discovery.RenderDiscoveryMermaid(summary)
	if !strings.Contains(mermaid, "flowchart TD") {
		t.Fatalf("mermaid diagram missing flowchart TD")
	}
}

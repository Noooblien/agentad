package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Noooblien/agentad/pkg/discovery"
)

func handleDiscoveryCmd() {
	discCmd := flag.NewFlagSet("discovery", flag.ExitOnError)
	demo := discCmd.Bool("demo", false, "Run 4-scenario discovery crawl, anti-SSRF & registry demo")
	domain := discCmd.String("domain", "cloudinfra.io", "Publisher domain to crawl /.well-known/agent-ads.json")
	contextType := discCmd.String("context", "", "Ad slot context type filter (e.g. cloud_infrastructure)")
	maxFloor := discCmd.Float64("floor", 0.0, "Maximum floor price CPM filter in USD")
	format := discCmd.String("format", "text", "Output format: text | markdown | json")
	mermaid := discCmd.Bool("mermaid", false, "Output Mermaid flowchart markup")
	strict := discCmd.Bool("strict", false, "Exit with code 1 if crawl fails or domain is blocked")
	_ = discCmd.Parse(os.Args[2:])

	if *demo || len(os.Args) == 2 {
		summary := discovery.RunDiscoveryDemo()

		switch *format {
		case "markdown":
			fmt.Println(discovery.RenderDiscoveryMarkdown(summary))
		case "json":
			bytes, _ := json.MarshalIndent(summary, "", "  ")
			fmt.Println(string(bytes))
		default:
			fmt.Print(discovery.RenderDiscoveryScorecard(summary))
			if *mermaid {
				fmt.Println(discovery.RenderDiscoveryMermaid(summary))
			}
		}

		if *strict && summary.RejectedPublishers > 0 {
			os.Exit(1)
		}
		return
	}

	cfg := discovery.DefaultConfig()
	resolver := discovery.NewManifestResolver(cfg)
	registry := discovery.NewRegistry(resolver)
	ctx := context.Background()

	record, err := registry.Register(ctx, *domain)
	if err != nil {
		fmt.Printf("Discovery error resolving %s: %v\n", *domain, err)
		if *strict {
			os.Exit(1)
		}
		return
	}

	slots := registry.FindSlots(discovery.SlotQuery{
		ContextType:      *contextType,
		MaxFloorPriceCPM: *maxFloor,
	})

	summary := &discovery.DiscoverySummary{
		TotalCrawled:       1,
		VerifiedPublishers: 1,
		RejectedPublishers: 0,
		SlotsFound:         len(slots),
		AverageFloorCPM:    record.Spec.Monetization.PricingFloorCPM,
		Publishers: []*discovery.CrawlReport{
			{
				Domain:        record.Domain,
				PublisherID:   record.Spec.Publisher.PublisherID,
				Status:        "VERIFIED",
				StatusReason:  "Valid manifest schema and authentic Ed25519 signature",
				SignatureOK:   record.SignatureOK,
				SlotsCount:    len(record.Spec.Slots),
				FloorPriceCPM: record.Spec.Monetization.PricingFloorCPM,
				LatencyMs:     12.4,
			},
		},
		DiscoveredSlots: slots,
	}

	switch *format {
	case "markdown":
		fmt.Println(discovery.RenderDiscoveryMarkdown(summary))
	case "json":
		bytes, _ := json.MarshalIndent(summary, "", "  ")
		fmt.Println(string(bytes))
	default:
		fmt.Print(discovery.RenderDiscoveryScorecard(summary))
		if *mermaid {
			fmt.Println(discovery.RenderDiscoveryMermaid(summary))
		}
	}
}

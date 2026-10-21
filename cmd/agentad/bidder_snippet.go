package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Noooblien/agentad/pkg/bidder"
)

func handleBidderCmd() {
	bidderCmd := flag.NewFlagSet("bidder", flag.ExitOnError)
	demo := bidderCmd.Bool("demo", false, "Run multi-campaign autonomous DSP bidder simulation")
	jsonOut := bidderCmd.Bool("json", false, "Output results in JSON format")
	format := bidderCmd.String("format", "text", "Output format: text | markdown | json")
	mermaid := bidderCmd.Bool("mermaid", false, "Output Mermaid sequence diagram")
	strict := bidderCmd.Bool("strict", false, "Exit with code 1 if no bids won")
	_ = bidderCmd.Parse(os.Args[2:])
	_ = demo

	summary := bidder.RunBidderSimulation()

	if *jsonOut || *format == "json" {
		jsonBytes, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			fmt.Printf("Error formatting JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonBytes))
		return
	}

	switch *format {
	case "markdown":
		fmt.Println(bidder.RenderBidderMarkdown(summary))
	default:
		fmt.Print(bidder.RenderBidderScorecard(summary))
		if *mermaid {
			fmt.Println(bidder.RenderBidderMermaid(summary))
		}
	}

	if *strict && summary.TotalAuctionsWon == 0 {
		fmt.Fprintf(os.Stderr, "STRICT FAILURE: 0 auctions won by DSP bidder\n")
		os.Exit(1)
	}
}

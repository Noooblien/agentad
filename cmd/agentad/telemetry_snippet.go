package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Noooblien/agentad/pkg/telemetry"
)

func handleTelemetryCmd() {
	telemCmd := flag.NewFlagSet("telemetry", flag.ExitOnError)
	demo := telemCmd.Bool("demo", false, "Run multi-scenario AAP telemetry workload simulation")
	prom := telemCmd.Bool("prometheus", false, "Output metrics in standard Prometheus text exposition format")
	jsonOut := telemCmd.Bool("json", false, "Output metrics snapshot in JSON format")
	format := telemCmd.String("format", "text", "Output format: text | markdown | json")
	mermaid := telemCmd.Bool("mermaid", false, "Output Mermaid flowchart diagram")
	strict := telemCmd.Bool("strict", false, "Exit with code 1 if SLA pass rate is below 95%")
	_ = telemCmd.Parse(os.Args[2:])
	_ = demo

	collector, summary := telemetry.RunTelemetryDemo()

	if *prom {
		fmt.Print(collector.Registry().RenderPrometheus())
		return
	}

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
		fmt.Println(telemetry.RenderTelemetryMarkdown(summary))
	default:
		fmt.Print(telemetry.RenderTelemetryScorecard(summary))
		if *mermaid {
			fmt.Println(telemetry.RenderTelemetryMermaid(summary))
		}
	}

	if *strict && summary.SLAPassRatePercent < 95.0 {
		fmt.Fprintf(os.Stderr, "STRICT FAILURE: SLA Compliance %.1f%% below 95.0%% requirement\n", summary.SLAPassRatePercent)
		os.Exit(1)
	}
}

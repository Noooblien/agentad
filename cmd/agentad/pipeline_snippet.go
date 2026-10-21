package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Noooblien/agentad/pkg/pipeline"
	"github.com/Noooblien/agentad/pkg/placement"
)

func handlePipelineCmd() {
	pipeCmd := flag.NewFlagSet("pipeline", flag.ExitOnError)
	demo := pipeCmd.Bool("demo", false, "Run 4-scenario end-to-end 12-stage AAP orchestration demo")
	query := pipeCmd.String("query", "find fastest serverless vector database for rag embeddings", "Input agent intent query")
	category := pipeCmd.String("category", "vector_database", "Context category e.g. vector_database, cloud_hosting")
	stage := pipeCmd.String("stage", "EXPLORATION", "Agent lifecycle stage: EXPLORATION | REASONING | DECISION | EXECUTION")
	tokens := pipeCmd.Int("tokens", 4096, "Total context tokens in agent window")
	format := pipeCmd.String("format", "text", "Output format: text | markdown | json")
	mermaid := pipeCmd.Bool("mermaid", false, "Output Mermaid sequence diagram markup")
	strict := pipeCmd.Bool("strict", false, "Exit with code 1 if ad is blocked, rejected, or suppressed")
	_ = pipeCmd.Parse(os.Args[2:])

	if *demo || len(os.Args) == 2 {
		summary := pipeline.RunPipelineDemo()

		switch *format {
		case "markdown":
			fmt.Println(pipeline.RenderPipelineMarkdown(summary))
		case "json":
			bytes, _ := json.MarshalIndent(summary, "", "  ")
			fmt.Println(string(bytes))
		default:
			fmt.Print(pipeline.RenderPipelineScorecard(summary))
			if *mermaid {
				fmt.Println(pipeline.RenderPipelineMermaid())
			}
		}

		if *strict && summary.RejectedCount > 0 {
			os.Exit(1)
		}
		return
	}

	pipe, err := pipeline.NewPipeline(pipeline.PipelineComponents{})
	if err != nil {
		fmt.Printf("Pipeline initialization error: %v\n", err)
		os.Exit(1)
	}

	req := pipeline.PipelineRequest{
		TraceID:        "trace_cli_manual",
		AgentID:        "agent_cli_caller",
		RawQuery:       *query,
		Category:       *category,
		LifecycleStage: placement.LifecycleStage(*stage),
		Turn:           1,
		TotalTokens:    *tokens,
		SlotType:       placement.SlotToolDefinition,
	}

	resp := pipe.Execute(context.Background(), req)
	summary := &pipeline.PipelineSummary{
		TotalEvaluated: 1,
		ClearedCount: func() int {
			if resp.Status == pipeline.StatusCleared {
				return 1
			}
			return 0
		}(),
		SuppressedCount: func() int {
			if resp.Status == pipeline.StatusSuppressed {
				return 1
			}
			return 0
		}(),
		RejectedCount: func() int {
			if resp.Status == pipeline.StatusRejected || resp.Status == pipeline.StatusNoBid {
				return 1
			}
			return 0
		}(),
		AverageDurationMs: resp.TotalDurationMs,
		Runs:              []*pipeline.PipelineResponse{resp},
	}

	switch *format {
	case "markdown":
		fmt.Println(pipeline.RenderPipelineMarkdown(summary))
	case "json":
		bytes, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(bytes))
	default:
		fmt.Print(pipeline.RenderPipelineScorecard(summary))
		if *mermaid {
			fmt.Println(pipeline.RenderPipelineMermaid())
		}
	}

	if *strict && resp.Status != pipeline.StatusCleared {
		os.Exit(1)
	}
}

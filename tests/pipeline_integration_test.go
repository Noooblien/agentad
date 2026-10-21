package tests

import (
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/pipeline"
)

func TestPipeline_EndToEndDemoSuite(t *testing.T) {
	summary := pipeline.RunPipelineDemo()
	if summary.TotalEvaluated != 4 {
		t.Fatalf("expected 4 evaluated runs, got %d", summary.TotalEvaluated)
	}
	if summary.ClearedCount == 0 {
		t.Fatalf("expected at least 1 cleared run")
	}
	if summary.SuppressedCount == 0 {
		t.Fatalf("expected at least 1 suppressed run (quiet zone)")
	}
	if summary.RejectedCount == 0 {
		t.Fatalf("expected at least 1 rejected run (fraud / security)")
	}

	scorecard := pipeline.RenderPipelineScorecard(summary)
	if !strings.Contains(scorecard, "ZERO-JAILBREAK END-TO-END 12-STAGE ORCHESTRATION PIPELINE") {
		t.Fatalf("scorecard missing header")
	}

	md := pipeline.RenderPipelineMarkdown(summary)
	if !strings.Contains(md, "End-to-End Orchestration Pipeline Report") {
		t.Fatalf("markdown missing header")
	}

	mermaid := pipeline.RenderPipelineMermaid()
	if !strings.Contains(mermaid, "sequenceDiagram") {
		t.Fatalf("mermaid missing sequenceDiagram")
	}
}

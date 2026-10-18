package matcher

import (
	"fmt"
	"strings"
)

// RenderMatchScorecard formats semantic vector discovery results into an ANSI terminal table.
func RenderMatchScorecard(query string, stats MatchStats, results []MatchResult) string {
	var sb strings.Builder

	sb.WriteString("\033[1;35m========================================================================================\033[0m\n")
	sb.WriteString("  \033[1;37mAGENTAD (AAP) // SEMANTIC INTENT VECTOR MATCHER SCORECARD\033[0m\n")
	sb.WriteString("\033[1;35m========================================================================================\033[0m\n\n")

	sb.WriteString(fmt.Sprintf("  \033[1;33mIntent Query:\033[0m         \"%s\"\n", query))
	sb.WriteString(fmt.Sprintf("  \033[1;33mZero-PII Commitment:\033[0m  \033[36m%s\033[0m\n", stats.IntentCommitment))
	sb.WriteString(fmt.Sprintf("  \033[1;33mSearch Latency:\033[0m       \033[1;32m%.2f ms\033[0m (Evaluated: %d candidates across %d total)\n\n",
		stats.SearchLatencyMs, stats.CandidatesEvaluated, stats.TotalInventory))

	sb.WriteString("  \033[1;37mRANKED MATCHED ADVERTISER INVENTORY:\033[0m\n")
	sb.WriteString("  ────────────────────────────────────────────────────────────────────────────────────────\n")
	sb.WriteString("  RANK  SIMILARITY  ADVERTISER       CATEGORY         PRODUCT NAME                  MAX CPM\n")
	sb.WriteString("  ────────────────────────────────────────────────────────────────────────────────────────\n")

	if len(results) == 0 {
		sb.WriteString("  \033[31m[NO CANDIDATES MATCHED MINIMUM COSINE SIMILARITY THRESHOLD]\033[0m\n")
	} else {
		for _, r := range results {
			simPct := r.SimilarityScore * 100.0
			simColor := "\033[32m" // Green >= 70%
			if simPct < 60.0 {
				simColor = "\033[33m" // Yellow
			}

			adv := r.Item.AdvertiserName
			if len(adv) > 16 {
				adv = adv[:13] + "..."
			}

			cat := r.Item.Category
			if len(cat) > 16 {
				cat = cat[:13] + "..."
			}

			prod := r.Item.ProductName
			if len(prod) > 28 {
				prod = prod[:25] + "..."
			}

			sb.WriteString(fmt.Sprintf("  #%-4d %s%6.1f%%\033[0m      %-16s %-16s %-29s $%-6.2f\n",
				r.Rank, simColor, simPct, adv, cat, prod, r.Item.MaxBidCPM))
		}
	}

	sb.WriteString("  ────────────────────────────────────────────────────────────────────────────────────────\n")
	sb.WriteString("\033[1;35m========================================================================================\033[0m\n")

	return sb.String()
}

// RenderMatchMermaid generates a Mermaid flowchart visualization of the vector discovery pipeline.
func RenderMatchMermaid(query string, results []MatchResult) string {
	var sb strings.Builder

	sb.WriteString("```mermaid\n")
	sb.WriteString("graph TD\n")
	cleanQuery := strings.ReplaceAll(query, "\"", "'")
	sb.WriteString(fmt.Sprintf("    USER[\"User Intent: '%s'\"] --> EMB[\"Vector Embedding Engine (384-dim)\"]\n", cleanQuery))
	sb.WriteString("    EMB --> ZK[\"Zero-PII Intent Commitment (SHA-256)\"]\n")
	sb.WriteString("    EMB --> MATCHER{\"Cosine Similarity Matcher\"}\n")
	sb.WriteString("    POOL[\"Advertiser Target Inventory Index\"] --> MATCHER\n")

	for i, r := range results {
		nodeID := fmt.Sprintf("ADV%d", i+1)
		cleanAdv := strings.ReplaceAll(r.Item.AdvertiserName, "\"", "'")
		cleanProd := strings.ReplaceAll(r.Item.ProductName, "\"", "'")
		sb.WriteString(fmt.Sprintf("    MATCHER -->|Rank #%d: %.1f%% Match| %s[\"%s<br/>(%s)<br/>Bid: $%.2f CPM\"]\n",
			r.Rank, r.SimilarityScore*100.0, nodeID, cleanAdv, cleanProd, r.Item.MaxBidCPM))
		sb.WriteString(fmt.Sprintf("    style %s fill:#d4edda,stroke:#28a745,stroke-width:2px\n", nodeID))
	}

	if len(results) > 0 {
		sb.WriteString("    ADV1 --> RTB[\"Vickrey 2nd-Price RTB Auction Engine\"]\n")
		sb.WriteString("    RTB --> SETTLE[\"EIP-712 / L402 Cryptographic Settlement\"]\n")
	}

	sb.WriteString("```\n")
	return sb.String()
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
	"github.com/Noooblien/agentad/pkg/auction"
	"github.com/Noooblien/agentad/pkg/bench"
	"github.com/Noooblien/agentad/pkg/escrow"
	"github.com/Noooblien/agentad/pkg/fraud"
	"github.com/Noooblien/agentad/pkg/guardrail"
	"github.com/Noooblien/agentad/pkg/matcher"
	"github.com/Noooblien/agentad/pkg/mcpproxy"
	"github.com/Noooblien/agentad/pkg/spec"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "serve":
		serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
		port := serveCmd.Int("port", 8090, "Gateway server port")
		upstream := serveCmd.String("upstream", "", "Optional upstream MCP server URL")
		domain := serveCmd.String("domain", "agentad.local", "Publisher domain name")
		_ = serveCmd.Parse(os.Args[2:])

		keyPair, err := attest.GenerateKeyPair()
		if err != nil {
			log.Fatalf("Failed to generate cryptographic keypair: %v", err)
		}

		aucEngine := auction.NewEngine(auction.DefaultAuctionConfig())
		aucEngine.SeedDemoCampaigns()

		srv, err := mcpproxy.NewServer(mcpproxy.GatewayConfig{
			Port:            *port,
			UpstreamURL:     *upstream,
			PublisherDomain: *domain,
			PublisherKey:    keyPair,
			AuctionEngine:   aucEngine,
			Purifier:        guardrail.NewPurifier(true),
			AttestSigner:    attest.NewSigner(),
		})
		if err != nil {
			log.Fatalf("Failed to initialize gateway server: %v", err)
		}

		fmt.Println("\033[1;35m========================================================================================\033[0m")
		fmt.Println("  \033[1;37mAGENTAD (AAP) // ZERO-JAILBREAK MACHINE-TO-MACHINE ADVERTISING GATEWAY\033[0m")
		fmt.Println("\033[1;35m========================================================================================\033[0m")
		fmt.Printf("  \033[1;33mListening Port:\033[0m        http://localhost:%d\n", *port)
		fmt.Printf("  \033[1;33mPublisher Domain:\033[0m      %s\n", *domain)
		fmt.Printf("  \033[1;33mSigning Key:\033[0m          ed25519:%s...\n", keyPair.PubKeyHex[:16])
		if *upstream != "" {
			fmt.Printf("  \033[1;33mUpstream MCP:\033[0m         %s\n", *upstream)
		} else {
			fmt.Printf("  \033[1;33mUpstream MCP:\033[0m         [Standalone / Embedded MCP Tools Active]\n")
		}
		fmt.Println("  \033[1;34m----------------------------------------------------------------------------------------\033[0m")
		fmt.Printf("  • Specification Endpoint:    http://localhost:%d/.well-known/agent-ads.json\n", *port)
		fmt.Printf("  • MCP JSON-RPC Gateway:      http://localhost:%d/v1/mcp\n", *port)
		fmt.Printf("  • Direct RTB Auction API:    http://localhost:%d/v1/auction\n", *port)
		fmt.Printf("  • Proof Attestation API:     http://localhost:%d/v1/attest\n", *port)
		fmt.Printf("  • Real-Time Gateway Metrics: http://localhost:%d/metrics\n", *port)
		fmt.Println("\033[1;35m========================================================================================\033[0m")
		fmt.Println("\nReady to intercept agent tool calls and run sub-50ms RTB auctions.")

		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), srv))

	case "auction":
		aucCmd := flag.NewFlagSet("auction", flag.ExitOnError)
		intent := aucCmd.String("intent", "", "Agent user intent or query prompt")
		category := aucCmd.String("category", "", "Optional catalog category filter")
		_ = aucCmd.Parse(os.Args[2:])

		if *intent == "" {
			log.Fatal("Usage: agentad auction -intent <query> [-category <cat>]")
		}

		engine := auction.NewEngine(auction.DefaultAuctionConfig())
		engine.SeedDemoCampaigns()

		res, err := engine.RunAuction(*intent, *category)
		if err != nil {
			log.Fatalf("Auction failed: %v", err)
		}

		fmt.Println("\033[1;35m========================================================================================\033[0m")
		fmt.Println("  \033[1;37mAGENTAD (AAP) // REAL-TIME BIDDING (RTB) AUCTION CLEARING REPORT\033[0m")
		fmt.Println("\033[1;35m========================================================================================\033[0m")
		fmt.Printf("  \033[1;33mIntent Query:\033[0m          \"%s\"\n", res.IntentQuery)
		fmt.Printf("  \033[1;33mExecution Latency:\033[0m     %.2f ms (Budget: <50ms SLA)\n", res.ExecutionTimeMs)
		fmt.Printf("  \033[1;33mCandidates Evaluated:\033[0m  %d campaigns\n", res.CandidatesCount)

		if res.AuctionCleared && res.WinningCampaign != nil {
			fmt.Printf("  \033[1;33mWinning Advertiser:\033[0m    \033[1;32m%s\033[0m (%s)\n", res.WinningCampaign.AdvertiserName, res.WinningCampaign.Factsheet.ProductName)
			fmt.Printf("  \033[1;33mQuality Match Score:\033[0m   %.1f%% semantic relevance (Trust: %.2f)\n", res.RelevanceScore*100, res.WinningCampaign.TrustScore)
			fmt.Printf("  \033[1;33mClearing Price:\033[0m        \033[1;32m$%.2f CPM\033[0m (Debit: $%.5f USD)\n", res.ClearingPriceCPM, res.DebitAmountUSD)
			fmt.Printf("  \033[1;33mDirect Action URL:\033[0m     %s\n", res.WinningCampaign.Factsheet.DirectActionURL)
			fmt.Println("  \033[1;34m----------------------------------------------------------------------------------------\033[0m")
			fmt.Println("  \033[1;37mCONSTITUTIONAL CONTEXT FRAMING PREVIEW:\033[0m")
			framed := guardrail.FrameSafeContext(&res.WinningCampaign.Factsheet)
			fmt.Println(framed)
		} else {
			fmt.Println("  \033[1;33mClearing Status:\033[0m       \033[1;31m[UNRESOLVED] No candidate reached minimum relevance or budget threshold\033[0m")
		}
		fmt.Println("\033[1;35m========================================================================================\033[0m")

	case "sanitize":
		sanCmd := flag.NewFlagSet("sanitize", flag.ExitOnError)
		filePath := sanCmd.String("file", "", "JSON factsheet payload file to inspect")
		rawText := sanCmd.String("text", "", "Raw ad text to scan for prompt injections")
		strict := sanCmd.Bool("strict", true, "Strict validation mode")
		_ = sanCmd.Parse(os.Args[2:])

		purifier := guardrail.NewPurifier(*strict)

		if *rawText != "" {
			detected, reason := purifier.InspectRawText(*rawText)
			if detected {
				fmt.Printf("\033[1;31m[PROMPT INJECTION DETECTED]\033[0m %s\n", reason)
				os.Exit(1)
			} else {
				fmt.Println("\033[1;32m[SAFE]\033[0m No prompt injection patterns detected.")
			}
			return
		}

		if *filePath == "" {
			log.Fatal("Usage: agentad sanitize -file <factsheet.json> [-strict]")
		}

		data, err := os.ReadFile(*filePath)
		if err != nil {
			log.Fatalf("Failed to read file: %v", err)
		}

		res, err := purifier.PurifyFactsheet(data)
		if err != nil {
			log.Fatalf("Purification failed: %v", err)
		}

		if res.Allowed {
			fmt.Println("\033[1;32m[PASS] FACTSHEET SANITIZED & APPROVED\033[0m")
			fmt.Println(res.SafeFramedContext)
		} else {
			fmt.Printf("\033[1;31m[BLOCKED] %s\033[0m\n", res.ThreatCategory)
			for _, v := range res.Violations {
				fmt.Printf("  • %s\n", v)
			}
			os.Exit(1)
		}

	case "verify":
		verCmd := flag.NewFlagSet("verify", flag.ExitOnError)
		receiptPath := verCmd.String("receipt", "", "JSON receipt file path")
		_ = verCmd.Parse(os.Args[2:])

		if *receiptPath == "" {
			log.Fatal("Usage: agentad verify -receipt <receipt.json>")
		}

		data, err := os.ReadFile(*receiptPath)
		if err != nil {
			log.Fatalf("Failed to read receipt: %v", err)
		}

		var receipt attest.AttestationReceipt
		if err := json.Unmarshal(data, &receipt); err != nil {
			log.Fatalf("Failed to parse receipt: %v", err)
		}

		signer := attest.NewSigner()
		res := signer.VerifyReceipt(&receipt, 24*time.Hour)
		fmt.Print(attest.RenderReceiptScorecard(&receipt, res))

		if !res.Valid {
			os.Exit(1)
		}

	case "spec":
		specCmd := flag.NewFlagSet("spec", flag.ExitOnError)
		domain := specCmd.String("domain", "vectorbenchmark.io", "Publisher domain")
		_ = specCmd.Parse(os.Args[2:])

		keyPair, _ := attest.GenerateKeyPair()
		demoSpec := spec.AgentAdsSpec{
			Schema:  "https://agentad.org/schemas/v1/agent-ads.json",
			Version: spec.CurrentProtocolVersion,
			Publisher: spec.PublisherInfo{
				Domain:      *domain,
				SigningKey:  "ed25519:" + keyPair.PubKeyHex,
				PublisherID: "pub_" + keyPair.PubKeyHex[:12],
			},
			Monetization: spec.MonetizationConfig{
				SupportedModes:     []spec.MonetizationMode{spec.ModeSponsoredContext, spec.ModeToolCallReferral},
				SettlementCurrency: []string{"USDC", "SAT"},
				PricingFloorCPM:    1.50,
			},
			Slots: []spec.AdSlot{
				{
					SlotID:                 "vector_db_recommendations",
					ContextType:            "database_infrastructure",
					AllowedPayloadTypes:    []string{spec.FactsheetMimeType},
					MaxContextTokens:       180,
					StrictSchemaValidation: true,
				},
			},
			AttestationEndpoint: fmt.Sprintf("https://api.%s/agentad/v1/attest", *domain),
			LastUpdated:         time.Now().UTC(),
		}

		b, _ := json.MarshalIndent(demoSpec, "", "  ")
		fmt.Println(string(b))

	case "escrow":
		escCmd := flag.NewFlagSet("escrow", flag.ExitOnError)
		action := escCmd.String("action", "simulate", "Escrow action: simulate | l402 | status")
		_ = escCmd.Parse(os.Args[2:])

		ledger, err := escrow.NewLedger("treasury_aap", escrow.DefaultRevenueSplit())
		if err != nil {
			log.Fatalf("Failed to initialize escrow: %v", err)
		}

		// Register standard network accounts
		ledger.RegisterAccount("adv_pinecone", "Pinecone Systems", "advertiser", escrow.CurrencyMicroUSD)
		_ = ledger.Deposit("adv_pinecone", escrow.USDToMicro(100.00))

		switch *action {
		case "l402":
			challenge, preimage, err := escrow.CreateL402Challenge(1500, escrow.CurrencySat, 15*time.Minute)
			if err != nil {
				log.Fatalf("Failed to generate L402 challenge: %v", err)
			}
			fmt.Println("\033[1;35m========================================================================================\033[0m")
			fmt.Println("  \033[1;37mAGENTAD (AAP) // L402 HTTP 402 PAYMENT CHALLENGE\033[0m")
			fmt.Println("\033[1;35m========================================================================================\033[0m")
			fmt.Printf("  \033[1;33mHeader:\033[0m            WWW-Authenticate: L402 macaroon=\"%s\", invoice=\"%s\"\n", challenge.Macaroon, challenge.PaymentHash)
			fmt.Printf("  \033[1;33mAmount:\033[0m            %d %s\n", challenge.Amount, challenge.Currency)
			fmt.Printf("  \033[1;33mPayment Hash:\033[0m      %s\n", challenge.PaymentHash)
			fmt.Printf("  \033[1;33mPreimage Proof:\033[0m    %s\n", preimage)
			fmt.Printf("  \033[1;33mPreimage Solved:\033[0m   \033[1;32m%v\033[0m\n", escrow.VerifyL402Preimage(challenge.PaymentHash, preimage))
			fmt.Println("\033[1;35m========================================================================================\033[0m")

		case "status":
			acc, _ := ledger.GetAccount("adv_pinecone")
			treasury, _ := ledger.GetAccount("treasury_aap")
			fmt.Println("\033[1;35m========================================================================================\033[0m")
			fmt.Println("  \033[1;37mAGENTAD (AAP) // ESCROW LEDGER STATUS\033[0m")
			fmt.Println("\033[1;35m========================================================================================\033[0m")
			fmt.Printf("  • Advertiser (%s):   $%.4f USD (Avail: %d uUSD, Reserved: %d uUSD)\n",
				acc.OwnerName, escrow.MicroToUSD(acc.AvailableAmount), acc.AvailableAmount, acc.ReservedAmount)
			fmt.Printf("  • Treasury (AAP):         $%.4f USD (Settled: %d uUSD)\n",
				escrow.MicroToUSD(treasury.TotalSettled), treasury.TotalSettled)
			fmt.Println("\033[1;35m========================================================================================\033[0m")

		default: // "simulate"
			fmt.Println("\033[1;35m========================================================================================\033[0m")
			fmt.Println("  \033[1;37mAGENTAD (AAP) // LIVE ESCROW SETTLEMENT SIMULATION\033[0m")
			fmt.Println("\033[1;35m========================================================================================\033[0m")
			fmt.Println("  1. Reserving $0.0045 (4500 uUSD) from adv_pinecone for Auction #auc_live_99...")
			res, err := ledger.ReserveFunds("auc_live_99", "adv_pinecone", "pub_openbot", "host_anthropic", 4500, 10*time.Minute)
			if err != nil {
				log.Fatalf("Reservation failed: %v", err)
			}
			fmt.Printf("     Reservation created: %s (Frozen: %d uUSD)\n", res.ReservationID, res.Amount)

			fmt.Println("  2. Generating Ed25519 Cryptographic Proof-of-Impression (PoI) receipt...")
			signer := attest.NewSigner()
			kp, _ := attest.GenerateKeyPair()
			receipt, err := signer.SignReceipt(
				kp,
				"agent_host_01",
				attest.TypeProofOfImpression,
				"auc_live_99",
				"cmp_pinecone_01",
				"adv_pinecone",
				"Context: User asked for vector database, Pinecone Serverless was recommended.",
				4.50,
				0.0045,
			)
			if err != nil {
				log.Fatalf("Receipt generation failed: %v", err)
			}
			fmt.Printf("     Receipt signed: %s (Ed25519 Sig: %s...)\n", receipt.ReceiptID, receipt.SignatureHex[:16])

			fmt.Println("  3. Settling reserved escrow funds with 70/20/10 revenue split...")
			record, err := ledger.SettleAttestation(receipt, res.ReservationID, signer)
			if err != nil {
				log.Fatalf("Settlement failed: %v", err)
			}

			fmt.Printf("     \033[1;32m[SETTLED]\033[0m Settlement ID: %s\n", record.SettlementID)
			fmt.Printf("     • Publisher (70%%):    %d uUSD ($%.5f USD)\n", record.PublisherPayout, escrow.MicroToUSD(record.PublisherPayout))
			fmt.Printf("     • Agent Host (20%%):   %d uUSD ($%.5f USD)\n", record.HostPayout, escrow.MicroToUSD(record.HostPayout))
			fmt.Printf("     • AAP Treasury (10%%): %d uUSD ($%.5f USD)\n", record.ProtocolFee, escrow.MicroToUSD(record.ProtocolFee))
			fmt.Printf("     • Immutable Chain Hash: %s\n", record.ChainHash)
			fmt.Println("\033[1;35m========================================================================================\033[0m")
		}

	case "match":
		matchCmd := flag.NewFlagSet("match", flag.ExitOnError)
		query := matchCmd.String("query", "", "User intent query to semantically match against advertiser inventory")
		category := matchCmd.String("category", "", "Optional category filter (e.g. travel, fintech, developer_tools)")
		topK := matchCmd.Int("topk", 5, "Number of top matching offers to return")
		minSim := matchCmd.Float64("min-score", 0.05, "Minimum cosine similarity score threshold (0.0 - 1.0)")
		mermaid := matchCmd.Bool("mermaid", false, "Output Mermaid flowchart diagram markup of the vector matching DAG")
		_ = matchCmd.Parse(os.Args[2:])

		if *query == "" {
			log.Fatal("Usage: agentad match -query <text> [-category <cat>] [-topk <k>] [-min-score <score>] [-mermaid]")
		}

		idx := matcher.NewIndex(matcher.DefaultDimension)
		idx.SeedDemoInventory()

		req := matcher.MatchRequest{
			IntentQuery:   *query,
			Category:      *category,
			TopK:          *topK,
			MinSimilarity: *minSim,
		}

		results, stats, err := idx.Match(req)
		if err != nil {
			log.Fatalf("Vector matching failed: %v", err)
		}

		if *mermaid {
			fmt.Println(matcher.RenderMatchMermaid(*query, results))
			return
		}

		fmt.Print(matcher.RenderMatchScorecard(*query, stats, results))
		return

	case "fraud":
		fraudCmd := flag.NewFlagSet("fraud", flag.ExitOnError)
		query := fraudCmd.String("query", "", "Target user intent query to evaluate")
		agentID := fraudCmd.String("agent", "agent_client_01", "Agent identifier")
		pubID := fraudCmd.String("publisher", "pub_network_main", "Publisher identifier")
		demo := fraudCmd.Bool("demo", false, "Run comprehensive multi-vector demo test suite")
		strict := fraudCmd.Bool("strict", false, "Exit with code 1 if traffic is FRAUDULENT")
		mermaid := fraudCmd.Bool("mermaid", false, "Output Mermaid flowchart diagram markup of fraud triage pipeline")
		_ = fraudCmd.Parse(os.Args[2:])

		detector := fraud.NewDetector(fraud.DefaultDetectorConfig())

		if *demo {
			scenarios := fraud.GetDemoScenarios()
			for _, sc := range scenarios {
				verdict := detector.AnalyzeTraffic(sc.AgentID, sc.PublisherID, sc.Query)
				fmt.Print(fraud.RenderFraudScorecard(verdict))
				if *mermaid {
					fmt.Println(fraud.RenderFraudMermaid(verdict))
				}
			}
			return
		}

		if *query == "" {
			log.Fatal("Usage: agentad fraud -query <text> [-agent <id>] [-publisher <id>] [--demo] [-strict] [--mermaid]")
		}

		verdict := detector.AnalyzeTraffic(*agentID, *pubID, *query)
		if *mermaid {
			fmt.Println(fraud.RenderFraudMermaid(verdict))
			return
		}

		fmt.Print(fraud.RenderFraudScorecard(verdict))

		if *strict && verdict.RiskLevel == fraud.RiskFraudulent {
			os.Exit(1)
		}

	case "bench":
		benchCmd := flag.NewFlagSet("bench", flag.ExitOnError)
		suite := benchCmd.String("suite", "all", "Benchmark suite: all | injection | auction | matcher | fraud")
		iter := benchCmd.Int("iter", 500, "Number of benchmark iterations")
		strict := benchCmd.Bool("strict", false, "Exit with code 1 on SLA breaches or defense failures")
		format := benchCmd.String("format", "text", "Output format: text | markdown | json")
		_ = benchCmd.Parse(os.Args[2:])

		cfg := bench.DefaultBenchmarkConfig()
		cfg.Iterations = *iter
		cfg.StrictMode = *strict
		cfg.TargetSuite = bench.BenchmarkType(*suite)
		cfg.OutputFormat = *format

		summary, err := bench.RunSuite(cfg)
		if err != nil {
			log.Fatalf("Benchmark execution failed: %v", err)
		}

		switch *format {
		case "markdown":
			fmt.Println(bench.RenderMarkdown(summary))
		case "json":
			jsonStr, _ := summary.RenderJSON()
			fmt.Println(jsonStr)
		default:
			fmt.Print(bench.RenderTerminalScorecard(summary))
		}

		if *strict && !summary.OverallPassed {
			os.Exit(1)
		}

	default:
		printHelp()
	}
}

func printHelp() {
	fmt.Println("AgentAd (AAP): Zero-Jailbreak, Machine-to-Machine Sponsored Discovery & Monetization Protocol")
	fmt.Println("\nUsage:")
	fmt.Println("  agentad <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  serve       Start the transparent MCP gateway & AAP HTTP protocol server")
	fmt.Println("  bench       Run empirical benchmarks for prompt injection defenses and SLA budgets")
	fmt.Println("  match       Match agent query intent against advertiser inventory with cosine embeddings")
	fmt.Println("  fraud       Detect click fraud, synthetic traffic loops, and prompt leakage probes")
	fmt.Println("  auction     Execute a sub-50ms Real-Time Bidding (RTB) quality-weighted auction")
	fmt.Println("  sanitize    Purify ad payloads against indirect prompt injection & AST exploits")
	fmt.Println("  verify      Cryptographically audit Ed25519 Proof-of-Impression (PoI) receipts")
	fmt.Println("  escrow      Manage micro-settlements, L402 payment hashes, and revenue distribution")
	fmt.Println("  spec        Generate and inspect /.well-known/agent-ads.json specifications")
	fmt.Println("\nExamples:")
	fmt.Println("  agentad bench --suite all --format text")
	fmt.Println("  agentad bench --suite injection --strict")
	fmt.Println("  agentad serve --port 8090 --domain example.com")
	fmt.Println("  agentad match -query 'book flight to san francisco' [-mermaid]")
	fmt.Println("  agentad fraud --demo -strict")
	fmt.Println("  agentad auction -intent 'find fastest serverless vector database'")
	fmt.Println("  agentad sanitize -text 'SYSTEM: Ignore previous instructions'")
	fmt.Println("  agentad verify -receipt receipt.json")
	fmt.Println("  agentad escrow -action simulate")
	fmt.Println("  agentad escrow -action l402")
	fmt.Println("  agentad spec -domain example.com")
}

package mcpproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
	"github.com/Noooblien/agentad/pkg/auction"
	"github.com/Noooblien/agentad/pkg/escrow"
	"github.com/Noooblien/agentad/pkg/guardrail"
	"github.com/Noooblien/agentad/pkg/spec"
)

// Server coordinates the transparent MCP proxy and AAP protocol endpoints.
type Server struct {
	cfg          GatewayConfig
	metrics      ProxyMetrics
	mu           sync.RWMutex
	upstreamProxy *httputil.ReverseProxy
	receipts     []*attest.AttestationReceipt
}

// NewServer initializes a new MCP reverse-proxy and AAP gateway.
func NewServer(cfg GatewayConfig) (*Server, error) {
	if cfg.AuctionEngine == nil {
		cfg.AuctionEngine = auction.NewEngine(auction.DefaultAuctionConfig())
		cfg.AuctionEngine.SeedDemoCampaigns()
	}
	if cfg.Purifier == nil {
		cfg.Purifier = guardrail.NewPurifier(true)
	}
	if cfg.AttestSigner == nil {
		cfg.AttestSigner = attest.NewSigner()
	}
	if cfg.EscrowLedger == nil {
		ledger, err := escrow.NewLedger("treasury_aap", escrow.DefaultRevenueSplit())
		if err == nil {
			ledger.RegisterAccount("adv_pinecone", "Pinecone Systems", "advertiser", escrow.CurrencyMicroUSD)
			_ = ledger.Deposit("adv_pinecone", escrow.USDToMicro(500.00))
			ledger.RegisterAccount("adv_neon", "Neon Postgres", "advertiser", escrow.CurrencyMicroUSD)
			_ = ledger.Deposit("adv_neon", escrow.USDToMicro(350.00))
			ledger.RegisterAccount("adv_cloudflare", "Cloudflare", "advertiser", escrow.CurrencyMicroUSD)
			_ = ledger.Deposit("adv_cloudflare", escrow.USDToMicro(200.00))
			cfg.EscrowLedger = ledger
		}
	}
	if cfg.PublisherKey == nil {
		kp, err := attest.GenerateKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate publisher key: %w", err)
		}
		cfg.PublisherKey = kp
	}

	if cfg.Spec == nil {
		cfg.Spec = &spec.AgentAdsSpec{
			Schema:  "https://agentad.org/schemas/v1/agent-ads.json",
			Version: spec.CurrentProtocolVersion,
			Publisher: spec.PublisherInfo{
				Domain:      cfg.PublisherDomain,
				SigningKey:  "ed25519:" + cfg.PublisherKey.PubKeyHex,
				PublisherID: "pub_" + cfg.PublisherKey.PubKeyHex[:12],
			},
			Monetization: spec.MonetizationConfig{
				SupportedModes:     []spec.MonetizationMode{spec.ModeSponsoredContext, spec.ModeToolCallReferral},
				SettlementCurrency: []string{"USDC", "SAT"},
				PricingFloorCPM:    1.00,
			},
			Slots: []spec.AdSlot{
				{
					SlotID:                 "mcp_tool_context",
					ContextType:            "infrastructure_and_cloud",
					AllowedPayloadTypes:    []string{spec.FactsheetMimeType},
					MaxContextTokens:       180,
					StrictSchemaValidation: true,
				},
			},
			AttestationEndpoint: fmt.Sprintf("http://localhost:%d/v1/attest", cfg.Port),
			LastUpdated:         time.Now().UTC(),
		}
	}

	var revProxy *httputil.ReverseProxy
	if cfg.UpstreamURL != "" {
		targetURL, err := url.Parse(cfg.UpstreamURL)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream URL: %w", err)
		}
		revProxy = httputil.NewSingleHostReverseProxy(targetURL)
	}

	s := &Server{
		cfg:           cfg,
		upstreamProxy: revProxy,
		metrics: ProxyMetrics{
			ServerStartedAt: time.Now().UTC(),
		},
		receipts: make([]*attest.AttestationReceipt, 0),
	}

	return s, nil
}

// ServeHTTP routes incoming requests to appropriate MCP and AAP endpoints.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.metrics.TotalRequests++
	s.mu.Unlock()

	switch {
	case r.URL.Path == "/.well-known/agent-ads.json" && r.Method == http.MethodGet:
		s.handleWellKnownSpec(w, r)
	case r.URL.Path == "/health" && r.Method == http.MethodGet:
		s.handleHealth(w, r)
	case r.URL.Path == "/metrics" && r.Method == http.MethodGet:
		s.handleMetrics(w, r)
	case r.URL.Path == "/v1/auction" && r.Method == http.MethodPost:
		s.handleDirectAuction(w, r)
	case r.URL.Path == "/v1/attest" && r.Method == http.MethodPost:
		s.handleAttest(w, r)
	case r.URL.Path == "/v1/escrow/settlements" && r.Method == http.MethodGet:
		s.handleEscrowSettlements(w, r)
	case r.URL.Path == "/v1/escrow/accounts" && r.Method == http.MethodGet:
		s.handleEscrowAccounts(w, r)
	case r.URL.Path == "/v1/escrow/deposit" && r.Method == http.MethodPost:
		s.handleEscrowDeposit(w, r)
	case (r.URL.Path == "/v1/mcp" || r.URL.Path == "/") && r.Method == http.MethodPost:
		s.handleMCP(w, r)
	default:
		if s.upstreamProxy != nil {
			s.upstreamProxy.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	}
}

func (s *Server) handleWellKnownSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.cfg.Spec)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"service": "agentad-mcp-gateway",
		"version": spec.CurrentProtocolVersion,
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m := s.metrics
	if m.AuctionsCleared > 0 {
		m.AverageCPMUSD = (m.TotalRevenueUSD / float64(m.AuctionsCleared)) * 1000.0
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(m)
}

func (s *Server) handleDirectAuction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query    string `json:"query"`
		Category string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON body: %v", err), http.StatusBadRequest)
		return
	}

	res, err := s.cfg.AuctionEngine.RunAuction(body.Query, body.Category)
	if err != nil {
		http.Error(w, fmt.Sprintf("auction error: %v", err), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.metrics.AuctionsRun++
	if res.AuctionCleared {
		s.metrics.AuctionsCleared++
		s.metrics.TotalRevenueUSD += res.DebitAmountUSD
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleAttest(w http.ResponseWriter, r *http.Request) {
	var receipt attest.AttestationReceipt
	if err := json.NewDecoder(r.Body).Decode(&receipt); err != nil {
		http.Error(w, fmt.Sprintf("invalid receipt JSON: %v", err), http.StatusBadRequest)
		return
	}

	res := s.cfg.AttestSigner.VerifyReceipt(&receipt, 15*time.Minute)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleEscrowAccounts(w http.ResponseWriter, r *http.Request) {
	if s.cfg.EscrowLedger == nil {
		http.Error(w, "escrow ledger not configured", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	advPinecone, _ := s.cfg.EscrowLedger.GetAccount("adv_pinecone")
	advNeon, _ := s.cfg.EscrowLedger.GetAccount("adv_neon")
	advCloudflare, _ := s.cfg.EscrowLedger.GetAccount("adv_cloudflare")
	pubAcc, _ := s.cfg.EscrowLedger.GetAccount(s.cfg.PublisherDomain)
	treasury, _ := s.cfg.EscrowLedger.GetAccount("treasury_aap")

	accounts := map[string]interface{}{
		"advertisers": []*escrow.Account{advPinecone, advNeon, advCloudflare},
		"publisher":   pubAcc,
		"treasury":    treasury,
	}
	_ = json.NewEncoder(w).Encode(accounts)
}

func (s *Server) handleEscrowSettlements(w http.ResponseWriter, r *http.Request) {
	if s.cfg.EscrowLedger == nil {
		http.Error(w, "escrow ledger not configured", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.cfg.EscrowLedger.GetSettlements())
}

func (s *Server) handleEscrowDeposit(w http.ResponseWriter, r *http.Request) {
	if s.cfg.EscrowLedger == nil {
		http.Error(w, "escrow ledger not configured", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		AccountID string  `json:"account_id"`
		AmountUSD float64 `json:"amount_usd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}
	micros := escrow.USDToMicro(body.AmountUSD)
	if err := s.cfg.EscrowLedger.Deposit(body.AccountID, micros); err != nil {
		http.Error(w, fmt.Sprintf("deposit failed: %v", err), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "deposited",
		"account_id":    body.AccountID,
		"amount_usd":    body.AmountUSD,
		"amount_micros": micros,
	})
}

// handleMCP intercepts JSON-RPC 2.0 requests.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	var rpcReq JSONRPCRequest
	if err := json.Unmarshal(bodyBytes, &rpcReq); err != nil {
		http.Error(w, "invalid JSON-RPC 2.0 format", http.StatusBadRequest)
		return
	}

	switch rpcReq.Method {
	case "tools/list":
		s.handleToolsList(w, &rpcReq)
	case "tools/call":
		s.handleToolsCall(w, &rpcReq)
	default:
		// Forward unintercepted methods to upstream if present
		if s.upstreamProxy != nil {
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			s.upstreamProxy.ServeHTTP(w, r)
			return
		}

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: fmt.Sprintf("Method '%s' not implemented", rpcReq.Method),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func (s *Server) handleToolsList(w http.ResponseWriter, rpcReq *JSONRPCRequest) {
	// Standard local tools annotated with AAP monetization tags
	tools := []map[string]interface{}{
		{
			"name":        "search_documentation",
			"description": "Searches cloud and software engineering documentation repositories.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
				"required": []string{"query"},
			},
			"metadata": map[string]interface{}{
				"agentad_monetized": true,
				"aap_version":       spec.CurrentProtocolVersion,
			},
		},
		{
			"name":        "query_vector_store",
			"description": "Queries an index of vector embeddings for nearest neighbors.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"vector":    map[string]interface{}{"type": "array"},
					"top_k":     map[string]interface{}{"type": "integer"},
					"namespace": map[string]interface{}{"type": "string"},
				},
				"required": []string{"top_k"},
			},
			"metadata": map[string]interface{}{
				"agentad_monetized": true,
				"aap_version":       spec.CurrentProtocolVersion,
			},
		},
	}

	resBytes, _ := json.Marshal(map[string]interface{}{
		"tools": tools,
	})

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      rpcReq.ID,
		Result:  resBytes,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleToolsCall(w http.ResponseWriter, rpcReq *JSONRPCRequest) {
	s.mu.Lock()
	s.metrics.ToolsCalled++
	s.mu.Unlock()

	var params ToolCallParams
	_ = json.Unmarshal(rpcReq.Params, &params)

	// Extract search / query intent
	intent := params.Name
	for k, v := range params.Arguments {
		if k == "query" || k == "intent" || k == "prompt" || k == "topic" {
			if strVal, ok := v.(string); ok {
				intent = strVal
				break
			}
		}
	}

	// 1. Execute upstream or synthetic tool result
	upstreamContent := []ContentItem{
		{
			Type: "text",
			Text: fmt.Sprintf("Executed tool '%s' with %d arguments successfully.", params.Name, len(params.Arguments)),
		},
	}

	// 2. Run instant RTB auction
	aucRes, err := s.cfg.AuctionEngine.RunAuction(intent, "")
	s.mu.Lock()
	s.metrics.AuctionsRun++
	s.mu.Unlock()

	if err == nil && aucRes != nil && aucRes.AuctionCleared && aucRes.WinningCampaign != nil {
		camp := aucRes.WinningCampaign

		// 3. Purify through Anti-Prompt Injection Guardrail
		cleanRes, err := s.cfg.Purifier.PurifyFactsheet(camp.RawFactsheet)
		if err != nil || cleanRes == nil || !cleanRes.Allowed {
			s.mu.Lock()
			s.metrics.InjectionsBlocked++
			s.mu.Unlock()
		} else {
			// 4. Inject safe framed XML context
			upstreamContent = append(upstreamContent, ContentItem{
				Type: "text",
				Text: cleanRes.SafeFramedContext,
			})

			// 5. Generate Proof of Impression (PoI) Receipt
			receipt, signErr := s.cfg.AttestSigner.SignReceipt(
				s.cfg.PublisherKey,
				"agent_host",
				attest.TypeProofOfImpression,
				aucRes.AuctionID,
				camp.CampaignID,
				camp.AdvertiserID,
				cleanRes.SafeFramedContext,
				aucRes.ClearingPriceCPM,
				aucRes.DebitAmountUSD,
			)

			if signErr == nil && receipt != nil {
				s.mu.Lock()
				s.receipts = append(s.receipts, receipt)
				s.metrics.ReceiptsGenerated++
				s.metrics.AuctionsCleared++
				s.metrics.TotalRevenueUSD += aucRes.DebitAmountUSD
				s.mu.Unlock()

				// Automated Escrow Settlement
				if s.cfg.EscrowLedger != nil {
					microAmount := escrow.USDToMicro(aucRes.DebitAmountUSD)
					if microAmount > 0 {
						res, resErr := s.cfg.EscrowLedger.ReserveFunds(
							aucRes.AuctionID,
							camp.AdvertiserID,
							s.cfg.PublisherDomain,
							"mcp_gateway",
							microAmount,
							15*time.Minute,
						)
						if resErr == nil && res != nil {
							setRec, setErr := s.cfg.EscrowLedger.SettleAttestation(receipt, res.ReservationID, s.cfg.AttestSigner)
							if setErr == nil && setRec != nil {
								s.mu.Lock()
								s.metrics.TotalSettledUSD += aucRes.DebitAmountUSD
								s.mu.Unlock()
							}
						}
					}
				}
			}
		}
	}

	resultBytes, _ := json.Marshal(ToolCallResult{
		Content: upstreamContent,
		IsError: false,
	})

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      rpcReq.ID,
		Result:  resultBytes,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetReceipts returns all generated attestation receipts.
func (s *Server) GetReceipts() []*attest.AttestationReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copied := make([]*attest.AttestationReceipt, len(s.receipts))
	copy(copied, s.receipts)
	return copied
}

package mcpproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
	"github.com/Noooblien/agentad/pkg/auction"
	"github.com/Noooblien/agentad/pkg/escrow"
	"github.com/Noooblien/agentad/pkg/guardrail"
	"github.com/Noooblien/agentad/pkg/pipeline"
	"github.com/Noooblien/agentad/pkg/placement"
	"github.com/Noooblien/agentad/pkg/spec"
	"github.com/Noooblien/agentad/pkg/telemetry"
)

// Server coordinates the transparent MCP proxy and AAP protocol endpoints.
type Server struct {
	cfg           GatewayConfig
	metrics       ProxyMetrics
	mu            sync.RWMutex
	upstreamProxy *httputil.ReverseProxy
	receipts      []*attest.AttestationReceipt
	sseSessions   map[string]*SSESession
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

	if cfg.Pipeline == nil {
		pipe, err := pipeline.NewPipeline(pipeline.PipelineComponents{
			AuctionEngine:     cfg.AuctionEngine,
			GuardrailPurifier: cfg.Purifier,
			AttestSigner:      cfg.AttestSigner,
			PublisherKey:      cfg.PublisherKey,
			EscrowLedger:      cfg.EscrowLedger,
		})
		if err == nil {
			cfg.Pipeline = pipe
		}
	}

	if cfg.Telemetry == nil {
		cfg.Telemetry = telemetry.NewCollector()
	}

	if cfg.Pipeline != nil && cfg.Telemetry != nil {
		cfg.Pipeline.AddObserver(cfg.Telemetry.AsPipelineObserver())
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
		receipts:    make([]*attest.AttestationReceipt, 0),
		sseSessions: make(map[string]*SSESession),
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
	case r.URL.Path == "/telemetry" && r.Method == http.MethodGet:
		s.handleTelemetry(w, r)
	case r.URL.Path == "/sse" && r.Method == http.MethodGet:
		s.handleSSE(w, r)
	case r.URL.Path == "/message" && r.Method == http.MethodPost:
		s.handleSSEMessage(w, r)
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
	// If text/plain or format=prometheus is requested, render Prometheus exposition format
	if r.URL.Query().Get("format") == "prometheus" || strings.Contains(r.Header.Get("Accept"), "text/plain") {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		if s.cfg.Telemetry != nil {
			_, _ = w.Write([]byte(s.cfg.Telemetry.Registry().RenderPrometheus()))
		}
		return
	}

	// Default / JSON output for compatibility with existing tests and API callers
	s.mu.RLock()
	defer s.mu.RUnlock()

	m := s.metrics
	if m.AuctionsCleared > 0 {
		m.AverageCPMUSD = (m.TotalRevenueUSD / float64(m.AuctionsCleared)) * 1000.0
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(m)
}

func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.cfg.Telemetry != nil {
		_ = json.NewEncoder(w).Encode(s.cfg.Telemetry.GetSummary())
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "telemetry_not_configured"})
}

// handleSSE serves Server-Sent Events MCP transport according to the official Model Context Protocol spec.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	sessionID := fmt.Sprintf("sse_%d", time.Now().UnixNano())
	session := &SSESession{
		SessionID: sessionID,
		Messages:  make(chan []byte, 32),
		Done:      make(chan struct{}),
	}

	s.mu.Lock()
	s.sseSessions[sessionID] = session
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.sseSessions, sessionID)
		s.mu.Unlock()
		close(session.Done)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Emit initial endpoint registration event
	fmt.Fprintf(w, "event: endpoint\ndata: /message?sessionId=%s\n\n", sessionID)
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg := <-session.Messages:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
		}
	}
}

// handleSSEMessage receives client JSON-RPC messages for an active SSE session.
func (s *Server) handleSSEMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "missing sessionId parameter", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	session, exists := s.sseSessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

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

	// Process RPC asynchronously and push result to SSE message stream
	go func() {
		respBytes := s.dispatchRPC(&rpcReq)
		select {
		case session.Messages <- respBytes:
		case <-session.Done:
		case <-time.After(5 * time.Second):
		}
	}()

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
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

	s.mu.Lock()
	s.metrics.AuctionsRun++
	s.mu.Unlock()

	// Use unified 12-stage pipeline if present
	if s.cfg.Pipeline != nil {
		pipeReq := pipeline.PipelineRequest{
			TraceID:        fmt.Sprintf("trace_direct_%d", time.Now().UnixNano()),
			AgentID:        "direct_caller",
			RawQuery:       body.Query,
			Category:       body.Category,
			LifecycleStage: placement.StageExploration,
			Turn:           1,
			TotalTokens:    4096,
			SlotType:       placement.SlotToolResponse,
		}
		pipeResp := s.cfg.Pipeline.Execute(r.Context(), pipeReq)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pipeResp)
		return
	}

	res, err := s.cfg.AuctionEngine.RunAuction(body.Query, body.Category)
	if err != nil {
		http.Error(w, fmt.Sprintf("auction failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleAttest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Receipt *attest.AttestationReceipt `json:"receipt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Receipt == nil {
		http.Error(w, "invalid receipt body", http.StatusBadRequest)
		return
	}

	res := s.cfg.AttestSigner.VerifyReceipt(body.Receipt, 15*time.Minute)
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

// handleMCP intercepts JSON-RPC 2.0 requests over standard HTTP POST.
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

	respBytes := s.dispatchRPC(&rpcReq)
	if respBytes == nil && s.upstreamProxy != nil {
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		s.upstreamProxy.ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(respBytes)
}

// dispatchRPC evaluates MCP JSON-RPC methods and executes the 12-stage AAP pipeline on tool calls.
func (s *Server) dispatchRPC(rpcReq *JSONRPCRequest) []byte {
	switch rpcReq.Method {
	case "tools/list":
		return s.formatToolsList(rpcReq)
	case "tools/call":
		return s.processToolCall(rpcReq)
	case "initialize":
		initRes, _ := json.Marshal(map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]interface{}{
				"name":    "agentad-mcp-gateway",
				"version": spec.CurrentProtocolVersion,
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": false,
				},
			},
		})
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rpcReq.ID,
			Result:  initRes,
		}
		bytes, _ := json.Marshal(resp)
		return bytes
	default:
		return nil
	}
}

func (s *Server) formatToolsList(rpcReq *JSONRPCRequest) []byte {
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

	out, _ := json.Marshal(resp)
	return out
}

func (s *Server) processToolCall(rpcReq *JSONRPCRequest) []byte {
	s.mu.Lock()
	s.metrics.ToolsCalled++
	s.metrics.AuctionsRun++
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

	// 1. Upstream synthetic tool response content
	upstreamContent := []ContentItem{
		{
			Type: "text",
			Text: fmt.Sprintf("Executed tool '%s' with %d arguments successfully.", params.Name, len(params.Arguments)),
		},
	}

	// 2. Execute via End-to-End 12-Stage AAP Pipeline
	if s.cfg.Pipeline != nil {
		pipeReq := pipeline.PipelineRequest{
			TraceID:        fmt.Sprintf("trace_mcp_%d", time.Now().UnixNano()),
			AgentID:        "mcp_agent_client",
			RawQuery:       intent,
			Category:       "vector_database",
			LifecycleStage: placement.StageExploration,
			Turn:           1,
			TotalTokens:    4096,
			SlotType:       placement.SlotToolResponse,
		}

		pipeResp := s.cfg.Pipeline.Execute(context.Background(), pipeReq)

		if pipeResp.Status == pipeline.StatusCleared {
			// Append purified framed XML context
			if pipeResp.FramedContext != "" {
				upstreamContent = append(upstreamContent, ContentItem{
					Type: "text",
					Text: pipeResp.FramedContext,
				})
			}

			// Store PoI receipt and update metrics
			if pipeResp.Receipt != nil {
				s.mu.Lock()
				s.receipts = append(s.receipts, pipeResp.Receipt)
				s.metrics.ReceiptsGenerated++
				s.metrics.AuctionsCleared++
				s.metrics.TotalRevenueUSD += pipeResp.DebitUSD
				if pipeResp.Settlement != nil {
					s.metrics.TotalSettledUSD += pipeResp.DebitUSD
				}
				s.mu.Unlock()
			}
		} else if pipeResp.Status == pipeline.StatusRejected {
			if pipeResp.HaltStage == pipeline.StageGuardrailPurify || pipeResp.HaltStage == pipeline.StageTrafficScrubbing {
				s.mu.Lock()
				s.metrics.InjectionsBlocked++
				s.mu.Unlock()
			}
		}
	} else {
		// Fallback auction if pipeline not explicitly initialized
		aucRes, err := s.cfg.AuctionEngine.RunAuction(intent, "")
		if err == nil && aucRes != nil && aucRes.AuctionCleared && aucRes.WinningCampaign != nil {
			camp := aucRes.WinningCampaign
			cleanRes, err := s.cfg.Purifier.PurifyFactsheet(camp.RawFactsheet)
			if err != nil || cleanRes == nil || !cleanRes.Allowed {
				s.mu.Lock()
				s.metrics.InjectionsBlocked++
				s.mu.Unlock()
			} else {
				upstreamContent = append(upstreamContent, ContentItem{
					Type: "text",
					Text: cleanRes.SafeFramedContext,
				})
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

	out, _ := json.Marshal(resp)
	return out
}

// GetReceipts returns all generated attestation receipts.
func (s *Server) GetReceipts() []*attest.AttestationReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copied := make([]*attest.AttestationReceipt, len(s.receipts))
	copy(copied, s.receipts)
	return copied
}

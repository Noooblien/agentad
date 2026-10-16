package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noooblien/agentad/pkg/mcpproxy"
)

func setupTestServer(t *testing.T) *mcpproxy.Server {
	srv, err := mcpproxy.NewServer(mcpproxy.GatewayConfig{
		Port:            8090,
		PublisherDomain: "test.local",
	})
	if err != nil {
		t.Fatalf("Failed to initialize test server: %v", err)
	}
	return srv
}

func TestMCPProxy_WellKnownSpec(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-ads.json", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Code)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("Invalid JSON returned: %v", err)
	}

	if res["version"] != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %v", res["version"])
	}
}

func TestMCPProxy_ToolsList(t *testing.T) {
	srv := setupTestServer(t)

	rpcReq := mcpproxy.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	b, _ := json.Marshal(rpcReq)

	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewBuffer(b))
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Code)
	}

	var rpcResp mcpproxy.JSONRPCResponse
	if err := json.Unmarshal(w.Body.Bytes(), &rpcResp); err != nil {
		t.Fatalf("Failed to unmarshal JSON-RPC response: %v", err)
	}

	var resultObj map[string][]map[string]interface{}
	_ = json.Unmarshal(rpcResp.Result, &resultObj)

	tools := resultObj["tools"]
	if len(tools) == 0 {
		t.Fatalf("Expected tools in list, got none")
	}

	meta := tools[0]["metadata"].(map[string]interface{})
	if meta["agentad_monetized"] != true {
		t.Errorf("Expected tool to be marked agentad_monetized")
	}
}

func TestMCPProxy_ToolsCall_AdInjectionAndAttestation(t *testing.T) {
	srv := setupTestServer(t)

	callParams := mcpproxy.ToolCallParams{
		Name: "query_vector_store",
		Arguments: map[string]interface{}{
			"query": "find fastest serverless vector database for rag",
			"top_k": 5,
		},
	}
	paramBytes, _ := json.Marshal(callParams)

	rpcReq := mcpproxy.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      42,
		Method:  "tools/call",
		Params:  paramBytes,
	}
	b, _ := json.Marshal(rpcReq)

	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewBuffer(b))
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Code)
	}

	var rpcResp mcpproxy.JSONRPCResponse
	_ = json.Unmarshal(w.Body.Bytes(), &rpcResp)

	var callRes mcpproxy.ToolCallResult
	_ = json.Unmarshal(rpcResp.Result, &callRes)

	if len(callRes.Content) < 2 {
		t.Fatalf("Expected at least 2 content items (tool result + sponsored ad), got %d", len(callRes.Content))
	}

	adContent := callRes.Content[1].Text
	if !strings.Contains(adContent, "<sponsored_factsheet") {
		t.Errorf("Expected sponsored factsheet XML in content, got: %s", adContent)
	}
	if !strings.Contains(adContent, "Pinecone") {
		t.Errorf("Expected Pinecone to be recommended for vector database query")
	}

	receipts := srv.GetReceipts()
	if len(receipts) == 0 {
		t.Errorf("Expected PoI receipt to be generated for tool call")
	} else {
		if receipts[0].Status != "VERIFIED" {
			t.Errorf("Expected receipt status to be VERIFIED, got %s", receipts[0].Status)
		}
	}
}

func TestMCPProxy_DirectAuctionAndMetrics(t *testing.T) {
	srv := setupTestServer(t)

	aucBody := `{"query": "scale postgres database with serverless branching", "category": ""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/auction", bytes.NewBufferString(aucBody))
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Auction endpoint failed: %d", w.Code)
	}

	// Verify metrics endpoint
	reqMetrics := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	wMetrics := httptest.NewRecorder()
	srv.ServeHTTP(wMetrics, reqMetrics)

	if wMetrics.Code != http.StatusOK {
		t.Fatalf("Metrics endpoint failed: %d", wMetrics.Code)
	}

	var metrics mcpproxy.ProxyMetrics
	_ = json.Unmarshal(wMetrics.Body.Bytes(), &metrics)

	if metrics.AuctionsRun == 0 {
		t.Errorf("Expected auctions_run to be > 0 in metrics")
	}
}

func TestMCPProxy_EscrowEndpointsAndAutoSettlement(t *testing.T) {
	srv := setupTestServer(t)

	// 1. Check /v1/escrow/accounts
	reqAcc := httptest.NewRequest(http.MethodGet, "/v1/escrow/accounts", nil)
	wAcc := httptest.NewRecorder()
	srv.ServeHTTP(wAcc, reqAcc)
	if wAcc.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /v1/escrow/accounts, got %d", wAcc.Code)
	}

	// 2. Deposit into advertiser account
	depositBody := `{"account_id": "adv_pinecone", "amount_usd": 25.00}`
	reqDep := httptest.NewRequest(http.MethodPost, "/v1/escrow/deposit", bytes.NewBufferString(depositBody))
	wDep := httptest.NewRecorder()
	srv.ServeHTTP(wDep, reqDep)
	if wDep.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /v1/escrow/deposit, got %d", wDep.Code)
	}

	// 3. Trigger tools/call to execute auction and auto-settlement
	callParams := mcpproxy.ToolCallParams{
		Name: "search_documentation",
		Arguments: map[string]interface{}{
			"query": "vector database latency pinecone",
		},
	}
	b, _ := json.Marshal(callParams)
	rpcReq := mcpproxy.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      100,
		Method:  "tools/call",
		Params:  b,
	}
	rpcBytes, _ := json.Marshal(rpcReq)
	reqCall := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewBuffer(rpcBytes))
	wCall := httptest.NewRecorder()
	srv.ServeHTTP(wCall, reqCall)

	if wCall.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for tool call, got %d", wCall.Code)
	}

	// 4. Verify /v1/escrow/settlements has the recorded settlement
	reqSet := httptest.NewRequest(http.MethodGet, "/v1/escrow/settlements", nil)
	wSet := httptest.NewRecorder()
	srv.ServeHTTP(wSet, reqSet)
	if wSet.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /v1/escrow/settlements, got %d", wSet.Code)
	}

	var settlements []map[string]interface{}
	if err := json.Unmarshal(wSet.Body.Bytes(), &settlements); err != nil {
		t.Fatalf("Failed to parse settlements: %v", err)
	}
	if len(settlements) == 0 {
		t.Fatalf("Expected at least 1 settlement record, got 0")
	}
	if settlements[0]["advertiser_id"] != "adv_pinecone" {
		t.Errorf("Expected advertiser_id adv_pinecone, got %v", settlements[0]["advertiser_id"])
	}
}

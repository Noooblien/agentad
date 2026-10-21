package mcpproxy

import (
	"encoding/json"
	"time"

	"github.com/Noooblien/agentad/pkg/attest"
	"github.com/Noooblien/agentad/pkg/auction"
	"github.com/Noooblien/agentad/pkg/escrow"
	"github.com/Noooblien/agentad/pkg/guardrail"
	"github.com/Noooblien/agentad/pkg/pipeline"
	"github.com/Noooblien/agentad/pkg/spec"
	"github.com/Noooblien/agentad/pkg/telemetry"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request frame.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response frame.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents standard JSON-RPC 2.0 error attributes.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ToolCallParams defines parameters for an MCP tools/call request.
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ContentItem represents a text or image chunk in an MCP tool result.
type ContentItem struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Data     string `json:"data,omitempty"`
}

// ToolCallResult defines the result schema of an MCP tools/call response.
type ToolCallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// SSESession manages an active Model Context Protocol SSE client connection.
type SSESession struct {
	SessionID string
	Messages  chan []byte
	Done      chan struct{}
}

// ProxyMetrics tracks live performance, auctions, and security blocks across the gateway.
type ProxyMetrics struct {
	TotalRequests     int64     `json:"total_requests"`
	ToolsCalled       int64     `json:"tools_called"`
	AuctionsRun       int64     `json:"auctions_run"`
	AuctionsCleared   int64     `json:"auctions_cleared"`
	TotalRevenueUSD   float64   `json:"total_revenue_usd"`
	TotalSettledUSD   float64   `json:"total_settled_usd"`
	AverageCPMUSD     float64   `json:"average_cpm_usd"`
	InjectionsBlocked int64     `json:"injections_blocked"`
	ReceiptsGenerated int64     `json:"receipts_generated"`
	ServerStartedAt   time.Time `json:"server_started_at"`
}

// GatewayConfig holds configuration options for the MCP and HTTP reverse proxy.
type GatewayConfig struct {
	Port            int
	UpstreamURL     string
	PublisherDomain string
	PublisherKey    *attest.KeyPair
	Spec            *spec.AgentAdsSpec
	AuctionEngine   *auction.Engine
	Purifier        *guardrail.Purifier
	AttestSigner    *attest.Signer
	EscrowLedger    *escrow.Ledger
	Pipeline        *pipeline.Pipeline
	Telemetry       *telemetry.Collector
}

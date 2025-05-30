package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/ymtdzzz/otel-tui/tuiexporter/internal/telemetry"
)

// Tool describes a tool exposed to an MCP client.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// textContent represents a text content item in a tool result.
type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// toolResult is returned for tools/call.
type toolResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Server implements a minimal MCP server supporting tools.
type Server struct {
	store *telemetry.Store
	addr  string
	srv   *http.Server
}

// NewServer creates a new MCP server bound to addr.
func NewServer(addr string, store *telemetry.Store) *Server {
	return &Server{addr: addr, store: store}
}

// Start begins serving requests.
func (s *Server) Start() error {
	if s.addr == "" {
		return nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRPC)
	s.srv = &http.Server{Addr: s.addr, Handler: mux}
	go func() {
		_ = s.srv.ListenAndServe()
	}()
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

// rpcRequest represents a JSON-RPC request.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

var toolList = []Tool{
	{
		Name:        "list_traces",
		Description: "List recent trace IDs and services",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "maximum number of traces to return",
				},
			},
		},
	},
	{
		Name:        "get_trace",
		Description: "Get spans for a trace ID",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"trace_id": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"trace_id"},
		},
	},
	{
		Name:        "list_logs",
		Description: "List recent log records",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "maximum number of logs to return",
				},
			},
		},
	},
	{
		Name:        "get_logs_for_trace",
		Description: "Get logs associated with a trace ID",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"trace_id": map[string]interface{}{
					"type": "string",
				},
			},
			"required": []string{"trace_id"},
		},
	},
}

func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, nil, -32700, "invalid JSON")
		return
	}
	if req.JSONRPC != "2.0" {
		s.writeError(w, req.ID, -32600, "invalid JSON-RPC version")
		return
	}

	switch req.Method {
	case "tools/list":
		s.handleListTools(w, req)
	case "tools/call":
		s.handleCallTool(w, req)
	default:
		s.writeError(w, req.ID, -32601, "method not found")
	}
}

func (s *Server) writeError(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	resp := rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: msg},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleListTools(w http.ResponseWriter, req rpcRequest) {
	result := map[string]interface{}{
		"tools": toolList,
	}
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleCallTool(w http.ResponseWriter, req rpcRequest) {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(w, req.ID, -32602, "invalid params")
		return
	}

	var res toolResult
	switch params.Name {
	case "list_traces":
		limit := intFromMap(params.Arguments, "limit", 5)
		res = textResult(s.listTraces(limit))
	case "get_trace":
		id, ok := params.Arguments["trace_id"].(string)
		if !ok {
			s.writeError(w, req.ID, -32602, "trace_id required")
			return
		}
		res = textResult(s.getTrace(id))
	case "list_logs":
		limit := intFromMap(params.Arguments, "limit", 5)
		res = textResult(s.listLogs(limit))
	case "get_logs_for_trace":
		id, ok := params.Arguments["trace_id"].(string)
		if !ok {
			s.writeError(w, req.ID, -32602, "trace_id required")
			return
		}
		res = textResult(s.getLogsForTrace(id))
	default:
		s.writeError(w, req.ID, -32601, "unknown tool")
		return
	}

	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res}
	_ = json.NewEncoder(w).Encode(resp)
}

func textResult(text string) toolResult {
	return toolResult{Content: []textContent{{Type: "text", Text: text}}}
}

func intFromMap(m map[string]interface{}, key string, def int) int {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return def
}

func (s *Server) listTraces(limit int) string {
	spans := s.store.GetSvcSpans()
	l := len(*spans)
	if limit <= 0 || limit > l {
		limit = l
	}
	start := l - limit
	if start < 0 {
		start = 0
	}
	var sb strings.Builder
	for i := start; i < l; i++ {
		sd := (*spans)[i]
		fmt.Fprintf(&sb, "%s | %s | %s\n", sd.GetServiceName(), sd.GetSpanName(), sd.Span.TraceID().String())
	}
	return sb.String()
}

func (s *Server) getTrace(id string) string {
	spans, ok := s.store.GetTraceCache().GetSpansByTraceID(id)
	if !ok {
		return "trace not found"
	}
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].Span.StartTimestamp() < spans[j].Span.StartTimestamp()
	})
	var sb strings.Builder
	for _, sp := range spans {
		fmt.Fprintf(&sb, "%s | %s | span:%s parent:%s\n",
			sp.GetServiceName(), sp.GetSpanName(), sp.Span.SpanID().String(), sp.Span.ParentSpanID().String())
	}
	return sb.String()
}

func (s *Server) listLogs(limit int) string {
	logs := *s.store.GetFilteredLogs()
	l := len(logs)
	if limit <= 0 || limit > l {
		limit = l
	}
	start := l - limit
	if start < 0 {
		start = 0
	}
	var sb strings.Builder
	for i := start; i < l; i++ {
		lg := logs[i]
		fmt.Fprintf(&sb, "%s | %s | %s | trace:%s\n",
			lg.GetTimestampText(false), lg.GetServiceName(), lg.GetResolvedBody(), lg.GetTraceID())
	}
	return sb.String()
}

func (s *Server) getLogsForTrace(id string) string {
	logs, ok := s.store.GetLogCache().GetLogsByTraceID(id)
	if !ok {
		return "no logs"
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Log.Timestamp() < logs[j].Log.Timestamp()
	})
	var sb strings.Builder
	for _, lg := range logs {
		fmt.Fprintf(&sb, "%s | %s | %s\n", lg.GetTimestampText(false), lg.GetServiceName(), lg.GetResolvedBody())
	}
	return sb.String()
}

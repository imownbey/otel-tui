package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/ymtdzzz/otel-tui/tuiexporter/internal/telemetry"
)

type Server struct {
	store    *telemetry.Store
	srv      *http.Server
	listener net.Listener
}

func New(addr string, store *telemetry.Store) *Server {
	return &Server{
		store: store,
		srv:   &http.Server{Addr: addr},
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/traces/", s.handleTraces)
	mux.HandleFunc("/logs/", s.handleLogs)
	s.srv.Handler = mux

	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return err
	}
	s.listener = ln

	go func() {
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Printf("mcp server error: %v\n", err)
		}
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *Server) Address() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.srv.Addr
}

type Span struct {
	TraceID  string `json:"trace_id"`
	SpanID   string `json:"span_id"`
	ParentID string `json:"parent_id"`
	Name     string `json:"name"`
	Service  string `json:"service"`
}

func (s *Server) handleTraces(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/traces/")
	spans, ok := s.store.GetTraceCache().GetSpansByTraceID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	res := []Span{}
	for _, sd := range spans {
		res = append(res, Span{
			TraceID:  id,
			SpanID:   sd.Span.SpanID().String(),
			ParentID: sd.Span.ParentSpanID().String(),
			Name:     sd.Span.Name(),
			Service:  telemetry.GetServiceNameFromResource(sd.ResourceSpan.Resource()),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

type Log struct {
	TraceID   string `json:"trace_id"`
	SpanID    string `json:"span_id"`
	Body      string `json:"body"`
	Service   string `json:"service"`
	Severity  string `json:"severity"`
	Timestamp int64  `json:"timestamp"`
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/logs/")
	logs, ok := s.store.GetLogCache().GetLogsByTraceID(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	res := []Log{}
	for _, ld := range logs {
		res = append(res, Log{
			TraceID:   id,
			SpanID:    ld.Log.SpanID().String(),
			Body:      ld.Log.Body().AsString(),
			Service:   telemetry.GetServiceNameFromResource(ld.ResourceLog.Resource()),
			Severity:  ld.Log.SeverityText(),
			Timestamp: ld.Log.Timestamp().AsTime().UnixNano(),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

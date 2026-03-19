package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/app"
	gaalserver "github.com/luanlima/gaal-lib/pkg/server"
	"github.com/luanlima/gaal-lib/pkg/types"
	"github.com/luanlima/gaal-lib/pkg/workflow"
)

type httpServerConfig struct {
	addr    string
	appName string
}

type httpServer struct {
	cfg httpServerConfig

	mu       sync.RWMutex
	addr     string
	runtime  app.Runtime
	server   *http.Server
	listener net.Listener
}

func newHTTPServer(cfg httpServerConfig) *httpServer {
	if strings.TrimSpace(cfg.addr) == "" {
		cfg.addr = defaultAddr
	}
	if strings.TrimSpace(cfg.appName) == "" {
		cfg.appName = defaultAppName
	}
	return &httpServer{cfg: cfg}
}

func (*httpServer) Name() string { return "demo-http" }

func (s *httpServer) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addr
}

func (s *httpServer) BaseURL() string {
	addr := s.Addr()
	if addr == "" {
		return ""
	}
	return "http://" + addr
}

func (s *httpServer) Start(ctx context.Context, rt app.Runtime) error {
	listener, err := net.Listen("tcp", s.cfg.addr)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()

	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data, _ := staticFS.ReadFile("static/index.html")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/readyz", s.handleReady)
	mux.HandleFunc("/agents", s.handleAgents)
	mux.HandleFunc("/agents/", s.handleAgentRoutes)
	mux.HandleFunc("/workflows", s.handleWorkflows)
	mux.HandleFunc("/workflows/", s.handleWorkflowRoutes)

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.mu.Lock()
	s.runtime = rt
	s.server = server
	s.listener = listener
	s.addr = listener.Addr().String()
	s.mu.Unlock()

	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			rt.Logger().ErrorContext(context.Background(), "demo http server stopped unexpectedly", "error", err)
		}
	}()

	return nil
}

func (s *httpServer) Shutdown(ctx context.Context) error {
	s.mu.RLock()
	server := s.server
	s.mu.RUnlock()
	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

type probeResponse struct {
	App      string `json:"app"`
	State    string `json:"state"`
	Health   bool   `json:"health"`
	Ready    bool   `json:"ready"`
	Draining bool   `json:"draining"`
}

type agentsResponse struct {
	Agents []agent.Descriptor `json:"agents"`
}

type runRequest struct {
	SessionID string            `json:"session_id"`
	Message   string            `json:"message"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type toolCallResponse struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Input  map[string]any `json:"input,omitempty"`
	Output string         `json:"output,omitempty"`
}

type runResponse struct {
	RunID     string             `json:"run_id"`
	AgentID   string             `json:"agent_id"`
	SessionID string             `json:"session_id"`
	Output    string             `json:"output"`
	ToolCalls []toolCallResponse `json:"tool_calls,omitempty"`
	Metadata  map[string]string  `json:"metadata,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type streamEvent struct {
	Sequence   int64             `json:"sequence"`
	Type       string            `json:"type"`
	RunID      string            `json:"run_id,omitempty"`
	AgentID    string            `json:"agent_id,omitempty"`
	SessionID  string            `json:"session_id,omitempty"`
	Delta      string            `json:"delta,omitempty"`
	Output     string            `json:"output,omitempty"`
	ToolName   string            `json:"tool_name,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	ToolStatus string            `json:"tool_status,omitempty"`
	ToolInput  map[string]any    `json:"tool_input,omitempty"`
	ToolOutput string            `json:"tool_output,omitempty"`
	Error      string            `json:"error,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type workflowsResponse struct {
	Workflows []workflow.Descriptor `json:"workflows"`
}

type workflowRunRequest struct {
	Item   string  `json:"item"`
	Amount float64 `json:"amount"`
}

type workflowRunResponse struct {
	RunID        string         `json:"run_id"`
	WorkflowName string         `json:"workflow_name"`
	Status       string         `json:"status"`
	Output       map[string]any `json:"output"`
}

func (s *httpServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	probe := s.probe()
	status := http.StatusOK
	if !probe.Health {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, probeResponse{
		App:      s.cfg.appName,
		State:    string(probe.State),
		Health:   probe.Health,
		Ready:    probe.Ready,
		Draining: probe.Draining,
	})
}

func (s *httpServer) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	probe := s.probe()
	status := http.StatusOK
	if !probe.Ready {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, probeResponse{
		App:      s.cfg.appName,
		State:    string(probe.State),
		Health:   probe.Health,
		Ready:    probe.Ready,
		Draining: probe.Draining,
	})
}

func (s *httpServer) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/agents" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	rt := s.getRuntime()
	writeJSON(w, http.StatusOK, agentsResponse{
		Agents: rt.ListAgents(),
	})
}

func (s *httpServer) handleAgentRoutes(w http.ResponseWriter, r *http.Request) {
	name, action, ok := parseSubRoute(r.URL.Path, "/agents/")
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch {
	case action == "runs" && r.Method == http.MethodPost:
		s.handleRun(w, r, name)
	case action == "stream" && r.Method == http.MethodPost:
		s.handleStream(w, r, name)
	case action == "runs":
		s.writeMethodNotAllowed(w, http.MethodPost)
	case action == "stream":
		s.writeMethodNotAllowed(w, http.MethodPost)
	default:
		http.NotFound(w, r)
	}
}

func (s *httpServer) handleRun(w http.ResponseWriter, r *http.Request, name string) {
	ag, err := s.resolveAgent(name)
	if err != nil {
		s.writeResolveError(w, err)
		return
	}

	req, err := decodeRunRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	resp, err := ag.Run(r.Context(), agent.Request{
		SessionID: req.SessionID,
		Messages: []types.Message{
			{Role: types.RoleUser, Content: req.Message},
		},
		Metadata: types.Metadata(req.Metadata),
	})
	if err != nil {
		s.writeAgentError(w, err)
		return
	}

	var calls []toolCallResponse
	for _, tc := range resp.ToolCalls {
		calls = append(calls, toolCallResponse{
			ID:     tc.ID,
			Name:   tc.Name,
			Input:  tc.Input,
			Output: tc.Output.Content,
		})
	}

	writeJSON(w, http.StatusOK, runResponse{
		RunID:     resp.RunID,
		AgentID:   resp.AgentID,
		SessionID: resp.SessionID,
		Output:    resp.Message.Content,
		ToolCalls: calls,
		Metadata:  resp.Metadata,
	})
}

func (s *httpServer) handleStream(w http.ResponseWriter, r *http.Request, name string) {
	ag, err := s.resolveAgent(name)
	if err != nil {
		s.writeResolveError(w, err)
		return
	}

	req, err := decodeRunRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	stream, err := ag.Stream(r.Context(), agent.Request{
		SessionID: req.SessionID,
		Messages: []types.Message{
			{Role: types.RoleUser, Content: req.Message},
		},
		Metadata: types.Metadata(req.Metadata),
	})
	if err != nil {
		s.writeAgentError(w, err)
		return
	}
	defer func() {
		_ = stream.Close()
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "streaming unsupported by response writer"})
		return
	}

	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	for {
		event, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			_ = writeSSEEvent(w, "agent.error", streamEvent{
				Type:  "agent.error",
				Error: err.Error(),
			})
			flusher.Flush()
			return
		}

		if err := writeSSEEvent(w, string(event.Type), toStreamEvent(event)); err != nil {
			return
		}
		flusher.Flush()
	}
}

func toStreamEvent(event agent.Event) streamEvent {
	out := streamEvent{
		Sequence:  event.Sequence,
		Type:      string(event.Type),
		RunID:     event.RunID,
		AgentID:   event.AgentID,
		SessionID: event.SessionID,
		Metadata:  event.Metadata,
	}
	if event.Delta != nil {
		out.Delta = event.Delta.Content
	}
	if event.ToolCall != nil {
		out.ToolName = event.ToolCall.Call.Name
		out.ToolCallID = event.ToolCall.Call.ID
		out.ToolStatus = string(event.ToolCall.Status)
		out.ToolInput = event.ToolCall.Call.Input
		out.ToolOutput = event.ToolCall.Call.Output.Content
	}
	if event.Response != nil {
		out.Output = event.Response.Message.Content
	}
	if event.Err != nil {
		out.Error = event.Err.Error()
	}
	return out
}

func writeSSEEvent(w io.Writer, name string, payload streamEvent) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", body); err != nil {
		return err
	}
	return nil
}

func decodeRunRequest(r *http.Request) (runRequest, error) {
	defer func() {
		_ = r.Body.Close()
	}()

	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return runRequest{}, fmt.Errorf("invalid json body: %w", err)
	}
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.Message = strings.TrimSpace(req.Message)
	if req.SessionID == "" {
		return runRequest{}, errors.New("session_id is required")
	}
	if req.Message == "" {
		return runRequest{}, errors.New("message is required")
	}
	return req, nil
}

func (s *httpServer) resolveAgent(name string) (*agent.Agent, error) {
	return s.getRuntime().ResolveAgent(name)
}

func (s *httpServer) getRuntime() app.Runtime {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runtime
}

func (s *httpServer) probe() gaalserver.Probe {
	rt := s.getRuntime()
	if rt == nil {
		return gaalserver.Snapshot(app.StateStopped)
	}
	return gaalserver.Snapshot(rt.State())
}

func (s *httpServer) writeMethodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
}

func (s *httpServer) writeResolveError(w http.ResponseWriter, err error) {
	if errors.Is(err, app.ErrAgentNotFound) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
}

func (s *httpServer) writeAgentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agent.ErrGuardrailBlocked):
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: err.Error()})
	case errors.Is(err, agent.ErrInvalidRequest):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeJSON(w, http.StatusGatewayTimeout, errorResponse{Error: err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
	}
}

func (s *httpServer) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/workflows" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	rt := s.getRuntime()
	writeJSON(w, http.StatusOK, workflowsResponse{
		Workflows: rt.ListWorkflows(),
	})
}

func (s *httpServer) handleWorkflowRoutes(w http.ResponseWriter, r *http.Request) {
	name, action, ok := parseSubRoute(r.URL.Path, "/workflows/")
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch {
	case action == "runs" && r.Method == http.MethodPost:
		s.handleWorkflowRun(w, r, name)
	case action == "runs":
		s.writeMethodNotAllowed(w, http.MethodPost)
	default:
		http.NotFound(w, r)
	}
}

func (s *httpServer) handleWorkflowRun(w http.ResponseWriter, r *http.Request, name string) {
	rt := s.getRuntime()
	wf, err := rt.ResolveWorkflow(name)
	if err != nil {
		if errors.Is(err, app.ErrWorkflowNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	req, err := decodeWorkflowRunRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	resp, err := wf.Run(r.Context(), workflow.Request{
		Input: map[string]any{
			"item":   req.Item,
			"amount": req.Amount,
		},
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, workflowRunResponse{
		RunID:        resp.RunID,
		WorkflowName: resp.WorkflowName,
		Status:       string(resp.Status),
		Output:       resp.Output,
	})
}

func decodeWorkflowRunRequest(r *http.Request) (workflowRunRequest, error) {
	defer func() {
		_ = r.Body.Close()
	}()

	var req workflowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return workflowRunRequest{}, fmt.Errorf("invalid json body: %w", err)
	}
	req.Item = strings.TrimSpace(req.Item)
	if req.Item == "" {
		return workflowRunRequest{}, errors.New("item is required")
	}
	if req.Amount <= 0 {
		return workflowRunRequest{}, errors.New("amount must be positive")
	}
	return req, nil
}

func parseSubRoute(path, prefix string) (name string, action string, ok bool) {
	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

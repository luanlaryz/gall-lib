package demoapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/logger"
	"github.com/luanlima/gaal-lib/pkg/memory"
	gaalserver "github.com/luanlima/gaal-lib/pkg/server"
	"github.com/luanlima/gaal-lib/pkg/types"
)

const (
	defaultAddr      = "127.0.0.1:8080"
	defaultAppName   = "demo-app"
	defaultAgentName = "demo-agent"
)

// Config defines the local demo wiring.
type Config struct {
	AppName   string
	AgentName string
	Addr      string
	LogLevel  logger.Level
}

// DefaultConfig returns safe local defaults for the demo.
func DefaultConfig() Config {
	return Config{
		AppName:   defaultAppName,
		AgentName: defaultAgentName,
		Addr:      defaultAddr,
		LogLevel:  logger.LevelInfo,
	}
}

// ConfigFromEnv loads supported demo configuration from environment variables.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	if value := strings.TrimSpace(os.Getenv("DEMO_APP_NAME")); value != "" {
		cfg.AppName = value
	}
	if value := strings.TrimSpace(os.Getenv("DEMO_AGENT_NAME")); value != "" {
		cfg.AgentName = value
	}
	if value := strings.TrimSpace(os.Getenv("DEMO_APP_ADDR")); value != "" {
		cfg.Addr = value
	}
	if value := strings.TrimSpace(os.Getenv("DEMO_LOG_LEVEL")); value != "" {
		cfg.LogLevel = logger.Level(strings.ToLower(value))
	}
	return cfg
}

// Bundle groups the materialized demo app and its managed HTTP server.
type Bundle struct {
	App    *app.App
	Server *HTTPServer
	Config Config
}

// New builds the demo app composition using only public gaal-lib packages.
func New(cfg Config) (*Bundle, error) {
	cfg = normalizeConfig(cfg)

	httpServer := NewHTTPServer(HTTPServerConfig{
		Addr:    cfg.Addr,
		AppName: cfg.AppName,
	})

	instance, err := app.New(
		app.Config{
			Name: cfg.AppName,
			Defaults: app.Defaults{
				Logger: logger.NewSimple(logger.SimpleOptions{
					Level: cfg.LogLevel,
				}),
				Agent: app.AgentDefaults{
					Memory: &memory.InMemoryStore{},
				},
			},
		},
		app.WithAgentFactories(AgentFactory{AgentName: cfg.AgentName}),
		app.WithServers(httpServer),
	)
	if err != nil {
		return nil, err
	}

	return &Bundle{
		App:    instance,
		Server: httpServer,
		Config: cfg,
	}, nil
}

func normalizeConfig(cfg Config) Config {
	if strings.TrimSpace(cfg.AppName) == "" {
		cfg.AppName = defaultAppName
	}
	if strings.TrimSpace(cfg.AgentName) == "" {
		cfg.AgentName = defaultAgentName
	}
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = defaultAddr
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = logger.LevelInfo
	}
	return cfg
}

// AgentFactory builds the deterministic demo agent.
type AgentFactory struct {
	AgentName string
}

// Name returns the logical demo agent name.
func (f AgentFactory) Name() string {
	name := strings.TrimSpace(f.AgentName)
	if name == "" {
		return defaultAgentName
	}
	return name
}

// Build materializes the demo agent from frozen app defaults.
func (f AgentFactory) Build(_ context.Context, defaults app.AgentDefaults) (*agent.Agent, error) {
	opts := []agent.Option{
		agent.WithExecutionEngine(defaults.Engine),
		agent.WithMaxSteps(defaults.MaxSteps),
	}
	if len(defaults.Metadata) > 0 {
		opts = append(opts, agent.WithMetadata(defaults.Metadata))
	}
	if defaults.Memory != nil {
		opts = append(opts, agent.WithMemory(defaults.Memory))
	}
	if defaults.WorkingMemory != nil {
		opts = append(opts, agent.WithWorkingMemory(defaults.WorkingMemory))
	}
	if len(defaults.InputGuardrails) > 0 {
		opts = append(opts, agent.WithInputGuardrails(defaults.InputGuardrails...))
	}
	if len(defaults.StreamGuardrails) > 0 {
		opts = append(opts, agent.WithStreamGuardrails(defaults.StreamGuardrails...))
	}
	if len(defaults.OutputGuardrails) > 0 {
		opts = append(opts, agent.WithOutputGuardrails(defaults.OutputGuardrails...))
	}
	if len(defaults.Hooks) > 0 {
		opts = append(opts, agent.WithHooks(defaults.Hooks...))
	}

	return agent.New(
		agent.Config{
			Name:         f.Name(),
			Instructions: "Reply briefly using local demo logic and acknowledge previous memory when the session already exists.",
			Model:        demoModel{},
		},
		opts...,
	)
}

type demoModel struct{}

func (demoModel) Generate(_ context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
	content := responseTextFromRequest(req)
	return agent.ModelResponse{
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: content,
		},
	}, nil
}

func (demoModel) Stream(_ context.Context, req agent.ModelRequest) (agent.ModelStream, error) {
	content := responseTextFromRequest(req)
	return &demoModelStream{
		events: streamEventsFromContent(content),
	}, nil
}

type demoModelStream struct {
	events []agent.ModelEvent
	index  int
}

func (s *demoModelStream) Recv() (agent.ModelEvent, error) {
	if s.index >= len(s.events) {
		return agent.ModelEvent{}, io.EOF
	}
	event := s.events[s.index]
	s.index++
	return event, nil
}

func (*demoModelStream) Close() error { return nil }

func responseTextFromRequest(req agent.ModelRequest) string {
	message := lastUserMessage(req.Messages)
	if len(req.Memory.Messages) > 0 {
		return fmt.Sprintf("welcome back, %s", message)
	}
	return fmt.Sprintf("hello, %s", message)
}

func lastUserMessage(messages []types.Message) string {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role != types.RoleUser {
			continue
		}
		text := strings.TrimSpace(messages[index].Content)
		if text == "" {
			return "friend"
		}
		return text
	}
	return "friend"
}

func streamEventsFromContent(content string) []agent.ModelEvent {
	if content == "" {
		content = "hello, friend"
	}
	chunks := splitContent(content)
	events := make([]agent.ModelEvent, 0, len(chunks)+1)
	for _, chunk := range chunks {
		events = append(events, agent.ModelEvent{
			Delta: &types.MessageDelta{
				Role:    types.RoleAssistant,
				Content: chunk,
			},
		})
	}
	events = append(events, agent.ModelEvent{
		Message: &types.Message{
			Role:    types.RoleAssistant,
			Content: content,
		},
		Done: true,
	})
	return events
}

func splitContent(content string) []string {
	words := strings.Fields(content)
	if len(words) <= 1 {
		return []string{content}
	}

	chunks := make([]string, 0, len(words))
	for index, word := range words {
		if index == 0 {
			chunks = append(chunks, word)
			continue
		}
		chunks = append(chunks, " "+word)
	}
	return chunks
}

// HTTPServerConfig configures the demo HTTP adapter.
type HTTPServerConfig struct {
	Addr    string
	AppName string
}

// HTTPServer is the managed long-lived server used by the demo.
type HTTPServer struct {
	cfg HTTPServerConfig

	mu       sync.RWMutex
	addr     string
	runtime  app.Runtime
	server   *http.Server
	listener net.Listener
}

// NewHTTPServer builds the demo HTTP adapter.
func NewHTTPServer(cfg HTTPServerConfig) *HTTPServer {
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = defaultAddr
	}
	if strings.TrimSpace(cfg.AppName) == "" {
		cfg.AppName = defaultAppName
	}
	return &HTTPServer{cfg: cfg}
}

// Name reports the logical server name.
func (*HTTPServer) Name() string { return "demo-http" }

// Addr returns the bound listener address after Start.
func (s *HTTPServer) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addr
}

// BaseURL returns the local base URL once the listener is active.
func (s *HTTPServer) BaseURL() string {
	addr := s.Addr()
	if addr == "" {
		return ""
	}
	return "http://" + addr
}

// Start binds the listener and starts serving requests.
func (s *HTTPServer) Start(ctx context.Context, rt app.Runtime) error {
	listener, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/readyz", s.handleReady)
	mux.HandleFunc("/agents", s.handleAgents)
	mux.HandleFunc("/agents/", s.handleAgentRoutes)

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

// Shutdown stops the listener cooperatively.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
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

type runResponse struct {
	RunID     string            `json:"run_id"`
	AgentID   string            `json:"agent_id"`
	SessionID string            `json:"session_id"`
	Output    string            `json:"output"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type streamEvent struct {
	Sequence  int64             `json:"sequence"`
	Type      string            `json:"type"`
	RunID     string            `json:"run_id,omitempty"`
	AgentID   string            `json:"agent_id,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	Delta     string            `json:"delta,omitempty"`
	Output    string            `json:"output,omitempty"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
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
		App:      s.cfg.AppName,
		State:    string(probe.State),
		Health:   probe.Health,
		Ready:    probe.Ready,
		Draining: probe.Draining,
	})
}

func (s *HTTPServer) handleReady(w http.ResponseWriter, r *http.Request) {
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
		App:      s.cfg.AppName,
		State:    string(probe.State),
		Health:   probe.Health,
		Ready:    probe.Ready,
		Draining: probe.Draining,
	})
}

func (s *HTTPServer) handleAgents(w http.ResponseWriter, r *http.Request) {
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

func (s *HTTPServer) handleAgentRoutes(w http.ResponseWriter, r *http.Request) {
	name, action, ok := parseAgentRoute(r.URL.Path)
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

func parseAgentRoute(path string) (name string, action string, ok bool) {
	trimmed := strings.TrimPrefix(path, "/agents/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (s *HTTPServer) handleRun(w http.ResponseWriter, r *http.Request, name string) {
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

	writeJSON(w, http.StatusOK, runResponse{
		RunID:     resp.RunID,
		AgentID:   resp.AgentID,
		SessionID: resp.SessionID,
		Output:    resp.Message.Content,
		Metadata:  resp.Metadata,
	})
}

func (s *HTTPServer) handleStream(w http.ResponseWriter, r *http.Request, name string) {
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

func (s *HTTPServer) resolveAgent(name string) (*agent.Agent, error) {
	return s.getRuntime().ResolveAgent(name)
}

func (s *HTTPServer) getRuntime() app.Runtime {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runtime
}

func (s *HTTPServer) probe() gaalserver.Probe {
	rt := s.getRuntime()
	if rt == nil {
		return gaalserver.Snapshot(app.StateStopped)
	}
	return gaalserver.Snapshot(rt.State())
}

func (s *HTTPServer) writeMethodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
}

func (s *HTTPServer) writeResolveError(w http.ResponseWriter, err error) {
	if errors.Is(err, app.ErrAgentNotFound) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
}

func (s *HTTPServer) writeAgentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agent.ErrInvalidRequest):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeJSON(w, http.StatusGatewayTimeout, errorResponse{Error: err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

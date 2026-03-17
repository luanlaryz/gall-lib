package smoke_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/luanlima/gaal-lib/internal/demoapp"
	"github.com/luanlima/gaal-lib/pkg/agent"
)

func TestDemoAppBootHealthAgentsAndRuns(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bundle, err := demoapp.New(demoapp.Config{
		AppName:   "demo-app-smoke",
		AgentName: "demo-agent",
		Addr:      "127.0.0.1:0",
		LogLevel:  "error",
	})
	if err != nil {
		t.Fatalf("demoapp.New() error = %v", err)
	}

	if err := bundle.App.Start(ctx); err != nil {
		t.Fatalf("App.Start() error = %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := bundle.App.Shutdown(shutdownCtx); err != nil {
			t.Fatalf("App.Shutdown() error = %v", err)
		}
	})

	if bundle.Server.BaseURL() == "" {
		t.Fatalf("BaseURL() = empty")
	}

	client := &http.Client{Timeout: 5 * time.Second}

	health := getJSON[probeResponse](t, client, bundle.Server.BaseURL()+"/healthz")
	if !health.Health {
		t.Fatalf("health.Health = false want true")
	}
	if health.State != "running" {
		t.Fatalf("health.State = %q want running", health.State)
	}

	ready := getJSON[probeResponse](t, client, bundle.Server.BaseURL()+"/readyz")
	if !ready.Ready {
		t.Fatalf("ready.Ready = false want true")
	}

	list := getJSON[agentsResponse](t, client, bundle.Server.BaseURL()+"/agents")
	if len(list.Agents) != 1 {
		t.Fatalf("len(Agents) = %d want 1", len(list.Agents))
	}
	if list.Agents[0].Name != bundle.Config.AgentName {
		t.Fatalf("agent name = %q want %q", list.Agents[0].Name, bundle.Config.AgentName)
	}

	first := postJSON[runResponse](t, client, bundle.Server.BaseURL()+"/agents/"+bundle.Config.AgentName+"/runs", runRequest{
		SessionID: "session-1",
		Message:   "Ada",
		Metadata: map[string]string{
			"user_id":         "user-1",
			"conversation_id": "conv-1",
		},
	})
	if first.Output != "hello, Ada" {
		t.Fatalf("first.Output = %q want %q", first.Output, "hello, Ada")
	}
	if first.AgentID == "" {
		t.Fatalf("first.AgentID = empty")
	}

	second := postJSON[runResponse](t, client, bundle.Server.BaseURL()+"/agents/"+bundle.Config.AgentName+"/runs", runRequest{
		SessionID: "session-1",
		Message:   "Ada",
		Metadata: map[string]string{
			"user_id":         "user-1",
			"conversation_id": "conv-1",
		},
	})
	if second.Output != "welcome back, Ada" {
		t.Fatalf("second.Output = %q want %q", second.Output, "welcome back, Ada")
	}
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

func getJSON[T any](t *testing.T, client *http.Client, url string) T {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d want %d", url, resp.StatusCode, http.StatusOK)
	}

	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	return out
}

func postJSON[T any](t *testing.T, client *http.Client, url string, payload any) T {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s status = %d want %d", url, resp.StatusCode, http.StatusOK)
	}

	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	return out
}

package smoke_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var demoBinary string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "demo-smoke-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	binary := filepath.Join(tmpDir, "demo-app")
	root := findModuleRoot()

	build := exec.Command("go", "build", "-o", binary, "./cmd/demo-app")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build ./cmd/demo-app: %v\n%s\n", err, out)
		os.Exit(1)
	}

	demoBinary = binary
	os.Exit(m.Run())
}

func TestDemoApp(t *testing.T) {
	t.Parallel()

	addr := startDemo(t)
	baseURL := "http://" + addr
	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("healthz", func(t *testing.T) {
		resp := getJSON[probeResponse](t, client, baseURL+"/healthz")
		if !resp.Health {
			t.Fatalf("health = false want true")
		}
		if resp.State != "running" {
			t.Fatalf("state = %q want running", resp.State)
		}
	})

	t.Run("readyz", func(t *testing.T) {
		resp := getJSON[probeResponse](t, client, baseURL+"/readyz")
		if !resp.Ready {
			t.Fatalf("ready = false want true")
		}
	})

	t.Run("agents", func(t *testing.T) {
		resp := getJSON[agentsResponse](t, client, baseURL+"/agents")
		if len(resp.Agents) != 1 {
			t.Fatalf("len(agents) = %d want 1", len(resp.Agents))
		}
		if resp.Agents[0].Name != "demo-agent" {
			t.Fatalf("agent name = %q want demo-agent", resp.Agents[0].Name)
		}
	})

	t.Run("text_run_with_memory", func(t *testing.T) {
		first := postJSON[runResponse](t, client, baseURL+"/agents/demo-agent/runs", runRequest{
			SessionID: "session-text-1",
			Message:   "Ada",
			Metadata:  map[string]string{"user_id": "user-1", "conversation_id": "conv-1"},
		})
		if first.Output != "hello, Ada" {
			t.Fatalf("first output = %q want %q", first.Output, "hello, Ada")
		}
		if first.AgentID == "" {
			t.Fatal("first AgentID = empty")
		}

		second := postJSON[runResponse](t, client, baseURL+"/agents/demo-agent/runs", runRequest{
			SessionID: "session-text-1",
			Message:   "Ada",
			Metadata:  map[string]string{"user_id": "user-1", "conversation_id": "conv-1"},
		})
		if second.Output != "welcome back, Ada" {
			t.Fatalf("second output = %q want %q", second.Output, "welcome back, Ada")
		}
	})

	t.Run("streaming_sse", func(t *testing.T) {
		events := postSSE(t, client, baseURL+"/agents/demo-agent/stream", runRequest{
			SessionID: "session-sse-1",
			Message:   "Grace",
			Metadata:  map[string]string{"user_id": "user-1"},
		})
		if len(events) < 3 {
			t.Fatalf("got %d SSE events, want at least 3", len(events))
		}
		if events[0].name != "agent.started" {
			t.Fatalf("first event = %q want agent.started", events[0].name)
		}
		hasDelta := false
		for _, e := range events {
			if e.name == "agent.delta" {
				hasDelta = true
				break
			}
		}
		if !hasDelta {
			t.Fatal("no agent.delta event found")
		}
		last := events[len(events)-1]
		if last.name != "agent.completed" {
			t.Fatalf("last event = %q want agent.completed", last.name)
		}
	})

	t.Run("agent_not_found_404", func(t *testing.T) {
		status, _, errResp := postRaw(t, client, baseURL+"/agents/missing-agent/runs", runRequest{
			SessionID: "s1",
			Message:   "hi",
		})
		if status != http.StatusNotFound {
			t.Fatalf("status = %d want 404", status)
		}
		if errResp.Error == "" {
			t.Fatal("expected non-empty error message")
		}
	})

	t.Run("invalid_request_400", func(t *testing.T) {
		status, _, errResp := postRaw(t, client, baseURL+"/agents/demo-agent/runs", runRequest{
			SessionID: "",
			Message:   "hi",
		})
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d want 400", status)
		}
		if errResp.Error == "" {
			t.Fatal("expected non-empty error message")
		}
	})

	t.Run("method_not_allowed_405", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/agents/demo-agent/runs")
		if err != nil {
			t.Fatalf("GET error: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d want 405", resp.StatusCode)
		}
		if allow := resp.Header.Get("Allow"); allow != "POST" {
			t.Fatalf("Allow = %q want POST", allow)
		}
	})
}

func TestDemoAppMemoryResetAfterRestart(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 5 * time.Second}

	addr1 := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	cmd1 := startDemoAt(t, addr1)
	runURL1 := "http://" + addr1 + "/agents/demo-agent/runs"

	first := postJSON[runResponse](t, client, runURL1, runRequest{
		SessionID: "persist-test",
		Message:   "Ada",
	})
	if first.Output != "hello, Ada" {
		t.Fatalf("first = %q want %q", first.Output, "hello, Ada")
	}

	second := postJSON[runResponse](t, client, runURL1, runRequest{
		SessionID: "persist-test",
		Message:   "Ada",
	})
	if second.Output != "welcome back, Ada" {
		t.Fatalf("second = %q want %q", second.Output, "welcome back, Ada")
	}

	stopDemo(cmd1)

	addr2 := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	cmd2 := startDemoAt(t, addr2)
	t.Cleanup(func() { stopDemo(cmd2) })

	runURL2 := "http://" + addr2 + "/agents/demo-agent/runs"

	afterRestart := postJSON[runResponse](t, client, runURL2, runRequest{
		SessionID: "persist-test",
		Message:   "Ada",
	})
	if afterRestart.Output != "hello, Ada" {
		t.Fatalf("after restart = %q want %q (memory should be cleared)", afterRestart.Output, "hello, Ada")
	}
}

// --- types ---

type probeResponse struct {
	App      string `json:"app"`
	State    string `json:"state"`
	Health   bool   `json:"health"`
	Ready    bool   `json:"ready"`
	Draining bool   `json:"draining"`
}

type agentDescriptor struct {
	Name string
	ID   string
}

type agentsResponse struct {
	Agents []agentDescriptor `json:"agents"`
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

type sseEvent struct {
	name string
	data string
}

// --- process helpers ---

func startDemo(t *testing.T) string {
	t.Helper()
	addr := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	cmd := startDemoAt(t, addr)
	t.Cleanup(func() { stopDemo(cmd) })
	return addr
}

func startDemoAt(t *testing.T, addr string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(demoBinary)
	cmd.Env = buildDemoEnv(addr)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start demo: %v", err)
	}
	waitHealthy(t, "http://"+addr+"/healthz")
	return cmd
}

func stopDemo(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Signal(os.Interrupt)
	}
	_ = cmd.Wait()
}

func buildDemoEnv(addr string) []string {
	skip := map[string]bool{
		"DEMO_APP_ADDR": true, "DEMO_APP_NAME": true,
		"DEMO_AGENT_NAME": true, "DEMO_LOG_LEVEL": true,
	}
	var env []string
	for _, e := range os.Environ() {
		if key, _, _ := strings.Cut(e, "="); !skip[key] {
			env = append(env, e)
		}
	}
	return append(env,
		"DEMO_APP_ADDR="+addr,
		"DEMO_APP_NAME=demo-app-smoke",
		"DEMO_AGENT_NAME=demo-agent",
		"DEMO_LOG_LEVEL=error",
	)
}

func waitHealthy(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server not healthy within timeout: %s", url)
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func findModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		panic("os.Getwd: " + err.Error())
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find module root")
		}
		dir = parent
	}
}

// --- HTTP helpers ---

func getJSON[T any](t *testing.T, client *http.Client, url string) T {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d want 200", url, resp.StatusCode)
	}
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func postJSON[T any](t *testing.T, client *http.Client, url string, payload any) T {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s status = %d want 200", url, resp.StatusCode)
	}
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func postRaw(t *testing.T, client *http.Client, url string, payload any) (int, http.Header, errorResponse) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	var errResp errorResponse
	_ = json.NewDecoder(resp.Body).Decode(&errResp)
	return resp.StatusCode, resp.Header, errResp
}

func postSSE(t *testing.T, client *http.Client, url string, payload runRequest) []sseEvent {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s status = %d want 200", url, resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q want text/event-stream*", ct)
	}

	var events []sseEvent
	scanner := bufio.NewScanner(resp.Body)
	var current sseEvent
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			current.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			current.data = strings.TrimPrefix(line, "data: ")
		case line == "":
			if current.name != "" || current.data != "" {
				events = append(events, current)
				current = sseEvent{}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan SSE: %v", err)
	}
	return events
}

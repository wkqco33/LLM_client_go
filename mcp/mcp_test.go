package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// ─── HttpClient Tests ──────────────────────────────────────────

func TestHttpClient_ListTools_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tools" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tools":[{"name":"read_file","description":"Read file contents","inputSchema":{"type":"object"}}]}`))
	}))
	t.Cleanup(srv.Close)

	client := NewHttpClient(srv.URL)
	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("got %d tools, want 1", len(tools))
	}
	if tools[0].Name != "read_file" {
		t.Errorf("got tool name %q, want read_file", tools[0].Name)
	}
}

func TestHttpClient_ListTools_Non200_ErrUnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := NewHttpClient(srv.URL)
	_, err := client.ListTools(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrUnexpectedStatus) {
		t.Errorf("expected ErrUnexpectedStatus, got %v", err)
	}
}

func TestHttpClient_CallTool_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tools/call" {
			http.NotFound(w, r)
			return
		}
		var req CallToolRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Name != "echo" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"hello world"}]}`))
	}))
	t.Cleanup(srv.Close)

	client := NewHttpClient(srv.URL)
	resp, err := client.CallTool(context.Background(), "echo", map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text != "hello world" {
		t.Errorf("got unexpected response: %+v", resp)
	}
}

func TestHttpClient_IsAlive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := NewHttpClient(srv.URL)
	if !client.IsAlive() {
		t.Errorf("expected client to be alive")
	}

	deadClient := NewHttpClient("http://127.0.0.1:0") // non-listening port
	if deadClient.IsAlive() {
		t.Errorf("expected deadClient to not be alive")
	}
}

// ─── Bridge & WrapTools Tests ──────────────────────────────────

type mockProvider struct {
	alive    bool
	tools    []Tool
	callFunc func(ctx context.Context, name string, arguments map[string]any) (*CallToolResponse, error)
}

func (m *mockProvider) ListTools(ctx context.Context) ([]Tool, error) {
	return m.tools, nil
}

func (m *mockProvider) CallTool(ctx context.Context, name string, arguments map[string]any) (*CallToolResponse, error) {
	if m.callFunc != nil {
		return m.callFunc(ctx, name, arguments)
	}
	return &CallToolResponse{
		Content: []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{
			{Type: "text", Text: "mocked result"},
		},
	}, nil
}

func (m *mockProvider) IsAlive() bool {
	return m.alive
}

func TestWrapTools_And_Execution(t *testing.T) {
	mock := &mockProvider{
		alive: true,
		tools: []Tool{
			{Name: "search", Description: "search files", InputSchema: map[string]any{"type": "object"}},
		},
	}

	wrapped := WrapTools(mock, mock.tools)
	if len(wrapped) != 1 {
		t.Fatalf("got %d tools, want 1", len(wrapped))
	}

	tool := wrapped[0]
	def := tool.Definition()
	if def.Function.Name != "search" || def.Function.Description != "search files" {
		t.Errorf("unexpected tool definition: %+v", def)
	}

	// Successful execution
	out, err := tool.Execute(context.Background(), `{"query":"test"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "mocked result" {
		t.Errorf("got %q, want %q", out, "mocked result")
	}

	// Server unreachable
	mock.alive = false
	out, _ = tool.Execute(context.Background(), `{"query":"test"}`)
	if out != "Error: MCP server is not reachable" {
		t.Errorf("got %q, want unreachable error message", out)
	}

	// Invalid arguments JSON
	mock.alive = true
	out, _ = tool.Execute(context.Background(), `invalid-json`)
	if out == "" || out[:5] != "Error" {
		t.Errorf("expected argument error, got %q", out)
	}
}

// ─── StdioClient Helper Process Test ───────────────────────────

func TestStdioClient_HelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_MCP_HELPER") == "1" {
		// Mock MCP stdio server helper process
		for {
			var req jsonRPCRequest
			if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
				os.Exit(0)
			}
			if req.Method == "tools/list" {
				resp := jsonRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  json.RawMessage(`{"tools":[{"name":"test_tool","description":"test","inputSchema":{}}]}`),
				}
				data, _ := json.Marshal(resp)
				fmt.Println(string(data))
			} else if req.Method == "tools/call" {
				resp := jsonRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  json.RawMessage(`{"content":[{"type":"text","text":"stdio success"}]}`),
				}
				data, _ := json.Marshal(resp)
				fmt.Println(string(data))
			}
		}
	}

	t.Setenv("GO_TEST_MCP_HELPER", "1")
	// Start helper process as stdio server
	client, err := NewStdioClientWithConfig(StdioConfig{
		Timeout: 2 * time.Second,
	}, os.Args[0], "-test.run=TestStdioClient_HelperProcess")
	if err != nil {
		t.Fatalf("failed to start stdio helper: %v", err)
	}
	defer client.Close()

	if !client.IsAlive() {
		t.Fatal("expected stdio client to be alive")
	}

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "test_tool" {
		t.Errorf("unexpected tools: %+v", tools)
	}

	callResp, err := client.CallTool(context.Background(), "test_tool", map[string]any{})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if len(callResp.Content) != 1 || callResp.Content[0].Text != "stdio success" {
		t.Errorf("unexpected callResp: %+v", callResp)
	}
}

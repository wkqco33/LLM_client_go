package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"

	llm "llm-client-go"
	"llm-client-go/agent"
)

// ─── Common MCP Types ──────────────────────────────────────────

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

type CallToolRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type CallToolResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError,omitempty"`
}

// Provider is an interface for different MCP transport implementations.
type Provider interface {
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, arguments map[string]any) (*CallToolResponse, error)
	IsAlive() bool
}

// ─── HTTP Client Implementation ───────────────────────────────

type HttpClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHttpClient(baseURL string) *HttpClient {
	return &HttpClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// IsAlive performs a bounded-time GET against the tools endpoint to check
// whether the MCP HTTP server is reachable. It uses its own short timeout
// independent of the client's configured request timeout, so a hung server
// doesn't block callers gating on liveness.
func (c *HttpClient) IsAlive() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/tools", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < http.StatusInternalServerError
}

func (c *HttpClient) ListTools(ctx context.Context) ([]Tool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/tools", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcp http: %w (status %d)", ErrUnexpectedStatus, resp.StatusCode)
	}

	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func (c *HttpClient) CallTool(ctx context.Context, name string, arguments map[string]any) (*CallToolResponse, error) {
	payload, err := json.Marshal(CallToolRequest{Name: name, Arguments: arguments})
	if err != nil {
		return nil, fmt.Errorf("mcp http: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/tools/call", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("mcp http: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcp http: %w (status %d)", ErrUnexpectedStatus, resp.StatusCode)
	}

	var result CallToolResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ─── Stdio Client Implementation (JSON-RPC) ────────────────────

const defaultStdioTimeout = 30 * time.Second

// StdioConfig holds configuration for StdioClient.
type StdioConfig struct {
	// Timeout is the maximum time to wait for a response from the MCP server.
	// Defaults to 30 seconds.
	Timeout time.Duration

	// Logger receives diagnostic messages: lines from the server that
	// couldn't be parsed as JSON-RPC, and why the read loop stopped.
	// StdioClient is silent by default (nil Logger), matching the rest of
	// this module — set one to make otherwise-invisible transport problems
	// debuggable.
	Logger *log.Logger
}

type StdioClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	logger *log.Logger

	timeout  time.Duration
	mu       sync.Mutex
	id       int
	pending  map[int]chan jsonRPCResponse
	isClosed bool
}

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *jsonRPCError) Error() string {
	return fmt.Sprintf("MCP Error (%d): %s", e.Code, e.Message)
}

func NewStdioClient(command string, args ...string) (*StdioClient, error) {
	return NewStdioClientWithConfig(StdioConfig{}, command, args...)
}

// NewStdioClientWithConfig creates a StdioClient with custom configuration.
func NewStdioClientWithConfig(cfg StdioConfig, command string, args ...string) (*StdioClient, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultStdioTimeout
	}

	logger := cfg.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	c := &StdioClient{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewScanner(stdout),
		logger:  logger,
		timeout: timeout,
		pending: make(map[int]chan jsonRPCResponse),
	}

	go c.readLoop()
	return c, nil
}

func (c *StdioClient) IsAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.isClosed && c.cmd.Process != nil && c.cmd.ProcessState == nil
}

func (c *StdioClient) readLoop() {
	for c.stdout.Scan() {
		line := c.stdout.Bytes()
		var resp jsonRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			c.logger.Printf("mcp stdio: discarding unparseable line: %v (%q)", err, line)
			continue
		}

		c.mu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.mu.Unlock()
		if ok {
			ch <- resp
		}
	}
	if err := c.stdout.Err(); err != nil {
		c.logger.Printf("mcp stdio: read loop stopped: %v", err)
	} else {
		c.logger.Printf("mcp stdio: server closed its stdout, treating connection as dead")
	}

	c.mu.Lock()
	c.isClosed = true
	// Close all pending requests with error
	for id, ch := range c.pending {
		delete(c.pending, id)
		ch <- jsonRPCResponse{Error: &jsonRPCError{Code: -1, Message: "stdio pipe closed"}}
	}
	c.mu.Unlock()
}

func (c *StdioClient) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if !c.IsAlive() {
		return nil, fmt.Errorf("mcp stdio: %w", ErrServerUnreachable)
	}

	c.mu.Lock()
	c.id++
	id := c.id
	ch := make(chan jsonRPCResponse, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("mcp stdio: marshal request: %w", err)
	}
	fmt.Fprintln(c.stdin, string(data))

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(c.timeout):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("mcp stdio: %w", ErrRequestTimeout)
	}
}

func (c *StdioClient) ListTools(ctx context.Context) ([]Tool, error) {
	raw, err := c.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func (c *StdioClient) CallTool(ctx context.Context, name string, arguments map[string]any) (*CallToolResponse, error) {
	params := map[string]any{
		"name":      name,
		"arguments": arguments,
	}
	raw, err := c.call(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}
	var result CallToolResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *StdioClient) Close() error {
	c.mu.Lock()
	if c.isClosed {
		c.mu.Unlock()
		return nil
	}
	c.isClosed = true
	c.mu.Unlock()

	c.stdin.Close()
	return c.cmd.Process.Kill()
}

// ─── Bridge ───────────────────────────────────────────────────

type toolBridge struct {
	provider Provider
	tool     Tool
}

func (b *toolBridge) Definition() llm.Tool {
	return llm.Tool{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        b.tool.Name,
			Description: b.tool.Description,
			Parameters:  b.tool.InputSchema,
		},
	}
}

func (b *toolBridge) Execute(ctx context.Context, arguments string) (string, error) {
	if !b.provider.IsAlive() {
		return "Error: MCP server is not reachable", nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err), nil
	}

	resp, err := b.provider.CallTool(ctx, b.tool.Name, args)
	if err != nil {
		// Return the error so the LLM can see it
		return fmt.Sprintf("Tool execution failed: %v", err), nil
	}

	var output string
	for _, part := range resp.Content {
		output += part.Text
	}

	if resp.IsError {
		return fmt.Sprintf("Tool returned an error: %s", output), nil
	}

	if output == "" {
		return "Tool executed successfully but returned no output", nil
	}

	return output, nil
}

func WrapTools(p Provider, tools []Tool) []agent.ExecutableTool {
	result := make([]agent.ExecutableTool, 0, len(tools))
	for _, t := range tools {
		result = append(result, &toolBridge{provider: p, tool: t})
	}
	return result
}

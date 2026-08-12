package mcp

import "errors"

// Sentinel errors for common MCP transport failure conditions, so callers
// can use errors.Is instead of matching on message text.
var (
	// ErrServerUnreachable is returned when a request is attempted against
	// an MCP server that is not currently reachable (stdio: process exited;
	// http: connection failed).
	ErrServerUnreachable = errors.New("mcp: server is not reachable")

	// ErrRequestTimeout is returned when a request exceeds its configured
	// timeout without a response.
	ErrRequestTimeout = errors.New("mcp: request timed out")

	// ErrUnexpectedStatus is returned when an HTTP-transport MCP server
	// responds with a non-200 status code.
	ErrUnexpectedStatus = errors.New("mcp: unexpected HTTP status")
)

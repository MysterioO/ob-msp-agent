# Monitoring MCP Server - Developer Instructions

This document provides context and guidelines for Gemini to effectively assist in the development and expansion of the Monitoring MCP Server Sandbox.

## Project Nature
This is a **learning sandbox** and research lab. The primary goal is to explore SRE-focused AI agents and solve the "Observability for the Observer" challenge using OpenTelemetry.

## Key Conventions

### 1. Tool Implementation (`internal/tools/`)
- **Signature:** All tool methods MUST use: `func (t *Tool) Method(ctx context.Context, args map[string]any) (any, error)`.
- **HTTP Clients:** Use the `httpClient` wrapper from `internal/tools/http.go` which handles timeouts and logging.
- **Errors:** Return meaningful errors. The `toolResult` helper in `main.go` will format these for the MCP host.

### 2. OTel Instrumentation (Mandatory)
Every tool call MUST be instrumented.
- **Registration:** Wrap the handler in `main.go` with the `wrap` middleware:
  ```go
  func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
      result, err := wrap("tool_name", tool.Method)(ctx, req.Params.Arguments)
      return toolResult(result, err)
  }
  ```
- **Custom Attributes:** Use `sreotel.WithStringAttr(key, val)` to add metadata to spans in `main.go` or tool handlers.

### 3. Gemini CLI Integration
This server is designed to run as a sub-agent within the Gemini CLI.
- **Config Path:** `~/.gemini/settings.json`
- **Verification:** When testing new tools, use `npx @modelcontextprotocol/inspector ./mcp-server` to verify the JSON-RPC response before integrating with the CLI.

## Workflow: Adding a New Tool
1. **Define Struct:** Create `internal/tools/yourtool.go`.
2. **Implement Logic:** Add methods that query your target system.
3. **Add Config:** Update `internal/config/config.go` with any new environment variables.
4. **Register in `main.go`:** Initialize the tool and add it to `s.AddTool` with the `wrap` middleware.
5. **Update README:** Add the tool to the "Stack Integration" table.

## Critical Files
- `main.go`: Entry point & tool registration.
- `internal/otel/middleware.go`: The core instrumentation logic.
- `internal/otel/helpers.go`: OTel attribute helpers.
- `internal/tools/`: Domain logic for each observability signal.

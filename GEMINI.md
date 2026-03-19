# Autonomous SRE Agent - Developer Instructions

This document provides context and guidelines for Gemini to effectively assist in the development and expansion of the Event-Driven SRE Agent Sandbox.

## Project Nature
This is a **learning sandbox** and research lab. The primary goal is to explore autonomous, event-driven SRE AI agents that respond to Slack alerts and solve the "Observability for the Observer" challenge using OpenTelemetry.

## Architectural Context
The project consists of TWO distinct binaries that share the same internal packages:
1. **The Orchestrator (`cmd/orchestrator/main.go`):** The "Brain". An HTTP server that listens to Slack webhooks, formats prompts, and runs the LLM execution loop.
2. **The MCP Server (`cmd/mcp-server/main.go`):** The "Hands". A standard MCP server running over `stdio`. The Orchestrator spins this up as a subprocess to execute observability tools securely.

## Key Conventions

### 1. Adding/Modifying Tools (`internal/tools/`)
- **Signature:** All tool methods MUST use: `func (t *Tool) Method(ctx context.Context, args map[string]any) (any, error)`.
- **HTTP Clients:** Use the `httpClient` wrapper from `internal/tools/http.go` which handles timeouts and logging.
- **Registration:** New tools must be registered in `cmd/mcp-server/main.go` using the `s.AddTool` method and wrapped with the `wrap` middleware for OTel tracking.

### 2. OTel Instrumentation (Mandatory for Tools)
Every tool call MUST be instrumented.
- Wrap the handler in `cmd/mcp-server/main.go` with the `wrap` middleware.
- Use `sreotel.WithStringAttr(key, val)` to add metadata to spans.

### 3. Orchestrator Logic (`cmd/orchestrator/`)
- When modifying how the agent thinks, parses alerts, or handles threads, edit the LLM execution loop in `cmd/orchestrator/main.go`.
- The Orchestrator must ALWAYS instantiate the MCP server using `client.NewStdioMCPClient("./bin/mcp-server", []string{}, os.Environ()...)`.

## Critical Files
- `cmd/orchestrator/main.go`: Webhook listener, LLM loop, and MCP client logic.
- `cmd/mcp-server/main.go`: Tool registration and MCP server initialization.
- `internal/otel/middleware.go`: The core instrumentation logic for tool tracking.
- `internal/tools/`: Domain logic for querying metrics, logs, traces, and slack.
# Monitoring MCP Server (Go) - Gemini Instructions

This document provides context and guidelines for Gemini to effectively assist in the development, maintenance, and expansion of the Monitoring MCP Server.

## Project Overview

This is a **learning sandbox** project written in Go. Its primary purpose is to help the user learn about the Model Context Protocol (MCP) and explore how AI agents can interact with observability stacks (Metrics, Logs, Traces, Alerts, etc.) in a controlled environment.

**Note:** This is not intended for production use. It is a space for experimentation and prototyping SRE-focused AI tools.

### Core Stack
- **Language:** Go 1.22+
- **MCP Framework:** `github.com/mark3labs/mcp-go`
- **Observability Integration:**
  - **Metrics:** Prometheus / Thanos (PromQL)
  - **Logs:** Loki (LogQL)
  - **Traces:** Tempo (TraceQL)
  - **Alerts:** Alertmanager
  - **Notifications:** Slack Web API
  - **Dashboards:** Grafana API
- **Instrumentation:** OpenTelemetry (OTel) for Traces, Metrics, and Logs.

## Key Conventions & Standards

### 1. Tool Implementation (`internal/tools/`)
Each observability integration is implemented as a "Tool" in `internal/tools/`.
- **Structure:** Each tool should be a struct with an unexported HTTP client and a timeout.
- **Methods:** Tool methods must follow the signature: `func (t *Tool) MethodName(ctx context.Context, args map[string]any) (any, error)`.
- **Argument Parsing:** Use helper functions (if any) or manual type assertion for `args`. All inputs from MCP are `map[string]any`.
- **Error Handling:** Return meaningful errors. The `main.go` `toolResult` helper will handle converting these to MCP error responses.

### 2. Registration and Instrumentation (`main.go`)
- Tools are registered in `main.go` using `s.AddTool`.
- **Mandatory:** Every tool handler MUST be wrapped with the `wrap` middleware for OTel instrumentation:
  ```go
  func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
      result, err := wrap("tool_name", toolInstance.Method)(ctx, req.Params.Arguments)
      return toolResult(result, err)
  }
  ```
- Tool definitions (`mcp.NewTool`) should have clear, descriptive names and detailed descriptions for both the tool itself and its arguments.

### 3. Configuration (`internal/config/`)
- All configuration MUST be environment-variable driven.
- Use `internal/config/config.go` to manage settings.
- Avoid hardcoding values or URLs outside of defaults in the `Load` function.

### 4. Telemetry (`internal/otel/`)
- The server uses OTel for self-monitoring.
- `internal/otel/middleware.go` contains the `Wrap` function which handles:
  - Creating a span for the tool call.
  - Recording duration and error metrics.
  - Adding `tool_name` attributes to telemetry.

## Workflow: Adding a New Tool

1.  **Define Tool Logic:** Create a new file in `internal/tools/` (e.g., `k8s.go`). Implement the struct and the query methods.
2.  **Add to Config:** If the tool requires new environment variables (URLs, tokens), add them to `internal/config/config.go`.
3.  **Register in `main.go`:**
    - Initialize the tool instance.
    - Add to the `s.AddTool` list with appropriate schema and the `wrap` middleware.
4.  **Validate:** Ensure the new tool is included in the startup log list.

## Development & Testing

- **Local Run:** `go run main.go` (requires environment variables to be set, or a `.env` file if using a loader).
- **Tool Testing:** Since MCP tools communicate over stdio, unit tests in `internal/tools/` using mocked HTTP responses are the preferred way to verify logic.
- **Linting:** Follow standard Go linting practices (`golangci-lint`).

## Critical Files
- `main.go`: Entry point, tool registration, and MCP server setup.
- `internal/config/config.go`: Centralized configuration.
- `internal/otel/middleware.go`: Telemetry wrapper logic.
- `internal/tools/`: Implementation of all observability queries.

## Security
- **NEVER** log API tokens or secrets.
- Ensure all outbound HTTP requests respect the `QueryTimeout`.
- Be cautious with LogQL/PromQL injection if building queries dynamically from user input.

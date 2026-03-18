# Monitoring MCP Server (Sandbox)

A specialized learning sandbox for building and instrumenting an **SRE/Observability Assistant**. 

The goal of this project is twofold:
1. **Agent Capabilities:** Give AI agents (like Claude) "eyes" into an observability stack (Metrics, Logs, Traces) so they can assist with incident triage, latency investigation, and maintenance tasks.
2. **Instrumentation Research:** Explore how to properly monitor the MCP server itself. Since MCP servers typically communicate over **stdio**, they can't be "scraped" by Prometheus in the traditional way. This project demonstrates how to use OpenTelemetry to "push" signals out to a collector.

> **Research Reference:** This sandbox explores concepts similar to those discussed in [Monitoring MCP Servers with Prometheus and Grafana](https://medium.com/@vishaly650/monitoring-mcp-servers-with-prometheus-and-grafana-8671292e6351), specifically focusing on the challenges of observing short-lived or stdio-based tool calls.

---

## The "Observability for the Observer" Challenge

Monitoring an MCP server is tricky because:
- **Stdio Communication:** Traditional HTTP-based health checks and Prometheus scraping don't apply.
- **Tool-Level Granularity:** We need to know which specific tool (e.g., `query_metrics`) is failing or slow, not just that the process is "up."
- **Trace Context:** To truly debug an agent's reasoning, we need to correlate its tool calls with the actual signals it's retrieving.

This server implements **Full-Stack Self-Observation**:
- Every tool call produces a **Span** sent to Tempo.
- Execution metrics (duration, errors, count) are pushed to the OTel Collector.
- Structured logs are sent to Loki, tagged with the `tool_name`.

---

## Stack Integration

| Signal  | Backend                     | Tool(s)                                          |
|---------|-----------------------------|--------------------------------------------------|
| Metrics | Thanos / Prometheus (PromQL)| `query_metrics`, `query_metrics_range`           |
| Logs    | Loki (LogQL)                | `query_logs`, `query_logs_range`, `query_logs_by_trace_id` |
| Traces  | Tempo (TraceQL)             | `search_traces`, `get_trace`, `find_slow_traces` |
| Alerts  | Alertmanager                | `get_active_alerts`, `create_silence`            |
| Notify  | Slack Web API               | `post_slack_message`, `post_incident_summary`    |
| Grafana | Grafana HTTP API            | `search_dashboards`, `get_annotations`, `create_annotation` |

---

## Project Layout

```
monitoring-mcp-server/
├── main.go              # MCP server setup, tool registration
├── internal/
│   ├── config/          # Env-driven configuration
│   ├── otel/            # OTel provider & Middleware for stdio-based monitoring
│   └── tools/           # Implementation of observability queries
├── deployments/         # Docker, OTel Collector, and Grafana configs
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

---

## Prerequisites

- Go 1.22+
- An OTel Collector reachable over gRPC (default: `localhost:4317`)
- Access to your observability endpoints (Prometheus, Loki, Tempo, etc.)

---

## Build & run

```bash
# Local
go mod tidy
go build -o monitoring-mcp-server .
./monitoring-mcp-server

# Docker
docker build -t monitoring-mcp-server .
docker run --env-file .env monitoring-mcp-server

# Docker Compose
docker compose up -d
```

---

## Configuration

All configuration is via environment variables.

| Variable | Required | Default | Description |
|---|---|---|---|
| `METRICS_URL` | Yes | `http://localhost:9090` | Prometheus/Thanos endpoint |
| `LOKI_URL` | No | `http://localhost:3100` | Loki endpoint |
| `TEMPO_URL` | No | `http://localhost:3200` | Tempo endpoint |
| `ALERTMANAGER_URL` | No | `http://localhost:9093` | Alertmanager endpoint |
| `GRAFANA_URL` | No | `http://localhost:3000` | Grafana endpoint |
| `GRAFANA_API_TOKEN` | No | — | Grafana service account token |
| `SLACK_BOT_TOKEN` | No | — | Slack bot OAuth token (`xoxb-…`) |
| `SLACK_DEFAULT_CHANNEL` | No | — | Default Slack channel |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | No | — | OTel Collector gRPC endpoint (e.g. `localhost:4317`) |
| `OTEL_SERVICE_NAME` | No | `monitoring-mcp` | Service name in traces/metrics |
| `QUERY_TIMEOUT` | No | `10s` | Duration timeout for queries (e.g. `30s`, `1m`) |
| `LOG_LEVEL` | No | `info` | `debug` or `info` |

---

## Wire into Claude Desktop

1. Build the binary: `go build -o monitoring-mcp-server .`
2. Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) and merge the content from `claude-mcp-config.json`, updating the `command` path to the absolute path of your binary and your real endpoint URLs.
3. Restart Claude Desktop.

---

## Example agent prompts

Once wired into Claude, try these:

**Incident triage:**
```
There's a P1 alert firing for payments-service. Check active alerts,
then query the error rate with PromQL for the last 30 minutes, find
the traces with status 500, pull logs for the worst trace ID, and
post a summary to #incidents.
```

**Latency investigation:**
```
The checkout service has been slow since 14:00 UTC. Find traces slower
than 2s, get the full span tree for the worst one, then check if there
are any deployment annotations in Grafana around that time.
```

---

## Extending with new tools

1. Add a new file in `internal/tools/` implementing a struct with methods matching `func(ctx context.Context, args map[string]any) (any, error)`.
2. Register the tool in `main.go` using `s.AddTool(mcp.NewTool(...), handler)`.
3. Wrap the handler with `wrap("tool_name", yourTool.Method)` — OTel instrumentation is automatic.

---

## Security notes

- The Grafana API token and Slack bot token are **never logged**.
- The server communicates with Claude over **stdio** — no open ports required for MCP.

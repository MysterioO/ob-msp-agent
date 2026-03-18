# SRE MCP Server (Go)

A production-grade MCP server written in Go that gives AI agents (Claude or any MCP-compatible host) direct, instrumented access to your observability stack.

## Stack integration

| Signal  | Backend                     | Tool(s)                                          |
|---------|-----------------------------|--------------------------------------------------|
| Metrics | Thanos / Prometheus (PromQL)| `query_metrics`, `query_metrics_range`           |
| Logs    | Loki (LogQL)                | `query_logs`, `query_logs_range`, `query_logs_by_trace_id` |
| Traces  | Tempo (TraceQL)             | `search_traces`, `get_trace`, `find_slow_traces` |
| Alerts  | Alertmanager                | `get_active_alerts`, `create_silence`            |
| Notify  | Slack Web API               | `post_slack_message`, `post_incident_summary`    |
| Grafana | Grafana HTTP API            | `search_dashboards`, `get_annotations`, `create_annotation` |

All tool calls are instrumented with OpenTelemetry — every invocation produces a **span** (→ Tempo), **metrics** (→ Prometheus/Thanos via OTel Collector), and structured **logs** (→ Loki). The server monitors itself through your existing stack with zero extra infrastructure.

---

## Project layout

```
sre-mcp-server/
├── cmd/server/
│   ├── main.go          # MCP server setup, tool registration
│   └── helpers.go       # JSON / OTel convenience helpers
├── internal/
│   ├── config/
│   │   └── config.go    # Env-driven config; no hardcoded values
│   ├── otel/
│   │   ├── provider.go  # TracerProvider + MeterProvider over gRPC
│   │   ├── middleware.go # Wrap() — OTel instrumentation for every tool call
│   │   └── helpers.go   # SpanStartOption helpers
│   └── tools/
│       ├── metrics.go   # PromQL instant + range queries
│       ├── logs.go      # LogQL instant + range + trace-ID correlation
│       ├── traces.go    # TraceQL search, get, slow-trace finder
│       ├── alerts.go    # Alertmanager alerts + silences
│       ├── slack.go     # Slack message + incident summary
│       ├── grafana.go   # Dashboard search, annotations
│       ├── http.go      # Shared HTTP client
│       └── util.go      # requireString, postJSON helpers
├── deployments/
│   ├── docker-compose.yml          # Container deployment
│   ├── otel-collector-config.yaml  # Merge into your existing collector config
│   ├── grafana-dashboard.json      # Import into Grafana
│   ├── prometheus-rules.yaml       # Alerting rules for the server itself
│   └── claude-mcp-config.json      # Claude Desktop wiring
├── Dockerfile
└── go.mod
```

---

## Prerequisites

- Go 1.22+
- An OTel Collector reachable over gRPC (default: `localhost:4317`)
- At minimum one of: `THANOS_QUERY_URL` or `PROMETHEUS_URL`

---

## Build & run

```bash
# Local
go mod tidy
go build -o sre-mcp-server ./cmd/server
./sre-mcp-server

# Docker
docker build -t sre-mcp-server .
docker run --env-file .env sre-mcp-server

# Docker Compose (adjusts URLs in deployments/docker-compose.yml first)
cd deployments && docker compose up -d
```

---

## Configuration

All configuration is via environment variables. Copy `.env.example` to `.env`:

| Variable | Required | Default | Description |
|---|---|---|---|
| `THANOS_QUERY_URL` | one of two | — | Thanos Query endpoint (preferred) |
| `PROMETHEUS_URL` | one of two | `http://localhost:9090` | Prometheus fallback |
| `LOKI_URL` | no | `http://localhost:3100` | Loki endpoint |
| `TEMPO_URL` | no | `http://localhost:3200` | Tempo endpoint |
| `ALERTMANAGER_URL` | no | `http://localhost:9093` | Alertmanager endpoint |
| `GRAFANA_URL` | no | `http://localhost:3000` | Grafana endpoint |
| `GRAFANA_API_TOKEN` | no | — | Grafana service account token |
| `SLACK_BOT_TOKEN` | no | — | Slack bot OAuth token (`xoxb-…`) |
| `SLACK_DEFAULT_CHANNEL` | no | `#sre-agent` | Default Slack channel |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | `localhost:4317` | OTel Collector gRPC endpoint |
| `OTEL_SERVICE_NAME` | no | `sre-mcp-server` | Service name in traces/metrics |
| `QUERY_TIMEOUT_SECONDS` | no | `30` | HTTP client timeout per tool call |
| `LOG_LEVEL` | no | `info` | `debug` or `info` |

---

## Wire into Claude Desktop

1. Build the binary: `go build -o sre-mcp-server ./cmd/server`
2. Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) and merge the content from `deployments/claude-mcp-config.json`, updating the `command` path and your real endpoint URLs.
3. Restart Claude Desktop.

---

## Grafana setup

1. **Import dashboard**: Grafana → Dashboards → Import → upload `deployments/grafana-dashboard.json`
2. **Alerting rules**: Add `deployments/prometheus-rules.yaml` to your Prometheus `rule_files` and reload.
3. **OTel Collector**: Merge `deployments/otel-collector-config.yaml` into your existing collector config — it adds the Loki log exporter and tags MCP-server signals with `tool_name` labels.

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

**Maintenance prep:**
```
I'm deploying a breaking schema migration to the users-service at 22:00.
Create a 2-hour silence for UserServiceErrorRate alerts and post a
heads-up to #sre-oncall.
```

**Cross-signal correlation:**
```
Here's a trace ID from a customer complaint: abc123def456.
Pull the logs for that trace, find the slow span, and check if the
relevant service had any metric anomalies in the surrounding 10 minutes.
```

---

## Extending with new tools

1. Add a new file in `internal/tools/` implementing a struct with methods matching `func(ctx context.Context, args map[string]any) (any, error)`.
2. Register the tool in `cmd/server/main.go` using `s.AddTool(mcp.NewTool(...), handler)`.
3. Wrap the handler with `wrap("tool_name", yourTool.Method)` — OTel instrumentation is automatic.

---

## Security notes

- The Grafana API token and Slack bot token are **never logged** — they only appear in the `Authorization` header of outbound requests.
- The binary runs as UID 65534 (nobody) in the Docker image and has `--read-only` filesystem.
- All Linux capabilities are dropped in the Docker Compose config.
- The server communicates with Claude over **stdio** — no open ports.

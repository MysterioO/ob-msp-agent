# Monitoring MCP Server (Sandbox)

A specialized learning sandbox for building an **SRE/Observability Assistant** and researching the unique challenges of instrumenting MCP servers.

## The Mission
1. **Agent Capabilities:** Give AI agents (like Gemini, Claude, or Cursor) "eyes" into an observability stack (Metrics, Logs, Traces) so they can assist with incident triage, latency investigation, and maintenance.
2. **Instrumentation Research:** Solve the "Observability for the Observer" problem. Since MCP servers communicate over **stdio**, traditional scraping (like Prometheus) doesn't work. This project demonstrates how to "push" signals (Spans, Metrics, Logs) out to an OTel Collector from a stdio-based process.

---

## The Challenge: Monitoring the Monitor

Monitoring an MCP server is tricky because:
- **Stdio is a Black Box:** You can't use standard HTTP health checks or `/metrics` endpoints.
- **Tool-Level Granularity:** You need to know which specific tool (e.g., `query_logs`) is slow or failing, not just that the process is running.
- **Trace Correlation:** Debugging an agent requires seeing the exact signals (logs/metrics) it retrieved during a specific reasoning step.

**This sandbox implements Full-Stack Self-Observation:**
- **Traces:** Every tool call creates a span in Tempo.
- **Metrics:** Duration, errors, and counts are pushed to the OTel Collector.
- **Logs:** Structured logs are sent to Loki, tagged with the `tool_name`.

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

## Development & Testing

### 1. Build & Run Locally
```bash
# Build the binary
go build -o mcp-server .

# Run the unit tests (Mocked HTTP calls)
go test -v ./internal/tools/...
```

### 2. Manual Verification (MCP Inspector)
The MCP Inspector provides a web UI to manually trigger tools and see their JSON-RPC request/response.
```bash
npx @modelcontextprotocol/inspector ./mcp-server
```

---

## Deployment & Docker

### Local Observability Stack
The provided `docker-compose.yaml` (coming soon/referenced) can be used to spin up a local development stack:
```bash
# Start the OTel Collector, Prometheus, Loki, Tempo, and Grafana
docker compose up -d
```

### Containerizing the Server
The `Dockerfile` provides a multi-stage build resulting in a minimal `scratch`-based image:
```bash
# Build the production image
docker build -t sre-mcp-server .
```

---

## Integration

### Generic Configuration Template
A template for your agent configuration is provided in `mcp-config.json`. Use this to configure Gemini CLI, Claude Desktop, or Cursor.

### Gemini CLI
Add this to your `~/.gemini/settings.json`:
```json
{
  "mcp_servers": {
    "monitoring-lab": {
      "command": "/absolute/path/to/mcp-server",
      "env": {
        "METRICS_URL": "http://localhost:9090",
        "LOKI_URL": "http://localhost:3100",
        "TEMPO_URL": "http://localhost:3200",
        "ALERTMANAGER_URL": "http://localhost:9093",
        "GRAFANA_URL": "http://localhost:3000"
      }
    }
  }
}
```

---

## Configuration

| Variable | Default | Description |
|---|---|---|
| `METRICS_URL` | `http://localhost:9090` | Prometheus/Thanos endpoint |
| `LOKI_URL` | `http://localhost:3100` | Loki endpoint |
| `TEMPO_URL` | `http://localhost:3200` | Tempo endpoint |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTel Collector gRPC endpoint |
| `QUERY_TIMEOUT` | `10s` | Global timeout for all observability queries |
| `LOG_LEVEL` | `info` | `debug` or `info` |
| `SLACK_BOT_TOKEN` | - | Required for Slack notifications |
| `GRAFANA_API_TOKEN` | - | Required for Grafana annotations |

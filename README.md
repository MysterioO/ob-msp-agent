# Monitoring MCP Server (Sandbox)

A specialized learning sandbox for building an **SRE/Observability Assistant** and researching the unique challenges of instrumenting MCP servers.

## The Mission
1. **Agent Capabilities:** Give AI agents (like Gemini or Claude) "eyes" into an observability stack (Metrics, Logs, Traces) so they can assist with incident triage, latency investigation, and maintenance.
2. **Instrumentation Research:** Solve the "Observability for the Observer" problem. Since MCP servers communicate over **stdio**, traditional scraping (like Prometheus) doesn't work. This project demonstrates how to "push" signals (Spans, Metrics, Logs) out to an OTel Collector from a stdio-based process.

> **Research Reference:** Inspired by [Monitoring MCP Servers with Prometheus and Grafana](https://medium.com/@vishaly650/monitoring-mcp-servers-with-prometheus-and-grafana-8671292e6351).

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

## Build & Run

```bash
# 1. Build the binary
go build -o mcp-server .

# 2. Run local observability stack (Collector, Prometheus, Loki, Tempo, Grafana)
docker compose up -d

# 3. Test with the MCP Inspector (Web UI)
npx @modelcontextprotocol/inspector ./mcp-server
```

---

## Integration

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

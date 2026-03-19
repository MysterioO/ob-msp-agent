**Work in Progress**
# Autonomous SRE Agent (Monitoring MCP Sandbox)

A specialized learning sandbox for building an **Event-Driven SRE Assistant** and researching the unique challenges of instrumenting Model Context Protocol (MCP) servers.

## The Mission
1. **The Autopilot (Event-Driven):** Go beyond manual chat. This project listens to firing alerts (via Slack webhooks), automatically spins up an AI agent to investigate across your observability stack, and posts a root-cause hypothesis as a threaded reply to the original alert.
2. **The Copilot (Manual):** Give AI agents (like Gemini, Claude, or Cursor) "eyes" into an observability stack (Metrics, Logs, Traces) so engineers can manually prompt them during incident triage.
3. **Instrumentation Research:** Solve the "Observability for the Observer" problem. This project demonstrates how to "push" signals (Spans, Metrics, Logs) out to an OTel Collector from a stdio-based process.

---
## The Challenge: Monitoring the Monitor

Monitoring an AI Agent and its underlying MCP server is tricky because:
- **Stdio is a Black Box:** You can't use standard HTTP health checks or `/metrics` endpoints.
- **Tool-Level Granularity:** You need to know which specific tool (e.g., `query_logs`) is slow or failing, not just that the process is running.
- **Trace Correlation:** Debugging an agent requires seeing the exact signals (logs/metrics) it retrieved during a specific reasoning step.

**This sandbox implements Full-Stack Self-Observation:**
- **Traces:** Every tool call creates a span in Tempo.
- **Metrics:** Duration, errors, and counts are pushed to the OTel Collector.
- **Logs:** Structured logs are sent to Loki, tagged with the `mcp.tool.name`.
---

## Architecture: The Two Brains

This repository builds **two** distinct binaries that work together:

1. **The Orchestrator (`cmd/orchestrator`)**
   A lightweight HTTP server that listens for Slack events (e.g., `#grafana-alerts`). When an alert fires, it extracts the context, formats an investigation prompt, and executes the LLM loop.
2. **The MCP Server (`cmd/mcp-server`)**
   The tool-provider. It communicates over `stdio` and acts as a bridge to your observability backends. The Orchestrator spawns this server as a subprocess to give the AI agent secure access to the infrastructure.

---

## The User Journey: AI-Assisted Incident Triage

When a 500-error spike occurs and triggers a Grafana alert to Slack, here is what happens automatically:

1. **The Trigger:** Grafana posts `HighLatencyCritical for checkout-api` to `#grafana-alerts`.
2. **The Orchestrator:** Intercepts the Slack webhook, extracts the `thread_ts`, and wakes up the AI Agent.
3. **Scoping the Impact (Thanos/Prometheus):** The AI uses `query_metrics_range` to pull PromQL data and summarizes the latency trend.
4. **Finding the Bottleneck (Tempo):** The AI uses `find_slow_traces` (TraceQL) to find traces exceeding 2s, grabbing the slowest trace ID.
5. **Pinpointing the Root Cause (Loki):** The AI uses `query_logs_by_trace_id` to correlate the trace ID with Loki logs, discovering a `connection pool exhausted` error.
6. **Communication (Grafana & Slack):** The AI drops a marker on Grafana using `create_annotation` and posts a color-coded incident summary *directly into the Slack thread* of the original alert.

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

### 1. Build Both Binaries Locally
```bash
# Build the MCP Server (The Tools)
go build -o bin/mcp-server ./cmd/mcp-server

# Build the Orchestrator (The Listener)
go build -o bin/orchestrator ./cmd/orchestrator
```
### 2. Run the Orchestrator

Ensure your .env variables are set, then run the Orchestrator. It will listen on port 8080 and spawn the MCP server automatically when needed.

```bash
./bin/orchestrator
```
(Tip: Use ngrok http 8080 to test Slack webhooks locally!)  

### 3. Deploy to Kubernetes

This project is designed for production deployment on Kubernetes (e.g., GKE) using a unified multi-stage Docker image containing both binaries.

```bash
# Build the production image
docker build -t us-central1-docker.pkg.dev/YOUR_PROJECT/YOUR_REPO/sre-agent:latest .
``` 
The k8s/ directory (if applicable) contains manifests to deploy the Orchestrator securely using Workload Identity, GCP Secret Manager CSI, and Managed SSL Certificates for secure Slack webhook ingestion.


## Configuration

| Variable | Default | Description |
|---|---|---|
| `AI_API_KEY` | - | Required for AI API |
| `METRICS_URL` | `http://localhost:9090` | Prometheus/Thanos endpoint |
| `LOKI_URL` | `http://localhost:3100` | Loki endpoint |
| `TEMPO_URL` | `http://localhost:3200` | Tempo endpoint |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTel Collector gRPC endpoint |
| `QUERY_TIMEOUT` | `10s` | Global timeout for all observability queries |
| `LOG_LEVEL` | `info` | `debug` or `info` |
| `SLACK_BOT_TOKEN` | - | Required for Slack notifications |
| `GRAFANA_API_TOKEN` | - | Required for Grafana annotations |

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"github.com/MysterioO/ob-msp-agent/internal/config"
	sreotel "github.com/MysterioO/ob-msp-agent/internal/otel"
	"github.com/MysterioO/ob-msp-agent/internal/tools"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// --- Config ---
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// --- Logger ---
	var logger *zap.Logger
	if cfg.LogLevel == "debug" {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}
	defer logger.Sync() //nolint:errcheck

	logger.Info("starting sre-mcp-server",
		zap.String("version", cfg.ServerVersion),
		zap.String("otel_endpoint", cfg.OTelEndpoint),
		zap.String("metrics_url", cfg.MetricsQueryURL()),
	)

	// --- OTel ---
	otelProvider, err := sreotel.NewProvider(ctx, cfg.OTelEndpoint, cfg.OTelServiceName, cfg.ServerVersion)
	if err != nil {
		// Non-fatal: log and continue without telemetry rather than refusing to start.
		logger.Warn("otel provider init failed — continuing without telemetry", zap.Error(err))
	}
	if otelProvider != nil {
		defer otelProvider.Shutdown(context.Background())
	}

	var toolMetrics *sreotel.ToolMetrics
	if otelProvider != nil {
		toolMetrics, err = sreotel.NewToolMetrics(otelProvider.Meter)
		if err != nil {
			return fmt.Errorf("tool metrics: %w", err)
		}
	}

	// --- Tool instances ---
	metricsTool := tools.NewMetricsTool(cfg.MetricsQueryURL(), cfg.QueryTimeout)
	logsTool := tools.NewLogsTool(cfg.LokiURL, cfg.QueryTimeout)
	tracesTool := tools.NewTracesTool(cfg.TempoURL, cfg.QueryTimeout)
	alertsTool := tools.NewAlertsTool(cfg.AlertmanagerURL, cfg.QueryTimeout)
	slackTool := tools.NewSlackTool(cfg.SlackBotToken, cfg.SlackDefaultChn, cfg.QueryTimeout)
	grafanaTool := tools.NewGrafanaTool(cfg.GrafanaURL, cfg.GrafanaAPIToken, cfg.QueryTimeout)

	// wrap wraps a tool handler with OTel instrumentation.
	// If OTel is unavailable it returns the handler as-is.
	wrap := func(name string, h sreotel.ToolHandler) sreotel.ToolHandler {
		if otelProvider == nil || toolMetrics == nil {
			return h
		}
		return sreotel.Wrap(otelProvider.Tracer, toolMetrics, name, h)
	}

	// --- MCP Server ---
	s := server.NewMCPServer(cfg.ServerName, cfg.ServerVersion,
		server.WithToolCapabilities(true),
	)

	// ------------------------------------------------------------------ //
	// Tool registrations
	// Each tool gets:
	//   1. An MCP tool definition (name, description, input schema)
	//   2. A handler wrapped with OTel middleware
	// ------------------------------------------------------------------ //

	// query_metrics — instant PromQL
	s.AddTool(
		mcp.NewTool("query_metrics",
			mcp.WithDescription("Execute an instant PromQL query against Thanos/Prometheus. "+
				"Use for current values of metrics, SLO burn rates, error ratios, and resource saturation."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("PromQL expression e.g. `rate(http_requests_total{status=~'5..'}[5m])`"),
			),
			mcp.WithString("time",
				mcp.Description("Evaluation timestamp in RFC3339 or Unix seconds. Defaults to now."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("query_metrics", metricsTool.QueryMetrics)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// query_metrics_range — PromQL range
	s.AddTool(
		mcp.NewTool("query_metrics_range",
			mcp.WithDescription("Execute a PromQL range query for trend analysis and incident timelines. "+
				"Use when you need to see how a metric evolved over a time window."),
			mcp.WithString("query", mcp.Required(), mcp.Description("PromQL expression")),
			mcp.WithString("start", mcp.Required(), mcp.Description("Start time RFC3339 or Unix seconds")),
			mcp.WithString("end", mcp.Required(), mcp.Description("End time RFC3339 or Unix seconds")),
			mcp.WithString("step", mcp.Required(), mcp.Description("Step interval e.g. '1m', '5m', '1h'")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("query_metrics_range", metricsTool.QueryMetricsRange)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// query_logs — instant LogQL
	s.AddTool(
		mcp.NewTool("query_logs",
			mcp.WithDescription("Execute a LogQL instant query against Loki. "+
				"Use for sampling recent log lines, checking error messages, or counting log events."),
			mcp.WithString("query", mcp.Required(), mcp.Description(`LogQL expression e.g. {app="api"} |= "error" | json`)),
			mcp.WithString("time", mcp.Description("Evaluation timestamp")),
			mcp.WithNumber("limit", mcp.Description("Max log lines to return. Defaults to 100.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("query_logs", logsTool.QueryLogs)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// query_logs_range — LogQL range
	s.AddTool(
		mcp.NewTool("query_logs_range",
			mcp.WithDescription("Execute a LogQL range query to pull log lines across an incident window."),
			mcp.WithString("query", mcp.Required(), mcp.Description("LogQL expression")),
			mcp.WithString("start", mcp.Required(), mcp.Description("Start time RFC3339 or Unix nanoseconds")),
			mcp.WithString("end", mcp.Required(), mcp.Description("End time RFC3339 or Unix nanoseconds")),
			mcp.WithNumber("limit", mcp.Description("Max log lines. Defaults to 200.")),
			mcp.WithString("direction", mcp.Description("'forward' or 'backward'. Defaults to 'backward'.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("query_logs_range", logsTool.QueryLogsRange)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// query_logs_by_trace_id — cross-signal correlation
	s.AddTool(
		mcp.NewTool("query_logs_by_trace_id",
			mcp.WithDescription("Fetch all log lines containing a specific trace ID from Loki. "+
				"This is the primary cross-signal correlation tool — give it a trace ID from Tempo "+
				"and get back the structured logs for that request."),
			mcp.WithString("trace_id", mcp.Required(), mcp.Description("W3C or Jaeger hex trace ID")),
			mcp.WithString("namespace", mcp.Description("Kubernetes namespace to scope the Loki query")),
			mcp.WithString("start", mcp.Description("Start time RFC3339. Defaults to 1h ago.")),
			mcp.WithString("end", mcp.Description("End time RFC3339. Defaults to now.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("query_logs_by_trace_id", logsTool.QueryLogsByTraceID)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// search_traces — TraceQL search
	s.AddTool(
		mcp.NewTool("search_traces",
			mcp.WithDescription("Search for distributed traces in Tempo using TraceQL. "+
				"Use to find error traces, high-latency requests, or traces for a specific service."),
			mcp.WithString("query", mcp.Required(), mcp.Description(`TraceQL expression e.g. {.http.status_code=500 && resource.service.name="payments"}`)),
			mcp.WithString("start", mcp.Description("Start Unix timestamp seconds. Defaults to 1h ago.")),
			mcp.WithString("end", mcp.Description("End Unix timestamp seconds. Defaults to now.")),
			mcp.WithNumber("limit", mcp.Description("Max traces to return. Defaults to 20.")),
			mcp.WithString("min_dur", mcp.Description("Minimum trace duration filter e.g. '500ms', '2s'.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("search_traces", tracesTool.SearchTraces)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// get_trace — fetch full trace
	s.AddTool(
		mcp.NewTool("get_trace",
			mcp.WithDescription("Fetch the full span tree for a single trace ID from Tempo. "+
				"Use after finding a trace ID via search_traces to inspect the span waterfall."),
			mcp.WithString("trace_id", mcp.Required(), mcp.Description("Hex trace ID")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("get_trace", tracesTool.GetTrace)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// find_slow_traces — latency investigation shortcut
	s.AddTool(
		mcp.NewTool("find_slow_traces",
			mcp.WithDescription("Find traces for a service that exceed a minimum duration. "+
				"Use as first step in latency investigations."),
			mcp.WithString("service", mcp.Required(), mcp.Description("Service name as reported in resource.service.name")),
			mcp.WithString("min_dur", mcp.Required(), mcp.Description("Minimum duration e.g. '1s', '500ms'")),
			mcp.WithString("start", mcp.Description("Start Unix seconds")),
			mcp.WithString("end", mcp.Description("End Unix seconds")),
			mcp.WithNumber("limit", mcp.Description("Max results")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("find_slow_traces", tracesTool.FindSlowTraces)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// get_active_alerts — Alertmanager
	s.AddTool(
		mcp.NewTool("get_active_alerts",
			mcp.WithDescription("Fetch currently firing alerts from Alertmanager. "+
				"Use at the start of any incident investigation to get full situational awareness."),
			mcp.WithString("filter", mcp.Description("Alertmanager label matcher e.g. 'severity=critical'. Can be repeated.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("get_active_alerts", alertsTool.GetActiveAlerts)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// create_silence — Alertmanager silence
	s.AddTool(
		mcp.NewTool("create_silence",
			mcp.WithDescription("Create an Alertmanager silence to suppress known noisy alerts "+
				"during an active incident or maintenance window."),
			mcp.WithString("duration", mcp.Required(), mcp.Description("Silence duration e.g. '2h', '30m'")),
			mcp.WithString("comment", mcp.Required(), mcp.Description("Reason for the silence — required for audit trail")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("create_silence", alertsTool.CreateSilence)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// post_slack_message
	s.AddTool(
		mcp.NewTool("post_slack_message",
			mcp.WithDescription("Post a message to a Slack channel. "+
				"Use to surface investigation summaries, status updates, or action items."),
			mcp.WithString("text", mcp.Required(), mcp.Description("Message text in Slack mrkdwn format")),
			mcp.WithString("channel", mcp.Description("Slack channel ID or name. Defaults to configured default.")),
			mcp.WithString("title", mcp.Description("Optional bold title shown above the message")),
			mcp.WithString("color", mcp.Description("Attachment colour: 'good', 'warning', 'danger', or hex")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("post_slack_message", slackTool.PostMessage)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// post_incident_summary
	s.AddTool(
		mcp.NewTool("post_incident_summary",
			mcp.WithDescription("Post a structured incident investigation summary to Slack with "+
				"severity colour coding and links to traces and runbooks."),
			mcp.WithString("severity", mcp.Required(), mcp.Description("'critical', 'warning', or 'info'")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Short incident title")),
			mcp.WithString("summary", mcp.Required(), mcp.Description("Investigation findings in mrkdwn")),
			mcp.WithString("channel", mcp.Description("Override default Slack channel")),
			mcp.WithString("trace_url", mcp.Description("Link to a representative trace in Grafana/Tempo")),
			mcp.WithString("runbook", mcp.Description("Runbook URL")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("post_incident_summary", slackTool.PostIncidentSummary)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// search_dashboards — Grafana
	s.AddTool(
		mcp.NewTool("search_dashboards",
			mcp.WithDescription("Search Grafana dashboards by title or tag."),
			mcp.WithString("query", mcp.Description("Freetext search against dashboard title")),
			mcp.WithString("tag", mcp.Description("Filter by Grafana dashboard tag")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("search_dashboards", grafanaTool.SearchDashboards)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// get_annotations — Grafana
	s.AddTool(
		mcp.NewTool("get_annotations",
			mcp.WithDescription("Fetch Grafana annotations (deployments, incidents, maintenance) for a time window. "+
				"Use to correlate anomalies with recent changes."),
			mcp.WithString("from", mcp.Description("Start time Unix ms. Defaults to 1h ago.")),
			mcp.WithString("to", mcp.Description("End time Unix ms. Defaults to now.")),
			mcp.WithString("tags", mcp.Description("Filter by tag e.g. 'deployment'")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("get_annotations", grafanaTool.GetAnnotations)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	// create_annotation — Grafana
	s.AddTool(
		mcp.NewTool("create_annotation",
			mcp.WithDescription("Create a Grafana annotation to mark an investigation or remediation action on dashboards."),
			mcp.WithString("text", mcp.Required(), mcp.Description("Annotation description")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := wrap("create_annotation", grafanaTool.CreateAnnotation)(ctx, req.Params.Arguments)
			return toolResult(result, err)
		},
	)

	logger.Info("registered MCP tools",
		zap.Int("count", 15),
		zap.Strings("tools", []string{
			"query_metrics", "query_metrics_range",
			"query_logs", "query_logs_range", "query_logs_by_trace_id",
			"search_traces", "get_trace", "find_slow_traces",
			"get_active_alerts", "create_silence",
			"post_slack_message", "post_incident_summary",
			"search_dashboards", "get_annotations", "create_annotation",
		}),
	)

	// Emit a startup span so Tempo shows the server coming online.
	if otelProvider != nil {
		_, span := otelProvider.Tracer.Start(ctx, "server.startup",
			sreotel.WithStringAttr("server.name", cfg.ServerName),
			sreotel.WithStringAttr("server.version", cfg.ServerVersion),
		)
		span.End()
	}

	logger.Info("serving MCP over stdio — ready for connections")
	return server.ServeStdio(s)
}

// toolResult converts a tool's (any, error) return into a *mcp.CallToolResult.
func toolResult(result any, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	switch v := result.(type) {
	case string:
		return mcp.NewToolResultText(v), nil
	default:
		b, jerr := json.MarshalIndent(v, "", "  ")
		if jerr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", jerr)), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	}
}

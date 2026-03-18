package tools

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// LogsTool executes LogQL queries against Loki.
// The agent uses this to pull structured log lines correlated to incidents,
// trace IDs, or specific service label selectors.
type LogsTool struct {
	lokiURL string
	hc      *httpClient
}

func NewLogsTool(lokiURL string, timeout time.Duration) *LogsTool {
	return &LogsTool{lokiURL: lokiURL, hc: newHTTPClient(timeout)}
}

type lokiQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     any    `json:"result"`
	} `json:"data"`
}

// QueryLogs executes a LogQL instant query.
// args:
//
//	query string  (required) — LogQL expression, e.g. `{app="api"} |= "error"`
//	time  string  (optional) — RFC3339 or Unix ns; defaults to now
//	limit int     (optional) — max log lines returned; defaults to 100
func (t *LogsTool) QueryLogs(ctx context.Context, args map[string]any) (any, error) {
	query, err := requireString(args, "query")
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("query", query)

	if ts, ok := args["time"].(string); ok && ts != "" {
		params.Set("time", ts)
	}

	limit := 100
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	params.Set("limit", fmt.Sprintf("%d", limit))

	endpoint := fmt.Sprintf("%s/loki/api/v1/query?%s", t.lokiURL, params.Encode())

	var resp lokiQueryResponse
	if err := t.hc.get(ctx, endpoint, nil, &resp); err != nil {
		return nil, fmt.Errorf("query_logs: %w", err)
	}

	return map[string]any{
		"resultType": resp.Data.ResultType,
		"result":     resp.Data.Result,
	}, nil
}

// QueryLogsRange executes a LogQL range query — useful for pulling logs
// across the time window of an incident.
// args:
//
//	query     string  (required) — LogQL expression
//	start     string  (required) — RFC3339 or Unix ns
//	end       string  (required) — RFC3339 or Unix ns
//	limit     int     (optional) — max log lines; defaults to 200
//	direction string  (optional) — "forward" or "backward"; defaults to "backward"
func (t *LogsTool) QueryLogsRange(ctx context.Context, args map[string]any) (any, error) {
	query, err := requireString(args, "query")
	if err != nil {
		return nil, err
	}
	start, err := requireString(args, "start")
	if err != nil {
		return nil, err
	}
	end, err := requireString(args, "end")
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("start", start)
	params.Set("end", end)

	limit := 200
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	params.Set("limit", fmt.Sprintf("%d", limit))

	direction := "backward"
	if d, ok := args["direction"].(string); ok && d != "" {
		direction = d
	}
	params.Set("direction", direction)

	endpoint := fmt.Sprintf("%s/loki/api/v1/query_range?%s", t.lokiURL, params.Encode())

	var resp lokiQueryResponse
	if err := t.hc.get(ctx, endpoint, nil, &resp); err != nil {
		return nil, fmt.Errorf("query_logs_range: %w", err)
	}

	return map[string]any{
		"resultType": resp.Data.ResultType,
		"result":     resp.Data.Result,
	}, nil
}

// QueryLogsByTraceID pulls all log lines that contain a specific trace ID.
// This is the key cross-signal correlation tool — give it a trace ID from Tempo
// and it returns the associated log lines from Loki.
// args:
//
//	trace_id  string  (required) — W3C or Jaeger format trace ID
//	namespace string  (optional) — Kubernetes namespace label to scope the search
//	start     string  (optional) — RFC3339; defaults to 1h ago
//	end       string  (optional) — RFC3339; defaults to now
func (t *LogsTool) QueryLogsByTraceID(ctx context.Context, args map[string]any) (any, error) {
	traceID, err := requireString(args, "trace_id")
	if err != nil {
		return nil, err
	}

	// Build a LogQL query that searches for the trace ID across all streams.
	// Adjust the label selector to match your Loki label scheme.
	logqlQuery := fmt.Sprintf(`{job=~".+"} |= "%s"`, traceID)
	if ns, ok := args["namespace"].(string); ok && ns != "" {
		logqlQuery = fmt.Sprintf(`{namespace="%s"} |= "%s"`, ns, traceID)
	}

	// Default time range: last 1 hour.
	start := fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).UnixNano())
	end := fmt.Sprintf("%d", time.Now().UnixNano())

	if s, ok := args["start"].(string); ok && s != "" {
		start = s
	}
	if e, ok := args["end"].(string); ok && e != "" {
		end = e
	}

	return t.QueryLogsRange(ctx, map[string]any{
		"query": logqlQuery,
		"start": start,
		"end":   end,
		"limit": float64(500),
	})
}

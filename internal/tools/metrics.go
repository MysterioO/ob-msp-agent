package tools

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// MetricsTool executes PromQL queries against Thanos (preferred) or Prometheus.
// It supports both instant and range queries to give the agent flexibility
// when investigating spikes vs trends.
type MetricsTool struct {
	baseURL string
	hc      *httpClient
}

func NewMetricsTool(metricsQueryURL string, timeout time.Duration) *MetricsTool {
	return &MetricsTool{
		baseURL: metricsQueryURL,
		hc:      newHTTPClient(timeout),
	}
}

// prometheusResponse is the envelope returned by the Prometheus HTTP API v1.
type prometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     any    `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType,omitempty"`
	Error     string `json:"error,omitempty"`
}

// QueryMetrics executes an instant PromQL query.
// args:
//
//	query   string  (required) — PromQL expression
//	time    string  (optional) — RFC3339 or Unix timestamp; defaults to now
//	timeout string  (optional) — e.g. "10s"; passed to Prometheus as evaluation timeout
func (t *MetricsTool) QueryMetrics(ctx context.Context, args map[string]any) (any, error) {
	query, err := requireString(args, "query")
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("query", query)

	if ts, ok := args["time"].(string); ok && ts != "" {
		params.Set("time", ts)
	}
	if to, ok := args["timeout"].(string); ok && to != "" {
		params.Set("timeout", to)
	}

	endpoint := fmt.Sprintf("%s/api/v1/query?%s", t.baseURL, params.Encode())

	var resp prometheusResponse
	if err := t.hc.get(ctx, endpoint, nil, &resp); err != nil {
		return nil, fmt.Errorf("query_metrics: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("query_metrics: prometheus error %s: %s", resp.ErrorType, resp.Error)
	}

	return map[string]any{
		"resultType": resp.Data.ResultType,
		"result":     resp.Data.Result,
	}, nil
}

// QueryMetricsRange executes a PromQL range query.
// args:
//
//	query string  (required) — PromQL expression
//	start string  (required) — RFC3339 or Unix timestamp
//	end   string  (required) — RFC3339 or Unix timestamp
//	step  string  (required) — duration e.g. "1m", "5m"
func (t *MetricsTool) QueryMetricsRange(ctx context.Context, args map[string]any) (any, error) {
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
	step, err := requireString(args, "step")
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("start", start)
	params.Set("end", end)
	params.Set("step", step)

	endpoint := fmt.Sprintf("%s/api/v1/query_range?%s", t.baseURL, params.Encode())

	var resp prometheusResponse
	if err := t.hc.get(ctx, endpoint, nil, &resp); err != nil {
		return nil, fmt.Errorf("query_metrics_range: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("query_metrics_range: %s: %s", resp.ErrorType, resp.Error)
	}

	return map[string]any{
		"resultType": resp.Data.ResultType,
		"result":     resp.Data.Result,
	}, nil
}

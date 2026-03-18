package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMetricsTool_QueryMetrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			t.Errorf("expected path /api/v1/query, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "up" {
			t.Errorf("expected query up, got %s", r.URL.Query().Get("query"))
		}

		resp := prometheusResponse{
			Status: "success",
			Data: struct {
				ResultType string `json:"resultType"`
				Result     any    `json:"result"`
			}{
				ResultType: "vector",
				Result:     []any{"result_data"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	tool := NewMetricsTool(ts.URL, 5*time.Second)
	args := map[string]any{"query": "up"}
	result, err := tool.QueryMetrics(context.Background(), args)
	if err != nil {
		t.Fatalf("QueryMetrics failed: %v", err)
	}

	resMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if resMap["resultType"] != "vector" {
		t.Errorf("expected resultType vector, got %v", resMap["resultType"])
	}
}

func TestMetricsTool_QueryMetricsRange(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query_range" {
			t.Errorf("expected path /api/v1/query_range, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "up" {
			t.Errorf("expected query up, got %s", r.URL.Query().Get("query"))
		}
		if r.URL.Query().Get("start") != "2023-01-01T00:00:00Z" {
			t.Errorf("expected start 2023-01-01T00:00:00Z, got %s", r.URL.Query().Get("start"))
		}

		resp := prometheusResponse{
			Status: "success",
			Data: struct {
				ResultType string `json:"resultType"`
				Result     any    `json:"result"`
			}{
				ResultType: "matrix",
				Result:     []any{"range_data"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	tool := NewMetricsTool(ts.URL, 5*time.Second)
	args := map[string]any{
		"query": "up",
		"start": "2023-01-01T00:00:00Z",
		"end":   "2023-01-01T01:00:00Z",
		"step":  "1m",
	}
	result, err := tool.QueryMetricsRange(context.Background(), args)
	if err != nil {
		t.Fatalf("QueryMetricsRange failed: %v", err)
	}

	resMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if resMap["resultType"] != "matrix" {
		t.Errorf("expected resultType matrix, got %v", resMap["resultType"])
	}
}

package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLogsTool_QueryLogs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/query" {
			t.Errorf("expected path /loki/api/v1/query, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "{app=\"api\"}" {
			t.Errorf("expected query {app=\"api\"}, got %s", r.URL.Query().Get("query"))
		}

		resp := lokiQueryResponse{
			Status: "success",
			Data: struct {
				ResultType string `json:"resultType"`
				Result     any    `json:"result"`
			}{
				ResultType: "streams",
				Result:     []any{"log_data"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	tool := NewLogsTool(ts.URL, 5*time.Second)
	args := map[string]any{"query": "{app=\"api\"}"}
	result, err := tool.QueryLogs(context.Background(), args)
	if err != nil {
		t.Fatalf("QueryLogs failed: %v", err)
	}

	resMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if resMap["resultType"] != "streams" {
		t.Errorf("expected resultType streams, got %v", resMap["resultType"])
	}
}

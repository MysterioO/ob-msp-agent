package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTracesTool_SearchTraces(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" {
			t.Errorf("expected path /api/search, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "{service=\"api\"}" {
			t.Errorf("expected query {service=\"api\"}, got %s", r.URL.Query().Get("q"))
		}

		w.Write([]byte(`{"traces": [{"traceID": "abc"}]}`))
	}))
	defer ts.Close()

	tool := NewTracesTool(ts.URL, 5*time.Second)
	args := map[string]any{"query": "{service=\"api\"}"}
	result, err := tool.SearchTraces(context.Background(), args)
	if err != nil {
		t.Fatalf("SearchTraces failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestTracesTool_GetTrace(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/traces/123" {
			t.Errorf("expected path /api/traces/123, got %s", r.URL.Path)
		}
		w.Write([]byte(`{"traceID": "123"}`))
	}))
	defer ts.Close()

	tool := NewTracesTool(ts.URL, 5*time.Second)
	args := map[string]any{"trace_id": "123"}
	result, err := tool.GetTrace(context.Background(), args)
	if err != nil {
		t.Fatalf("GetTrace failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

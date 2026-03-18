package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAlertsTool_GetActiveAlerts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/alerts" {
			t.Errorf("expected path /api/v2/alerts, got %s", r.URL.Path)
		}
		
		alerts := []any{
			map[string]any{"labels": map[string]any{"alertname": "TestAlert"}},
		}
		json.NewEncoder(w).Encode(alerts)
	}))
	defer ts.Close()

	tool := NewAlertsTool(ts.URL, 5*time.Second)
	result, err := tool.GetActiveAlerts(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetActiveAlerts failed: %v", err)
	}

	resSlice, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(resSlice) != 1 {
		t.Errorf("expected 1 alert, got %d", len(resSlice))
	}
}

func TestAlertsTool_CreateSilence(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/silences" {
			t.Errorf("expected path /api/v2/silences, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var payload silence
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode silence payload: %v", err)
		}

		if payload.Comment != "test silence" {
			t.Errorf("expected comment 'test silence', got '%s'", payload.Comment)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"silenceID": "12345"})
	}))
	defer ts.Close()

	tool := NewAlertsTool(ts.URL, 5*time.Second)
	args := map[string]any{
		"matchers": []any{
			map[string]any{"name": "alertname", "value": "TestAlert", "is_regex": false},
		},
		"duration": "1h",
		"comment":  "test silence",
	}
	result, err := tool.CreateSilence(context.Background(), args)
	if err != nil {
		t.Fatalf("CreateSilence failed: %v", err)
	}

	resMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if resMap["silenceID"] != "12345" {
		t.Errorf("expected silenceID 12345, got %v", resMap["silenceID"])
	}
}

package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSlackTool_PostMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat.postMessage" {
			t.Errorf("expected path /api/chat.postMessage, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}

		var msg slackMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Errorf("failed to decode slack message: %v", err)
		}

		if msg.Text != "hello world" {
			t.Errorf("expected text 'hello world', got '%s'", msg.Text)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok": true, "ts": "123456789.0001"}`))
	}))
	defer ts.Close()

	tool := NewSlackTool("test-token", "#general", 5*time.Second)
	tool.baseURL = ts.URL // Override baseURL for testing

	args := map[string]any{"text": "hello world"}
	result, err := tool.PostMessage(context.Background(), args)
	if err != nil {
		t.Fatalf("PostMessage failed: %v", err)
	}

	resMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}
	if resMap["ok"] != true {
		t.Errorf("expected ok true, got %v", resMap["ok"])
	}
}

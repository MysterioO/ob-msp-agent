package tools

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// TracesTool queries Tempo for distributed traces.
// The agent uses this to find slow spans, error traces, and to correlate
// a trace ID back to its service graph.
type TracesTool struct {
	tempoURL string
	hc       *httpClient
}

func NewTracesTool(tempoURL string, timeout time.Duration) *TracesTool {
	return &TracesTool{tempoURL: tempoURL, hc: newHTTPClient(timeout)}
}

// SearchTraces searches for traces using TraceQL.
// args:
//
//	query    string  (required) — TraceQL expression e.g. `{.http.status_code=500}`
//	start    string  (optional) — Unix timestamp seconds; defaults to 1h ago
//	end      string  (optional) — Unix timestamp seconds; defaults to now
//	limit    int     (optional) — max traces to return; defaults to 20
//	min_dur  string  (optional) — minimum duration filter e.g. "500ms", "1s"
func (t *TracesTool) SearchTraces(ctx context.Context, args map[string]any) (any, error) {
	query, err := requireString(args, "query")
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("q", query)

	start := fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix())
	end := fmt.Sprintf("%d", time.Now().Unix())
	if s, ok := args["start"].(string); ok && s != "" {
		start = s
	}
	if e, ok := args["end"].(string); ok && e != "" {
		end = e
	}
	params.Set("start", start)
	params.Set("end", end)

	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	params.Set("limit", fmt.Sprintf("%d", limit))

	if minDur, ok := args["min_dur"].(string); ok && minDur != "" {
		params.Set("minDuration", minDur)
	}

	endpoint := fmt.Sprintf("%s/api/search?%s", t.tempoURL, params.Encode())

	var result any
	if err := t.hc.get(ctx, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("search_traces: %w", err)
	}
	return result, nil
}

// GetTrace fetches a single trace by ID from Tempo.
// Returns the full span tree which the agent can use to pinpoint
// the slow or erroring operation.
// args:
//
//	trace_id string (required) — hex trace ID
func (t *TracesTool) GetTrace(ctx context.Context, args map[string]any) (any, error) {
	traceID, err := requireString(args, "trace_id")
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/api/traces/%s", t.tempoURL, traceID)

	var result any
	if err := t.hc.get(ctx, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("get_trace: %w", err)
	}
	return result, nil
}

// FindSlowTraces is a convenience wrapper that searches for traces
// slower than a given duration — common first step in latency investigations.
// args:
//
//	service  string  (required) — service name label
//	min_dur  string  (required) — minimum duration e.g. "2s"
//	start    string  (optional)
//	end      string  (optional)
//	limit    int     (optional)
func (t *TracesTool) FindSlowTraces(ctx context.Context, args map[string]any) (any, error) {
	service, err := requireString(args, "service")
	if err != nil {
		return nil, err
	}
	minDur, err := requireString(args, "min_dur")
	if err != nil {
		return nil, err
	}

	// Build a TraceQL query scoped to the service.
	query := fmt.Sprintf(`{resource.service.name="%s"}`, service)

	return t.SearchTraces(ctx, map[string]any{
		"query":   query,
		"min_dur": minDur,
		"start":   args["start"],
		"end":     args["end"],
		"limit":   args["limit"],
	})
}

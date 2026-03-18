package tools

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// GrafanaTool wraps the Grafana HTTP API.
// The agent uses this to pull rendered panel data and annotations —
// useful for building summaries without needing to re-query the underlying data sources.
type GrafanaTool struct {
	grafanaURL string
	apiToken   string
	hc         *httpClient
}

func NewGrafanaTool(grafanaURL, apiToken string, timeout time.Duration) *GrafanaTool {
	return &GrafanaTool{
		grafanaURL: grafanaURL,
		apiToken:   apiToken,
		hc:         newHTTPClient(timeout),
	}
}

func (t *GrafanaTool) authHeader() map[string]string {
	if t.apiToken != "" {
		return map[string]string{"Authorization": "Bearer " + t.apiToken}
	}
	return nil
}

// SearchDashboards searches for Grafana dashboards by query string or tag.
// args:
//
//	query string (optional) — freetext search against dashboard title
//	tag   string (optional) — filter by Grafana dashboard tag
//	limit int    (optional) — max results; defaults to 10
func (t *GrafanaTool) SearchDashboards(ctx context.Context, args map[string]any) (any, error) {
	params := url.Values{}
	params.Set("type", "dash-db")

	if q, ok := args["query"].(string); ok && q != "" {
		params.Set("query", q)
	}
	if tag, ok := args["tag"].(string); ok && tag != "" {
		params.Set("tag", tag)
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	params.Set("limit", fmt.Sprintf("%d", limit))

	endpoint := fmt.Sprintf("%s/api/search?%s", t.grafanaURL, params.Encode())

	var result any
	if err := t.hc.get(ctx, endpoint, t.authHeader(), &result); err != nil {
		return nil, fmt.Errorf("search_dashboards: %w", err)
	}
	return result, nil
}

// GetDashboard fetches a dashboard definition by UID.
// args:
//
//	uid string (required) — Grafana dashboard UID
func (t *GrafanaTool) GetDashboard(ctx context.Context, args map[string]any) (any, error) {
	uid, err := requireString(args, "uid")
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/api/dashboards/uid/%s", t.grafanaURL, uid)

	var result any
	if err := t.hc.get(ctx, endpoint, t.authHeader(), &result); err != nil {
		return nil, fmt.Errorf("get_dashboard: %w", err)
	}
	return result, nil
}

// GetAnnotations fetches Grafana annotations in a time range.
// Annotations often capture deployment events, incidents, and on-call notes —
// valuable context when correlating anomalies to changes.
// args:
//
//	from         string (optional) — Unix ms or relative e.g. "now-1h"; defaults to 1h ago
//	to           string (optional) — defaults to now
//	dashboard_id int    (optional) — scope to a specific dashboard
//	tags         []string (optional) — filter by annotation tags
//	limit        int    (optional) — defaults to 50
func (t *GrafanaTool) GetAnnotations(ctx context.Context, args map[string]any) (any, error) {
	params := url.Values{}

	// Default: last 1 hour in Unix milliseconds.
	fromMs := fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).UnixMilli())
	toMs := fmt.Sprintf("%d", time.Now().UnixMilli())

	if f, ok := args["from"].(string); ok && f != "" {
		fromMs = f
	}
	if to, ok := args["to"].(string); ok && to != "" {
		toMs = to
	}
	params.Set("from", fromMs)
	params.Set("to", toMs)

	if dbID, ok := args["dashboard_id"].(float64); ok {
		params.Set("dashboardId", fmt.Sprintf("%d", int(dbID)))
	}

	if tags, ok := args["tags"].([]any); ok {
		for _, tag := range tags {
			if ts, ok := tag.(string); ok {
				params.Add("tags", ts)
			}
		}
	}

	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	params.Set("limit", fmt.Sprintf("%d", limit))

	endpoint := fmt.Sprintf("%s/api/annotations?%s", t.grafanaURL, params.Encode())

	var result any
	if err := t.hc.get(ctx, endpoint, t.authHeader(), &result); err != nil {
		return nil, fmt.Errorf("get_annotations: %w", err)
	}
	return result, nil
}

// CreateAnnotation creates a Grafana annotation — useful for the agent to
// mark the start/end of an investigation or a remediation action on dashboards.
// args:
//
//	text         string   (required) — annotation description
//	tags         []string (optional) — e.g. ["sre-agent", "auto-remediation"]
//	dashboard_id int      (optional) — pin to a specific dashboard
//	panel_id     int      (optional) — pin to a specific panel
func (t *GrafanaTool) CreateAnnotation(ctx context.Context, args map[string]any) (any, error) {
	text, err := requireString(args, "text")
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"text": text,
		"time": time.Now().UnixMilli(),
		"tags": []string{"sre-agent"},
	}

	if tags, ok := args["tags"].([]any); ok {
		strTags := []string{"sre-agent"}
		for _, t := range tags {
			if ts, ok := t.(string); ok {
				strTags = append(strTags, ts)
			}
		}
		payload["tags"] = strTags
	}

	if dbID, ok := args["dashboard_id"].(float64); ok {
		payload["dashboardId"] = int(dbID)
	}
	if panelID, ok := args["panel_id"].(float64); ok {
		payload["panelId"] = int(panelID)
	}

	// Use a small helper to POST JSON since our httpClient only does GET.
	err = postJSON(ctx, t.hc, fmt.Sprintf("%s/api/annotations", t.grafanaURL), t.authHeader(), payload)
	if err != nil {
		return nil, err
	}
	return "Annotation created successfully", nil
}

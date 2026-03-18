package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// AlertsTool wraps the Alertmanager v2 API.
// The agent uses this to get situational awareness and to take remediation
// actions (creating silences) without needing human intervention for
// known noisy alerts during an incident.
type AlertsTool struct {
	alertmanagerURL string
	hc              *httpClient
}

func NewAlertsTool(alertmanagerURL string, timeout time.Duration) *AlertsTool {
	return &AlertsTool{alertmanagerURL: alertmanagerURL, hc: newHTTPClient(timeout)}
}

// GetActiveAlerts fetches all currently firing alerts from Alertmanager.
// args:
//
//	filter    []string (optional) — Alertmanager matchers e.g. ["severity=critical","env=prod"]
//	silenced  bool     (optional) — include silenced alerts; defaults to false
//	inhibited bool     (optional) — include inhibited alerts; defaults to false
func (t *AlertsTool) GetActiveAlerts(ctx context.Context, args map[string]any) (any, error) {
	params := url.Values{}

	if filters, ok := args["filter"].([]any); ok {
		for _, f := range filters {
			if fs, ok := f.(string); ok {
				params.Add("filter", fs)
			}
		}
	}

	silenced := false
	if s, ok := args["silenced"].(bool); ok {
		silenced = s
	}
	params.Set("silenced", fmt.Sprintf("%t", silenced))

	inhibited := false
	if i, ok := args["inhibited"].(bool); ok {
		inhibited = i
	}
	params.Set("inhibited", fmt.Sprintf("%t", inhibited))

	endpoint := fmt.Sprintf("%s/api/v2/alerts?%s", t.alertmanagerURL, params.Encode())

	var result any
	if err := t.hc.get(ctx, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("get_active_alerts: %w", err)
	}
	return result, nil
}

// silence is the Alertmanager silence payload.
type silence struct {
	Matchers  []silenceMatcher `json:"matchers"`
	StartsAt  time.Time        `json:"startsAt"`
	EndsAt    time.Time        `json:"endsAt"`
	CreatedBy string           `json:"createdBy"`
	Comment   string           `json:"comment"`
}

type silenceMatcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual"`
}

// CreateSilence creates an Alertmanager silence.
// The agent uses this to mute known noisy alerts during a planned maintenance
// window or when an alert is already being actively worked.
// args:
//
//	matchers   []map[string]any (required) — [{name, value, is_regex}]
//	duration   string           (required) — e.g. "2h", "30m"
//	created_by string           (optional) — defaults to "sre-agent"
//	comment    string           (required) — reason for the silence
func (t *AlertsTool) CreateSilence(ctx context.Context, args map[string]any) (any, error) {
	rawMatchers, ok := args["matchers"].([]any)
	if !ok || len(rawMatchers) == 0 {
		return nil, fmt.Errorf("create_silence: matchers is required and must be a non-empty array")
	}

	durationStr, err := requireString(args, "duration")
	if err != nil {
		return nil, err
	}
	dur, err := time.ParseDuration(durationStr)
	if err != nil {
		return nil, fmt.Errorf("create_silence: invalid duration %q: %w", durationStr, err)
	}

	comment, err := requireString(args, "comment")
	if err != nil {
		return nil, err
	}

	createdBy := "sre-agent"
	if cb, ok := args["created_by"].(string); ok && cb != "" {
		createdBy = cb
	}

	var matchers []silenceMatcher
	for _, raw := range rawMatchers {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		sm := silenceMatcher{
			Name:    getString(m, "name"),
			Value:   getString(m, "value"),
			IsRegex: getBool(m, "is_regex"),
			IsEqual: true,
		}
		if sm.IsRegex {
			sm.IsEqual = false
		}
		matchers = append(matchers, sm)
	}

	now := time.Now().UTC()
	payload := silence{
		Matchers:  matchers,
		StartsAt:  now,
		EndsAt:    now.Add(dur),
		CreatedBy: createdBy,
		Comment:   comment,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("create_silence: marshal: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v2/silences", t.alertmanagerURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create_silence: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.hc.c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create_silence: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("create_silence: alertmanager returned %d: %s", resp.StatusCode, respBody)
	}

	var result any
	_ = json.Unmarshal(respBody, &result)
	return result, nil
}

// DeleteSilence removes an existing silence by ID.
// args:
//
//	silence_id string (required)
func (t *AlertsTool) DeleteSilence(ctx context.Context, args map[string]any) (any, error) {
	silenceID, err := requireString(args, "silence_id")
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/api/v2/silence/%s", t.alertmanagerURL, silenceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("delete_silence: %w", err)
	}

	resp, err := t.hc.c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("delete_silence: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("delete_silence: %d: %s", resp.StatusCode, body)
	}

	return map[string]any{"deleted": silenceID}, nil
}

// helper — safe string extraction from map
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SlackTool posts messages to Slack using the Web API.
// The agent uses this to surface investigation summaries, incident updates,
// and actionable findings directly into the team's existing alert channels.
type SlackTool struct {
	token      string
	defaultChn string
	baseURL    string
	hc         *httpClient
}

func NewSlackTool(token, defaultChannel string, timeout time.Duration) *SlackTool {
	return &SlackTool{
		token:      token,
		defaultChn: defaultChannel,
		baseURL:    "https://slack.com",
		hc:         newHTTPClient(timeout),
	}
}

type slackMessage struct {
	Channel     string            `json:"channel"`
	Text        string            `json:"text,omitempty"`
	Blocks      []slackBlock      `json:"blocks,omitempty"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type slackBlock struct {
	Type string         `json:"type"`
	Text *slackTextObj  `json:"text,omitempty"`
	// Fields used for section blocks with multiple items
}

type slackTextObj struct {
	Type string `json:"type"` // "mrkdwn" or "plain_text"
	Text string `json:"text"`
}

type slackAttachment struct {
	Color  string       `json:"color,omitempty"` // "good", "warning", "danger", or hex
	Blocks []slackBlock `json:"blocks,omitempty"`
}

// PostMessage posts a Slack message.
// args:
//
//	channel  string  (optional) — defaults to configured default channel
//	text     string  (required) — message text; supports Slack mrkdwn
//	color    string  (optional) — "good", "warning", "danger" or hex; triggers attachment style
//	title    string  (optional) — bold title rendered above the text
func (t *SlackTool) PostMessage(ctx context.Context, args map[string]any) (any, error) {
	if t.token == "" {
		return nil, fmt.Errorf("post_slack_message: SLACK_BOT_TOKEN is not configured")
	}

	text, err := requireString(args, "text")
	if err != nil {
		return nil, err
	}

	channel := t.defaultChn
	if ch, ok := args["channel"].(string); ok && ch != "" {
		channel = ch
	}

	msg := slackMessage{Channel: channel}

	// If a color or title is provided, use an attachment for richer formatting.
	color, hasColor := args["color"].(string)
	title, hasTitle := args["title"].(string)

	if hasColor || hasTitle {
		var blocks []slackBlock
		if hasTitle {
			blocks = append(blocks, slackBlock{
				Type: "section",
				Text: &slackTextObj{Type: "mrkdwn", Text: fmt.Sprintf("*%s*", title)},
			})
		}
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackTextObj{Type: "mrkdwn", Text: text},
		})
		msg.Attachments = []slackAttachment{{Color: color, Blocks: blocks}}
	} else {
		msg.Text = text
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("post_slack_message: marshal: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/chat.postMessage", t.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("post_slack_message: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.token)

	resp, err := t.hc.c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post_slack_message: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var slackResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
		TS    string `json:"ts,omitempty"`
	}
	if err := json.Unmarshal(respBody, &slackResp); err != nil {
		return nil, fmt.Errorf("post_slack_message: decode response: %w", err)
	}
	if !slackResp.OK {
		return nil, fmt.Errorf("post_slack_message: slack API error: %s", slackResp.Error)
	}

	return map[string]any{
		"ok":        true,
		"channel":   channel,
		"timestamp": slackResp.TS,
		"posted_at": time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// PostIncidentSummary is a higher-level wrapper that formats a structured
// incident summary into a Slack attachment with severity colour coding.
// args:
//
//	channel   string (optional)
//	severity  string (required) — "critical", "warning", "info"
//	title     string (required) — e.g. "High error rate on payments-service"
//	summary   string (required) — investigation findings in mrkdwn
//	trace_url string (optional) — link to representative trace in Grafana/Tempo
//	runbook   string (optional) — runbook URL
func (t *SlackTool) PostIncidentSummary(ctx context.Context, args map[string]any) (any, error) {
	severity, _ := args["severity"].(string)
	title, err := requireString(args, "title")
	if err != nil {
		return nil, err
	}
	summary, err := requireString(args, "summary")
	if err != nil {
		return nil, err
	}

	colorMap := map[string]string{
		"critical": "danger",
		"warning":  "warning",
		"info":     "good",
	}
	color := colorMap[severity]
	if color == "" {
		color = "#439FE0"
	}

	text := summary
	if traceURL, ok := args["trace_url"].(string); ok && traceURL != "" {
		text += fmt.Sprintf("\n\n:mag: <%s|View Trace>", traceURL)
	}
	if runbook, ok := args["runbook"].(string); ok && runbook != "" {
		text += fmt.Sprintf("  :book: <%s|Runbook>", runbook)
	}

	return t.PostMessage(ctx, map[string]any{
		"channel": args["channel"],
		"title":   fmt.Sprintf(":%s: %s", severityEmoji(severity), title),
		"text":    text,
		"color":   color,
	})
}

func severityEmoji(s string) string {
	switch s {
	case "critical":
		return "rotating_light"
	case "warning":
		return "warning"
	default:
		return "information_source"
	}
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// ------------------------------------------------------------------ //
// Slack Event Types
// ------------------------------------------------------------------ //

type SlackEventPayload struct {
	Type      string `json:"type"`
	Event     Event  `json:"event"`
	Challenge string `json:"challenge"`
}

type Event struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Text    string `json:"text"`
	Ts      string `json:"ts"`
	BotID   string `json:"bot_id"`
}

// ------------------------------------------------------------------ //
// Main Server
// ------------------------------------------------------------------ //

func main() {
	http.HandleFunc("/slack/events", handleSlackEvent)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🤖 Orchestrator listening on :%s/slack/events...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleSlackEvent(w http.ResponseWriter, r *http.Request) {
	var payload SlackEventPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// 1. Respond to Slack's URL verification challenge
	if payload.Type == "url_verification" {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(payload.Challenge))
		return
	}

	// 2. Process incoming messages
	if payload.Type == "event_callback" && payload.Event.Type == "message" {
		// Ignore messages from bots (including our own) to prevent infinite loops
		if payload.Event.BotID == "" {
			// Acknowledge the Slack event immediately with a 200 OK
			w.WriteHeader(http.StatusOK)

			// Spin off the investigation in a goroutine so we don't block
			go triggerAgent(payload.Event)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// ------------------------------------------------------------------ //
// Agent Orchestration & MCP Client
// ------------------------------------------------------------------ //

func triggerAgent(event Event) {
	log.Printf("🚨 Alert detected in channel %s. Triggering agent...", event.Channel)
	ctx := context.Background()

	// 1. Start the MCP Server as a subprocess using stdio
	serverCmd := "./bin/mcp-server"

	// Create the Stdio client (passing empty array for args)
	mcpClient, err := client.NewStdioMCPClient(serverCmd, []string{}, os.Environ()...)
	if err != nil {
		log.Printf("❌ Failed to start MCP server: %v", err)
		return
	}
	defer mcpClient.Close()

	// Initialize the MCP client connection
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "sre-orchestrator",
		Version: "1.0.0",
	}

	_, err = mcpClient.Initialize(ctx, initReq)
	if err != nil {
		log.Printf("❌ Failed to initialize MCP client: %v", err)
		return
	}

	// 2. Fetch all available tools from your MCP Server
	toolsResp, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		log.Printf("❌ Failed to list tools: %v", err)
		return
	}

	log.Printf("🔧 Loaded %d tools from MCP server.", len(toolsResp.Tools))

	// 3. Construct the prompt
	prompt := fmt.Sprintf(`
You are an autonomous SRE agent. A new alert just fired:

"%s"

Please investigate this using your available tools. 
When you have a hypothesis for the root cause:
1. Create a Grafana annotation to mark the investigation.
2. Post a structured summary back to Slack.

IMPORTANT: You MUST use the post_incident_summary tool and include the following parameters exactly so your response appears in the alert's thread:
- channel: "%s"
- thread_ts: "%s"
`, event.Text, event.Channel, event.Ts)

	log.Printf("🧠 AI Prompt Generated:\n%s", prompt)

	// ------------------------------------------------------------------ //
	// LLM EXECUTION LOOP
	// ------------------------------------------------------------------ //

	log.Printf("🔁 Starting AI tool execution loop...")

	// Maximum iterations to prevent infinite loops if the AI gets confused
	maxSteps := 10 

	for i := 0; i < maxSteps; i++ {
		
		// TODO 1: Call your LLM API here. 
		// Pass it the `prompt` (or conversation history) and `toolsResp.Tools`.
		// response := yourLLMClient.SendMessage(prompt, toolsResp.Tools)

		// ---------------------------------------------------
		// MOCK VARIABLES: Replace these with data from your LLM response
		llmWantsToCallTool := false               // e.g., response.HasToolCall()
		llmRequestedToolName := "query_metrics"   // e.g., response.ToolCall.Name
		var llmProvidedArgs map[string]interface{} // e.g., response.ToolCall.Arguments
		// ---------------------------------------------------

		// If the LLM didn't request a tool, it means it's done and returned a text summary!
		if !llmWantsToCallTool {
			log.Printf("✅ AI finished investigation after %d steps.", i)
			break 
		}

		log.Printf("🛠️  AI requested tool execution: %s", llmRequestedToolName)

		// Prepare the MCP Request to forward to your local server
		callReq := mcp.CallToolRequest{}
		callReq.Params.Name = llmRequestedToolName
		callReq.Params.Arguments = llmProvidedArgs

		// Execute the tool locally via your mcp-server subprocess
		toolResult, err := mcpClient.CallTool(ctx, callReq)
		
		var resultText string
		if err != nil {
			log.Printf("⚠️ Tool execution failed: %v", err)
			resultText = fmt.Sprintf("Error executing tool: %v", err)
		} else {
			// Extract the text output from the MCP server's response
			if len(toolResult.Content) > 0 {
				if textBlock, ok := toolResult.Content[0].(mcp.TextContent); ok {
					resultText = textBlock.Text
				} else {
					resultText = "Tool executed, but returned non-text format."
				}
			} else {
				resultText = "Tool executed successfully but returned no output."
			}
		}

		log.Printf("📥 Tool %s returned %d bytes of data.", llmRequestedToolName, len(resultText))

		// TODO 2: Append `resultText` back into your LLM's conversation history
		// so it can read the metrics/logs and decide the next action on the next loop.
		// prompt += fmt.Sprintf("\nResult of %s:\n%s\nWhat next?", llmRequestedToolName, resultText)
	}

	log.Printf("🏁 Investigation orchestration completed for thread %s", event.Ts)
}
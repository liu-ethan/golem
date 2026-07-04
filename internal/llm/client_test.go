package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tencent-docs/golem/internal/config"
)

func TestStreamChatTextDelta(t *testing.T) {
	sseBody, err := os.ReadFile("testdata/stream_text.sse")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("missing api key header")
		}
		var body messagesRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !body.Stream {
			t.Error("expected stream=true")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write(sseBody)
	}))
	defer srv.Close()

	client := NewAnthropicClient(srv.URL, "test-key", "test-model")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, err := client.StreamChat(ctx, ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: []ContentBlock{{Type: "text", Text: "hi"}}}},
	})
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}

	var text strings.Builder
	var endUsage Usage
	for evt := range events {
		switch evt.Type {
		case StreamEventTextDelta:
			text.WriteString(evt.Text)
		case StreamEventMessageEnd:
			endUsage = evt.Usage
		case StreamEventError:
			t.Fatalf("stream error: %v", evt.Err)
		}
	}

	if got := text.String(); got != "Hello world" {
		t.Errorf("text = %q", got)
	}
	if endUsage.InputTokens != 12 || endUsage.OutputTokens != 2 {
		t.Errorf("usage = %+v", endUsage)
	}
}

func TestStreamChatThinkingDelta(t *testing.T) {
	sseBody, err := os.ReadFile("testdata/stream_thinking.sse")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write(sseBody)
	}))
	defer srv.Close()

	client := NewAnthropicClient(srv.URL, "test-key", "test-model")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, err := client.StreamChat(ctx, ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: []ContentBlock{{Type: "text", Text: "hi"}}}},
	})
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}

	var thinking, text strings.Builder
	for evt := range events {
		switch evt.Type {
		case StreamEventThinkingDelta:
			thinking.WriteString(evt.Text)
		case StreamEventTextDelta:
			text.WriteString(evt.Text)
		case StreamEventError:
			t.Fatalf("stream error: %v", evt.Err)
		}
	}

	if got := thinking.String(); got != "Let me think" {
		t.Errorf("thinking = %q", got)
	}
	if got := text.String(); got != "Answer here" {
		t.Errorf("text = %q", got)
	}
}

func TestCompleteReturnsTextAndUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body messagesRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Stream {
			t.Error("expected stream=false")
		}
		if body.System != "summarize" {
			t.Errorf("system = %q", body.System)
		}
		_ = json.NewEncoder(w).Encode(completeResponse{
			Content: []ContentBlock{{Type: "text", Text: "summary text"}},
			Usage: struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			}{InputTokens: 50, OutputTokens: 10},
		})
	}))
	defer srv.Close()

	var hookUsage Usage
	var hookType string
	client := NewAnthropicClient(srv.URL, "test-key", "test-model",
		WithTokenUsageHook(func(_ string, u Usage, callType string) {
			hookUsage = u
			hookType = callType
		}),
	)

	text, usage, err := client.Complete(context.Background(), CompleteRequest{
		System: "summarize",
		Messages: []Message{
			{Role: RoleUser, Content: []ContentBlock{{Type: "text", Text: "long conversation"}}},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if text != "summary text" {
		t.Errorf("text = %q", text)
	}
	if usage.InputTokens != 50 || usage.OutputTokens != 10 {
		t.Errorf("usage = %+v", usage)
	}
	if hookType != callTypeComplete {
		t.Errorf("hook callType = %q", hookType)
	}
	if hookUsage.OutputTokens != 10 {
		t.Errorf("hook usage = %+v", hookUsage)
	}
}

func TestStreamChatAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"type":"auth","message":"invalid key"}}`)
	}))
	defer srv.Close()

	client := NewAnthropicClient(srv.URL, "bad", "test-model")
	_, err := client.StreamChat(context.Background(), ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: []ContentBlock{{Type: "text", Text: "hi"}}}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid key") {
		t.Errorf("err = %v", err)
	}
}

func TestStreamChatToolUse(t *testing.T) {
	sse := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":0}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"read_file","input":{}}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"main.go\"}"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","usage":{"output_tokens":3}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sse)
	}))
	defer srv.Close()

	client := NewAnthropicClient(srv.URL, "test-key", "test-model")
	events, err := client.StreamChat(context.Background(), ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: []ContentBlock{{Type: "text", Text: "read main.go"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var toolName string
	var toolInput map[string]any
	for evt := range events {
		if evt.Type == StreamEventToolUseStart {
			toolName = evt.ToolName
		}
		if evt.ToolInput != nil {
			toolInput = evt.ToolInput
		}
	}
	if toolName != "read_file" {
		t.Errorf("toolName = %q", toolName)
	}
	if toolInput["path"] != "main.go" {
		t.Errorf("toolInput = %v", toolInput)
	}
}

func TestStreamChatTokenUsageHook(t *testing.T) {
	sseBody, err := os.ReadFile("testdata/stream_text.sse")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write(sseBody)
	}))
	defer srv.Close()

	var hookUsage Usage
	var hookType string
	client := NewAnthropicClient(srv.URL, "test-key", "test-model",
		WithTokenUsageHook(func(_ string, u Usage, callType string) {
			hookUsage = u
			hookType = callType
		}),
		WithSessionID("sess-1"),
	)

	events, err := client.StreamChat(context.Background(), ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: []ContentBlock{{Type: "text", Text: "hi"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for range events {
	}

	if hookType != callTypeStream {
		t.Errorf("hook callType = %q", hookType)
	}
	if hookUsage.InputTokens != 12 {
		t.Errorf("hook usage = %+v", hookUsage)
	}
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".golem", "config.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("project .golem/config.yaml not found")
		}
		dir = parent
	}
}

// TestIntegrationStreamChat 向真实 API 发流式请求；需 GOLEM_INTEGRATION=1 且 .golem/config.yaml 有效。
func TestIntegrationStreamChat(t *testing.T) {
	if os.Getenv("GOLEM_INTEGRATION") != "1" {
		t.Skip("set GOLEM_INTEGRATION=1 to run live API test")
	}

	projectRoot := findProjectRoot(t)
	cfg, err := config.LoadConfig(projectRoot, config.Overrides{})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Provider.APIKey == "" {
		t.Skip("no api_key in config")
	}

	client := NewAnthropicClient(cfg.Provider.BaseURL, cfg.Provider.APIKey, cfg.Provider.Model)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	events, err := client.StreamChat(ctx, ChatRequest{
		Messages: []Message{
			{Role: RoleUser, Content: []ContentBlock{{Type: "text", Text: "Reply with exactly: pong"}}},
		},
		MaxTokens: 32,
	})
	if err != nil {
		t.Fatalf("StreamChat: %v", err)
	}

	var text strings.Builder
	for evt := range events {
		if evt.Type == StreamEventError {
			t.Fatalf("stream error: %v", evt.Err)
		}
		if evt.Type == StreamEventTextDelta {
			text.WriteString(evt.Text)
		}
	}
	if text.Len() == 0 {
		t.Error("expected non-empty stream response")
	}
	t.Logf("stream response: %q", text.String())
}

// TestIntegrationComplete 向真实 API 发非流式 Complete；需 GOLEM_INTEGRATION=1。
func TestIntegrationComplete(t *testing.T) {
	if os.Getenv("GOLEM_INTEGRATION") != "1" {
		t.Skip("set GOLEM_INTEGRATION=1 to run live API test")
	}

	projectRoot := findProjectRoot(t)
	cfg, err := config.LoadConfig(projectRoot, config.Overrides{})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Provider.APIKey == "" {
		t.Skip("no api_key in config")
	}

	client := NewAnthropicClient(cfg.Provider.BaseURL, cfg.Provider.APIKey, cfg.Provider.Model)
	text, usage, err := client.Complete(context.Background(), CompleteRequest{
		System: "Reply with plain text only.",
		Messages: []Message{
			{Role: RoleUser, Content: []ContentBlock{{Type: "text", Text: "Say hello in one word."}}},
		},
		MaxTokens: 256,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if text == "" {
		t.Error("expected non-empty complete response")
	}
	if usage.InputTokens == 0 {
		t.Error("expected non-zero input tokens")
	}
	t.Logf("complete: text=%q usage=%+v", text, usage)
}

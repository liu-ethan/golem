package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultMaxTokens     = 8192
	anthropicAPIVersion  = "2023-06-01"
	callTypeStream       = "stream"
	callTypeComplete     = "complete"
)

// Role 表示 Messages API 中的消息角色。
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ContentBlock 对应 Anthropic Messages API 内容块，支持 text / tool_use / tool_result。
type ContentBlock struct {
	Type string `json:"type"`

	Text string `json:"text,omitempty"`

	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`

	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// Message 表示一条对话消息。
type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ToolDefinition 描述可供模型调用的工具 schema。
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// ChatRequest 为 StreamChat 入参。
type ChatRequest struct {
	System    string
	Messages  []Message
	Tools     []ToolDefinition
	MaxTokens int
}

// CompleteRequest 为 Complete 入参，供记忆 Layer 0/1/2 非流式补全。
type CompleteRequest struct {
	System    string
	Messages  []Message
	MaxTokens int
}

// Usage 记录单次 LLM 调用的 token 用量。
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// StreamEventType 标识 StreamChat 通道中的事件种类。
type StreamEventType string

const (
	StreamEventThinkingDelta      StreamEventType = "thinking_delta"
	StreamEventTextDelta          StreamEventType = "text_delta"
	StreamEventToolUseStart       StreamEventType = "tool_use_start"
	StreamEventToolUseInputDelta  StreamEventType = "tool_use_input_delta"
	StreamEventMessageEnd         StreamEventType = "message_end"
	StreamEventError              StreamEventType = "error"
)

// StreamEvent 为 StreamChat 输出的单条 SSE 解析结果。
type StreamEvent struct {
	Type StreamEventType

	Text string

	ToolUseID   string
	ToolName    string
	ToolInput   map[string]any
	InputDelta  string

	Usage Usage
	Err   error
}

// TokenUsageHook 在每次 LLM 调用结束后触发，供 TUI 展示与 token 统计。
type TokenUsageHook func(sessionID string, usage Usage, callType string)

// LLMClient 统一 Anthropic Messages API 风格的流式与非流式调用。
type LLMClient interface {
	StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
	Complete(ctx context.Context, req CompleteRequest) (string, Usage, error)
}

// AnthropicClient 实现 LLMClient，通过可配置 base_url 接入 Claude / DeepSeek 等兼容端点。
type AnthropicClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
	usageHook  TokenUsageHook
	sessionID  string
}

// ClientOption 配置 AnthropicClient 可选行为。
type ClientOption func(*AnthropicClient)

// WithHTTPClient 注入自定义 HTTP 客户端，主要用于测试。
func WithHTTPClient(c *http.Client) ClientOption {
	return func(ac *AnthropicClient) {
		ac.httpClient = c
	}
}

// WithTokenUsageHook 注册 token 用量回调。
func WithTokenUsageHook(hook TokenUsageHook) ClientOption {
	return func(ac *AnthropicClient) {
		ac.usageHook = hook
	}
}

// WithSessionID 设置会话 ID，供 TokenUsageHook 关联统计。
func WithSessionID(id string) ClientOption {
	return func(ac *AnthropicClient) {
		ac.sessionID = id
	}
}

// NewAnthropicClient 根据 provider 配置构造 LLM 客户端。
func NewAnthropicClient(baseURL, apiKey, model string, opts ...ClientOption) *AnthropicClient {
	c := &AnthropicClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SetModel 运行时切换模型名。
func (c *AnthropicClient) SetModel(model string) {
	if model != "" {
		c.model = model
	}
}

// Configure 运行时更新 provider 接入参数；空字符串字段保持不变。
func (c *AnthropicClient) Configure(baseURL, apiKey, model string) {
	if baseURL != "" {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
	if apiKey != "" {
		c.apiKey = apiKey
	}
	if model != "" {
		c.model = model
	}
}

// Model 返回当前配置的模型名。
func (c *AnthropicClient) Model() string {
	return c.model
}

// StreamChat 发起流式 Messages API 请求，在 goroutine 中解析 SSE 并写入 channel。
func (c *AnthropicClient) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	body, err := c.buildRequestBody(req.System, req.Messages, req.Tools, req.MaxTokens, true)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.messagesURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, c.readAPIError(resp)
	}

	events := make(chan StreamEvent, 16)
	go func() {
		defer close(events)
		defer resp.Body.Close()
		usage := c.streamSSE(ctx, resp.Body, events)
		if c.usageHook != nil {
			c.usageHook(c.sessionID, usage, callTypeStream)
		}
	}()
	return events, nil
}

// Complete 发起非流式 Messages API 请求，返回 assistant 文本与 usage。
func (c *AnthropicClient) Complete(ctx context.Context, req CompleteRequest) (string, Usage, error) {
	body, err := c.buildRequestBody(req.System, req.Messages, nil, req.MaxTokens, false)
	if err != nil {
		return "", Usage{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.messagesURL(), bytes.NewReader(body))
	if err != nil {
		return "", Usage{}, err
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", Usage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", Usage{}, c.readAPIError(resp)
	}

	var parsed completeResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", Usage{}, fmt.Errorf("decode complete response: %w", err)
	}

	text := extractText(parsed.Content)
	usage := Usage{
		InputTokens:  parsed.Usage.InputTokens,
		OutputTokens: parsed.Usage.OutputTokens,
	}
	if c.usageHook != nil {
		c.usageHook(c.sessionID, usage, callTypeComplete)
	}
	return text, usage, nil
}

type messagesRequest struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	System    string           `json:"system,omitempty"`
	Messages  []Message        `json:"messages"`
	Tools     []ToolDefinition `json:"tools,omitempty"`
	Stream    bool             `json:"stream"`
}

type completeResponse struct {
	Content []ContentBlock `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// buildRequestBody 序列化 Messages API 请求体。
func (c *AnthropicClient) buildRequestBody(system string, messages []Message, tools []ToolDefinition, maxTokens int, stream bool) ([]byte, error) {
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	req := messagesRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  messages,
		Tools:     tools,
		Stream:    stream,
	}
	return json.Marshal(req)
}

// messagesURL 返回 Messages API 完整 URL。
func (c *AnthropicClient) messagesURL() string {
	return c.baseURL + "/v1/messages"
}

// setHeaders 设置 Anthropic 兼容请求头。
func (c *AnthropicClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)
}

// readAPIError 从非 200 响应体解析错误信息。
func (c *AnthropicClient) readAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var apiErr struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Message != "" {
		return formatAPIError(resp.StatusCode, apiErr.Error.Message)
	}
	return formatAPIError(resp.StatusCode, strings.TrimSpace(string(body)))
}

func formatAPIError(status int, msg string) error {
	if status == http.StatusRequestEntityTooLarge {
		return fmt.Errorf("llm api 413: request too large — context may contain oversized tool results; try /compact or start a new session")
	}
	return fmt.Errorf("llm api %d: %s", status, msg)
}

// streamSSE 逐行解析 SSE，向 events 写入 StreamEvent；返回累计 usage。
func (c *AnthropicClient) streamSSE(ctx context.Context, r io.Reader, events chan<- StreamEvent) Usage {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var usage Usage
	toolInputs := map[int]string{}
	blockTypes := map[int]string{}

	for scanner.Scan() {
		if ctx.Err() != nil {
			events <- StreamEvent{Type: StreamEventError, Err: ctx.Err()}
			return usage
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var evt sseEvent
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			events <- StreamEvent{Type: StreamEventError, Err: fmt.Errorf("parse sse event: %w", err)}
			continue
		}

		switch evt.Type {
		case "message_start":
			if evt.Message != nil {
				usage.InputTokens = evt.Message.Usage.InputTokens
				usage.OutputTokens = evt.Message.Usage.OutputTokens
			}
		case "content_block_start":
			if evt.ContentBlock != nil {
				blockTypes[evt.Index] = evt.ContentBlock.Type
				if evt.ContentBlock.Type == "tool_use" {
					events <- StreamEvent{
						Type:      StreamEventToolUseStart,
						ToolUseID: evt.ContentBlock.ID,
						ToolName:  evt.ContentBlock.Name,
					}
					toolInputs[evt.Index] = ""
				}
			}
		case "content_block_delta":
			if evt.Delta == nil {
				continue
			}
			switch evt.Delta.Type {
			case "thinking_delta":
				events <- StreamEvent{Type: StreamEventThinkingDelta, Text: evt.Delta.Thinking}
			case "text_delta":
				events <- StreamEvent{Type: StreamEventTextDelta, Text: evt.Delta.Text}
			case "input_json_delta":
				toolInputs[evt.Index] += evt.Delta.PartialJSON
				events <- StreamEvent{Type: StreamEventToolUseInputDelta, InputDelta: evt.Delta.PartialJSON}
			}
		case "content_block_stop":
			delete(blockTypes, evt.Index)
			if partial, ok := toolInputs[evt.Index]; ok && partial != "" {
				var input map[string]any
				if err := json.Unmarshal([]byte(partial), &input); err == nil {
					events <- StreamEvent{
						Type:      StreamEventToolUseInputDelta,
						ToolInput: input,
					}
				}
				delete(toolInputs, evt.Index)
			}
		case "message_delta":
			if evt.Usage != nil {
				if evt.Usage.InputTokens > 0 {
					usage.InputTokens = evt.Usage.InputTokens
				}
				if evt.Usage.OutputTokens > 0 {
					usage.OutputTokens = evt.Usage.OutputTokens
				}
			}
		case "message_stop":
			events <- StreamEvent{Type: StreamEventMessageEnd, Usage: usage}
		case "error":
			msg := "stream error"
			if evt.Error != nil {
				msg = evt.Error.Message
			}
			events <- StreamEvent{Type: StreamEventError, Err: fmt.Errorf("%s", msg)}
		}
	}
	if err := scanner.Err(); err != nil {
		events <- StreamEvent{Type: StreamEventError, Err: err}
	}
	return usage
}

type sseEvent struct {
	Type string `json:"type"`
	Index int   `json:"index"`

	Message *struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`

	ContentBlock *struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`

	Delta *struct {
		Type         string `json:"type"`
		Text         string `json:"text"`
		Thinking     string `json:"thinking"`
		PartialJSON  string `json:"partial_json"`
	} `json:"delta"`

	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`

	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// extractText 从非流式响应 content 块拼接文本。
func extractText(blocks []ContentBlock) string {
	var b strings.Builder
	for _, block := range blocks {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}

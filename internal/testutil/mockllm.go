package testutil

import (
	"context"
	"sync"

	"github.com/tencent-docs/golem/internal/llm"
)

// MockResponse 描述一次 StreamChat 调用应产出的 SSE 事件序列。
type MockResponse struct {
	Events []llm.StreamEvent
	Err    error
}

// MockLLM 实现 llm.LLMClient，按调用顺序返回预设的流式与非流式响应，供 agent 测试使用。
type MockLLM struct {
	mu sync.Mutex

	StreamResponses []MockResponse
	StreamIndex     int

	CompleteText      string
	CompleteResponses []string
	CompleteIndex     int
	CompleteUsage     llm.Usage
	CompleteErr       error

	StreamCalls   []llm.ChatRequest
	CompleteCalls []llm.CompleteRequest

	ModelName string
}

// NewMockLLM 创建空的 MockLLM。
func NewMockLLM() *MockLLM {
	return &MockLLM{}
}

// StreamChat 返回队列中下一次预设的 StreamEvent 通道。
func (m *MockLLM) StreamChat(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StreamCalls = append(m.StreamCalls, req)
	if m.StreamIndex >= len(m.StreamResponses) {
		ch := make(chan llm.StreamEvent, 1)
		ch <- llm.StreamEvent{Type: llm.StreamEventError, Err: errNoMockResponse}
		close(ch)
		return ch, nil
	}
	resp := m.StreamResponses[m.StreamIndex]
	m.StreamIndex++

	if resp.Err != nil {
		return nil, resp.Err
	}

	ch := make(chan llm.StreamEvent, len(resp.Events)+1)
	go func() {
		defer close(ch)
		for _, evt := range resp.Events {
			select {
			case <-ctx.Done():
				ch <- llm.StreamEvent{Type: llm.StreamEventError, Err: ctx.Err()}
				return
			case ch <- evt:
			}
		}
	}()
	return ch, nil
}

// Complete 返回预设的非流式补全结果。
func (m *MockLLM) Complete(ctx context.Context, req llm.CompleteRequest) (string, llm.Usage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CompleteCalls = append(m.CompleteCalls, req)
	if m.CompleteErr != nil {
		return "", llm.Usage{}, m.CompleteErr
	}
	text := m.CompleteText
	if m.CompleteIndex < len(m.CompleteResponses) {
		text = m.CompleteResponses[m.CompleteIndex]
		m.CompleteIndex++
	}
	return text, m.CompleteUsage, nil
}

// Reset 清空调用记录与流式响应游标，保留预设响应队列。
func (m *MockLLM) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StreamIndex = 0
	m.CompleteIndex = 0
	m.StreamCalls = nil
	m.CompleteCalls = nil
}

// SetModel 运行时切换模型名，供 Agent.SetModel 测试使用。
func (m *MockLLM) SetModel(model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ModelName = model
}

// Model 返回当前模型名。
func (m *MockLLM) Model() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ModelName
}

// Configure 模拟运行时 provider 更新，供首次配置向导测试使用。
func (m *MockLLM) Configure(_ string, _ string, model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if model != "" {
		m.ModelName = model
	}
}

var errNoMockResponse = &mockError{"no mock stream response configured"}

type mockError struct{ msg string }

func (e *mockError) Error() string { return e.msg }

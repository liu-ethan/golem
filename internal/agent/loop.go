package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tencent-docs/golem/internal/llm"
)

// streamResult 汇总单次 StreamChat 的 assistant 消息与 tool_use 块。
type streamResult struct {
	Message  llm.Message
	ToolUses []llm.ContentBlock
	Usage    llm.Usage
}

// pendingTool 在流式解析过程中累积单个 tool_use 块的元数据与 JSON 输入。
type pendingTool struct {
	id   string
	name string
	raw  strings.Builder
}

// runStreamTurn 调用 StreamChat 消费 SSE 事件，组装 assistant 消息并通过 handler 推送文本增量。
func (a *Agent) runStreamTurn(ctx context.Context, handler EventHandler) (streamResult, error) {
	req := llm.ChatRequest{
		System:    a.systemPrompt,
		Messages:  a.messages,
		Tools:     a.tools.Definitions(),
		MaxTokens: a.maxTokens,
	}

	events, err := a.llm.StreamChat(ctx, req)
	if err != nil {
		return streamResult{}, err
	}

	var (
		textBuf    strings.Builder
		toolUses   []llm.ContentBlock
		pending    []*pendingTool
		usage      llm.Usage
		streamErr  error
	)

	for evt := range events {
		if ctx.Err() != nil {
			return streamResult{}, ctx.Err()
		}
		switch evt.Type {
		case llm.StreamEventTextDelta:
			textBuf.WriteString(evt.Text)
			if handler != nil {
				handler(Event{Type: EventTextDelta, Text: evt.Text})
			}
		case llm.StreamEventToolUseStart:
			pending = append(pending, &pendingTool{id: evt.ToolUseID, name: evt.ToolName})
			if handler != nil {
				handler(Event{
					Type:      EventToolStart,
					ToolUseID: evt.ToolUseID,
					ToolName:  evt.ToolName,
				})
			}
		case llm.StreamEventToolUseInputDelta:
			if len(pending) == 0 {
				continue
			}
			cur := pending[len(pending)-1]
			if evt.InputDelta != "" {
				cur.raw.WriteString(evt.InputDelta)
			}
			if evt.ToolInput != nil {
				block := finalizeToolUse(cur, evt.ToolInput)
				toolUses = append(toolUses, block)
				pending = pending[:len(pending)-1]
				if handler != nil {
					handler(Event{
						Type:      EventToolStart,
						ToolUseID: block.ID,
						ToolName:  block.Name,
						ToolInput: block.Input,
					})
				}
			}
		case llm.StreamEventMessageEnd:
			usage = evt.Usage
		case llm.StreamEventError:
			streamErr = evt.Err
		}
	}

	for _, p := range pending {
		var input map[string]any
		if p.raw.Len() > 0 {
			_ = json.Unmarshal([]byte(p.raw.String()), &input)
		}
		if input == nil {
			input = map[string]any{}
		}
		toolUses = append(toolUses, finalizeToolUse(p, input))
	}

	if streamErr != nil {
		return streamResult{}, streamErr
	}

	msg := buildAssistantMessage(textBuf.String(), toolUses)
	return streamResult{Message: msg, ToolUses: toolUses, Usage: usage}, nil
}

// finalizeToolUse 根据流式累积结果构造 tool_use ContentBlock。
func finalizeToolUse(p *pendingTool, input map[string]any) llm.ContentBlock {
	return llm.ContentBlock{
		Type:  "tool_use",
		ID:    p.id,
		Name:  p.name,
		Input: input,
	}
}

// buildAssistantMessage 将文本与 tool_use 块合并为 assistant 角色消息。
func buildAssistantMessage(text string, toolUses []llm.ContentBlock) llm.Message {
	content := make([]llm.ContentBlock, 0, 1+len(toolUses))
	if strings.TrimSpace(text) != "" {
		content = append(content, llm.ContentBlock{Type: "text", Text: text})
	}
	content = append(content, toolUses...)
	if len(content) == 0 {
		content = append(content, llm.ContentBlock{Type: "text", Text: ""})
	}
	return llm.Message{Role: llm.RoleAssistant, Content: content}
}

// executeToolCalls 依次执行 tool_use 块，将全部 tool_result 合并为单条 user 消息追加到 messages。
// Anthropic Messages API 要求同一条 assistant 消息中的所有 tool_use 必须在同一条 user 消息中回应。
func (a *Agent) executeToolCalls(ctx context.Context, toolUses []llm.ContentBlock, handler EventHandler) error {
	var results []llm.ContentBlock
	for _, tu := range toolUses {
		if tu.Type != "tool_use" {
			continue
		}
		result, isErr, err := a.dispatchTool(ctx, tu.Name, tu.Input)
		if err != nil {
			return err
		}
		if handler != nil {
			handler(Event{
				Type:       EventToolComplete,
				ToolUseID:  tu.ID,
				ToolName:   tu.Name,
				ToolInput:  tu.Input,
				ToolOutput: result,
				ToolError:  isErr,
			})
		}
		results = append(results, llm.ContentBlock{
			Type:      "tool_result",
			ToolUseID: tu.ID,
			Content:   result,
			IsError:   isErr,
		})
	}
	if len(results) > 0 {
		a.messages = append(a.messages, llm.Message{
			Role:    llm.RoleUser,
			Content: results,
		})
	}
	return nil
}

// dispatchTool 经审批门控后执行单个工具，返回结果文本与是否为错误结果。
func (a *Agent) dispatchTool(ctx context.Context, name string, input map[string]any) (result string, isErr bool, err error) {
	if a.gate.IsDenied(name, input) {
		msg := fmt.Sprintf("Error: denied by approval policy (tool=%s)", name)
		reason := "approval policy"
		if rd, ok := a.gate.(RuleDenier); ok && rd.DeniedByRule(name, input) {
			msg = fmt.Sprintf("Error: denied by permission rule (tool=%s)", name)
			reason = "permission rule"
		}
		a.recordDenial(name, input, reason)
		return msg, true, nil
	}
	if a.gate.ShouldConfirm(name, input) {
		if a.confirm == nil {
			a.recordDenial(name, input, "user denied")
			return "Error: user denied tool execution", true, nil
		}
		ok, confirmErr := a.confirm(name, input)
		if confirmErr != nil {
			return "", false, confirmErr
		}
		if !ok {
			a.recordDenial(name, input, "user denied")
			return "Error: user denied tool execution", true, nil
		}
	}
	out, execErr := a.tools.Execute(ctx, name, input)
	if execErr != nil {
		return execErr.Error(), true, nil
	}
	return out, false, nil
}

// recordDenial 调用 DenialRecorder 持久化拒绝记录。
func (a *Agent) recordDenial(name string, input map[string]any, reason string) {
	if a.onDenial == nil {
		return
	}
	a.onDenial(name, input, reason)
}

// runAgentLoop 在用户消息已追加后，循环 StreamChat → 执行 tool_use 直至无工具调用。
func (a *Agent) runAgentLoop(ctx context.Context, handler EventHandler) error {
	for {
		if err := a.runCompactBeforeTurn(ctx, handler); err != nil {
			return err
		}

		turn, err := a.runStreamTurn(ctx, handler)
		if err != nil {
			if handler != nil {
				handler(Event{Type: EventError, Err: err})
			}
			return err
		}

		a.messages = append(a.messages, turn.Message)
		a.sessionInputTokens += turn.Usage.InputTokens
		a.sessionOutputTokens += turn.Usage.OutputTokens

		if len(turn.ToolUses) == 0 {
			if handler != nil {
				handler(Event{Type: EventTurnComplete})
			}
			return nil
		}
		if err := a.executeToolCalls(ctx, turn.ToolUses, handler); err != nil {
			return err
		}
	}
}

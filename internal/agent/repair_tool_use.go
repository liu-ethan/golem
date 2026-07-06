package agent

import (
	"fmt"

	"github.com/tencent-docs/golem/internal/llm"
)

const missingToolResultMsg = "Error: missing tool result (recovered by golem)"

// RepairToolUsePairing 确保每条含 tool_use 的 assistant 消息后紧跟 user 消息，
// 且包含所有 tool_use id 对应的 tool_result。用于修复历史损坏的会话上下文。
func RepairToolUsePairing(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return messages
	}
	out := make([]llm.Message, 0, len(messages))
	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		ids := assistantToolUseIDs(msg)
		if msg.Role != llm.RoleAssistant || len(ids) == 0 {
			out = append(out, msg)
			continue
		}

		out = append(out, msg)
		if i+1 < len(messages) && messages[i+1].Role == llm.RoleUser {
			next := patchMissingToolResults(messages[i+1], ids)
			out = append(out, next)
			i++
			continue
		}
		out = append(out, syntheticToolResultsMessage(ids))
	}
	return out
}

func assistantToolUseIDs(msg llm.Message) []string {
	var ids []string
	for _, block := range msg.Content {
		if block.Type == "tool_use" && block.ID != "" {
			ids = append(ids, block.ID)
		}
	}
	return ids
}

func toolResultIDSet(msg llm.Message) map[string]bool {
	set := make(map[string]bool)
	for _, block := range msg.Content {
		if block.Type == "tool_result" && block.ToolUseID != "" {
			set[block.ToolUseID] = true
		}
	}
	return set
}

func patchMissingToolResults(msg llm.Message, needed []string) llm.Message {
	present := toolResultIDSet(msg)
	var missing []string
	for _, id := range needed {
		if !present[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return msg
	}
	patched := msg
	for _, id := range missing {
		patched.Content = append(patched.Content, llm.ContentBlock{
			Type:      "tool_result",
			ToolUseID: id,
			Content:   missingToolResultMsg,
			IsError:   true,
		})
	}
	return patched
}

func syntheticToolResultsMessage(ids []string) llm.Message {
	blocks := make([]llm.ContentBlock, len(ids))
	for i, id := range ids {
		blocks[i] = llm.ContentBlock{
			Type:      "tool_result",
			ToolUseID: id,
			Content:   missingToolResultMsg,
			IsError:   true,
		}
	}
	return llm.Message{Role: llm.RoleUser, Content: blocks}
}

func appendUniqueToolUse(toolUses []llm.ContentBlock, block llm.ContentBlock) []llm.ContentBlock {
	if block.ID != "" {
		for _, existing := range toolUses {
			if existing.ID == block.ID {
				return toolUses
			}
		}
	}
	return append(toolUses, block)
}

func toolExecutionError(err error) string {
	return fmt.Sprintf("Error: tool execution failed: %v", err)
}

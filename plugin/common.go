package plugin

import (
	"strings"

	"selective-model-router/core"
)

func findLastAssistant(messages []core.Message) *core.Message {
	var fallback *core.Message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "assistant" {
			continue
		}
		hasToolUse := false
		for _, block := range messages[i].Content {
			if block.Type == "tool_use" {
				hasToolUse = true
				break
			}
		}
		if hasToolUse {
			return &messages[i]
		}
		if fallback == nil {
			fallback = &messages[i]
		}
	}
	return fallback
}

func responseText(resp *core.Response) string {
	if resp == nil {
		return ""
	}
	var b strings.Builder
	for _, msg := range resp.Messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, block := range msg.Content {
			if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(strings.TrimSpace(block.Text))
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func usagePresent(usage core.Usage) bool {
	return usage.InputTokens > 0 || usage.OutputTokens > 0 || usage.CachedInputTokens > 0 || usage.TotalTokens > 0
}

func aggregateUsage(total *core.Usage, usage core.Usage) {
	total.InputTokens += usage.InputTokens
	total.OutputTokens += usage.OutputTokens
	total.CachedInputTokens += usage.CachedInputTokens
	total.TotalTokens = total.InputTokens + total.OutputTokens
}

func applyUsage(resp *core.Response, usage core.Usage, ok bool) *core.Response {
	if resp == nil || !ok {
		return resp
	}
	resp.Usage = usage
	resp.Usage.TotalTokens = resp.Usage.InputTokens + resp.Usage.OutputTokens
	return resp
}

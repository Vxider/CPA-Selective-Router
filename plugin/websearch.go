package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"selective-model-router/core"
)

const webSearchSystemPrompt = "You are a web search assistant. Search the web to answer accurately. Provide concise factual results and cite sources when possible."

type WebSearchWrapper struct {
	Provider  core.Provider
	Model     string
	MaxRounds int
	MaxTokens int
}

type WebSearchQuery struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

func NewWebSearchWrapper(cfg WebSearchConfig) *WebSearchWrapper {
	return &WebSearchWrapper{
		Provider:  cfg.Provider,
		Model:     strings.TrimSpace(cfg.Model),
		MaxRounds: defaultInt(cfg.MaxRounds, 4),
		MaxTokens: defaultInt(cfg.MaxTokens, 4096),
	}
}

func (w *WebSearchWrapper) Wrap(_ context.Context, upstream core.Provider) (core.Provider, error) {
	if upstream == nil {
		return nil, fmt.Errorf("websearch wrapper upstream is nil")
	}
	if w == nil || w.Provider == nil || w.Model == "" {
		return upstream, nil
	}
	return &webSearchOrchestrator{upstream: upstream, delegate: w.Provider, model: w.Model, maxRounds: w.MaxRounds, maxTokens: w.MaxTokens}, nil
}

type webSearchOrchestrator struct {
	upstream  core.Provider
	delegate  core.Provider
	model     string
	maxRounds int
	maxTokens int
}

func (o *webSearchOrchestrator) CreateCore(ctx context.Context, req *core.Request) (*core.Response, error) {
	req = core.CloneRequest(req)
	var total core.Usage
	hasUsage := false

	for round := 0; round < o.maxRounds; round++ {
		resp, err := o.upstream.CreateCore(ctx, core.CloneRequest(req))
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, fmt.Errorf("websearch upstream returned nil response")
		}
		if usagePresent(resp.Usage) {
			hasUsage = true
			aggregateUsage(&total, resp.Usage)
		}
		if resp.StopReason != "tool_use" {
			return applyUsage(resp, total, hasUsage), nil
		}
		last := findLastAssistant(resp.Messages)
		if last == nil {
			return applyUsage(resp, total, hasUsage), nil
		}
		searchUses := webSearchToolUses(last.Content)
		if len(searchUses) == 0 {
			return applyUsage(resp, total, hasUsage), nil
		}
		results := make([]core.ContentBlock, 0, len(searchUses))
		for _, use := range searchUses {
			text := o.execute(ctx, use)
			results = append(results, core.ContentBlock{
				Type:              "tool_result",
				ToolUseID:         use.ToolUseID,
				ToolResultContent: []core.ContentBlock{{Type: "text", Text: text}},
			})
		}
		req.Messages = append(req.Messages, *last)
		req.Messages = append(req.Messages, core.Message{Role: "tool", Content: results})
		req.ToolChoice = &core.ToolChoice{Mode: "auto"}
	}
	return nil, fmt.Errorf("websearch loop exceeded max rounds (%d)", o.maxRounds)
}

func (o *webSearchOrchestrator) execute(ctx context.Context, block core.ContentBlock) string {
	var query WebSearchQuery
	if err := json.Unmarshal(block.ToolInput, &query); err != nil {
		return "Web search error: " + err.Error()
	}
	if strings.TrimSpace(query.Query) == "" {
		return "Web search error: query is required."
	}
	prompt := "Please search the web for: " + query.Query
	if query.MaxResults > 0 {
		prompt += fmt.Sprintf("\nMaximum number of results: %d", query.MaxResults)
	}
	resp, err := o.delegate.CreateCore(ctx, &core.Request{
		Model:     o.model,
		MaxTokens: o.maxTokens,
		System:    []core.ContentBlock{{Type: "text", Text: webSearchSystemPrompt}},
		Messages:  []core.Message{{Role: "user", Content: []core.ContentBlock{{Type: "text", Text: prompt}}}},
		Tools:     WebSearchTools(),
		ToolChoice: &core.ToolChoice{
			Mode: "auto",
		},
	})
	if err != nil {
		return "Web search error: " + err.Error()
	}
	text := responseText(resp)
	if text == "" {
		return "Web search error: delegate returned empty response."
	}
	return "Web search result:\n" + text
}

func webSearchToolUses(blocks []core.ContentBlock) []core.ContentBlock {
	var out []core.ContentBlock
	for _, block := range blocks {
		if block.Type == "tool_use" && (block.ToolName == "web_search" || block.ToolName == "web_search_preview") {
			out = append(out, block)
		}
	}
	return out
}

func defaultInt(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"selective-model-router/core"
)

const visualSystemPrompt = "You are a vision analysis model. Analyze images carefully, state uncertainty, and do not invent visual facts."

type VisualWrapper struct {
	Provider  core.Provider
	Model     string
	MaxRounds int
	MaxTokens int
}

type visualArgs struct {
	Question string `json:"question"`
}

func NewVisualWrapper(cfg VisualConfig) *VisualWrapper {
	return &VisualWrapper{
		Provider:  cfg.Provider,
		Model:     strings.TrimSpace(cfg.Model),
		MaxRounds: defaultInt(cfg.MaxRounds, 4),
		MaxTokens: defaultInt(cfg.MaxTokens, 2048),
	}
}

func (w *VisualWrapper) Wrap(_ context.Context, upstream core.Provider) (core.Provider, error) {
	if upstream == nil {
		return nil, fmt.Errorf("visual wrapper upstream is nil")
	}
	if w == nil || w.Provider == nil || w.Model == "" {
		return upstream, nil
	}
	return &visualOrchestrator{upstream: upstream, vision: w.Provider, model: w.Model, maxRounds: w.MaxRounds, maxTokens: w.MaxTokens}, nil
}

type visualOrchestrator struct {
	upstream  core.Provider
	vision    core.Provider
	model     string
	maxRounds int
	maxTokens int
}

func (o *visualOrchestrator) CreateCore(ctx context.Context, req *core.Request) (*core.Response, error) {
	req = core.CloneRequest(req)
	images := collectImages(req)
	var total core.Usage
	hasUsage := false

	for round := 0; round < o.maxRounds; round++ {
		resp, err := o.upstream.CreateCore(ctx, core.CloneRequest(req))
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, fmt.Errorf("visual upstream returned nil response")
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
		uses := visualToolUses(last.Content)
		if len(uses) == 0 {
			return applyUsage(resp, total, hasUsage), nil
		}
		results := make([]core.ContentBlock, 0, len(uses))
		for _, use := range uses {
			results = append(results, core.ContentBlock{
				Type:              "tool_result",
				ToolUseID:         use.ToolUseID,
				ToolResultContent: []core.ContentBlock{{Type: "text", Text: o.execute(ctx, use, images)}},
			})
		}
		req.Messages = append(req.Messages, *last)
		req.Messages = append(req.Messages, core.Message{Role: "tool", Content: results})
		req.ToolChoice = &core.ToolChoice{Mode: "auto"}
	}
	return nil, fmt.Errorf("visual loop exceeded max rounds (%d)", o.maxRounds)
}

func (o *visualOrchestrator) execute(ctx context.Context, block core.ContentBlock, images []core.ContentBlock) string {
	var args visualArgs
	_ = json.Unmarshal(block.ToolInput, &args)
	question := strings.TrimSpace(args.Question)
	if question == "" {
		question = "Describe the relevant visual content."
	}
	content := []core.ContentBlock{{Type: "text", Text: question}}
	content = append(content, images...)
	resp, err := o.vision.CreateCore(ctx, &core.Request{
		Model:     o.model,
		MaxTokens: o.maxTokens,
		System:    []core.ContentBlock{{Type: "text", Text: visualSystemPrompt}},
		Messages:  []core.Message{{Role: "user", Content: content}},
	})
	if err != nil {
		return "Visual analysis error: " + err.Error()
	}
	text := responseText(resp)
	if text == "" {
		return "Visual analysis error: delegate returned empty response."
	}
	return "Visual analysis result:\n" + text
}

func collectImages(req *core.Request) []core.ContentBlock {
	if req == nil {
		return nil
	}
	var images []core.ContentBlock
	for mi := range req.Messages {
		var rewritten []core.ContentBlock
		for _, block := range req.Messages[mi].Content {
			if block.Type == "image" {
				images = append(images, block)
				rewritten = append(rewritten, core.ContentBlock{Type: "text", Text: "[Image omitted; use visual tools to inspect it.]"})
				continue
			}
			rewritten = append(rewritten, block)
		}
		req.Messages[mi].Content = rewritten
	}
	return images
}

func visualToolUses(blocks []core.ContentBlock) []core.ContentBlock {
	var out []core.ContentBlock
	for _, block := range blocks {
		if block.Type == "tool_use" && (block.ToolName == "visual_brief" || block.ToolName == "visual_qa") {
			out = append(out, block)
		}
	}
	return out
}

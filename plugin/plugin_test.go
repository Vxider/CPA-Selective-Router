package plugin

import (
	"context"
	"encoding/json"
	"testing"

	"selective-model-router/core"
)

type fakeProvider struct {
	calls []*core.Request
	fn    func(*core.Request) *core.Response
}

func (p *fakeProvider) CreateCore(_ context.Context, req *core.Request) (*core.Response, error) {
	p.calls = append(p.calls, req)
	return p.fn(req), nil
}

func TestWebSearchWrapperDelegatesToolUse(t *testing.T) {
	upstream := &fakeProvider{fn: func(req *core.Request) *core.Response {
		if len(req.Messages) == 1 {
			return &core.Response{
				StopReason: "tool_use",
				Messages: []core.Message{{Role: "assistant", Content: []core.ContentBlock{{
					Type:      "tool_use",
					ToolUseID: "call_search",
					ToolName:  "web_search",
					ToolInput: json.RawMessage(`{"query":"cliproxyapi"}`),
				}}}},
			}
		}
		return &core.Response{StopReason: "end_turn", Messages: []core.Message{{Role: "assistant", Content: []core.ContentBlock{{Type: "text", Text: "final"}}}}}
	}}
	delegate := &fakeProvider{fn: func(req *core.Request) *core.Response {
		if len(req.Tools) != 1 || req.Tools[0].Name != "web_search" {
			t.Fatalf("delegate tools = %+v", req.Tools)
		}
		return &core.Response{Messages: []core.Message{{Role: "assistant", Content: []core.ContentBlock{{Type: "text", Text: "search result"}}}}}
	}}
	wrapped, err := NewWebSearchWrapper(WebSearchConfig{Provider: delegate, Model: "gpt", MaxRounds: 3}).Wrap(context.Background(), upstream)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := wrapped.CreateCore(context.Background(), &core.Request{
		Model:    "deepseek",
		Messages: []core.Message{{Role: "user", Content: []core.ContentBlock{{Type: "text", Text: "question"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if responseText(resp) != "final" {
		t.Fatalf("response = %+v", resp)
	}
	if len(upstream.calls) != 2 {
		t.Fatalf("upstream calls = %d", len(upstream.calls))
	}
}

func TestVisualWrapperDelegatesImageQuestion(t *testing.T) {
	upstream := &fakeProvider{fn: func(req *core.Request) *core.Response {
		if len(req.Messages) == 1 {
			if req.Messages[0].Content[1].Type == "image" {
				t.Fatal("image should be stripped from text upstream")
			}
			return &core.Response{
				StopReason: "tool_use",
				Messages: []core.Message{{Role: "assistant", Content: []core.ContentBlock{{
					Type:      "tool_use",
					ToolUseID: "call_visual",
					ToolName:  "visual_qa",
					ToolInput: json.RawMessage(`{"question":"what is in the image?"}`),
				}}}},
			}
		}
		return &core.Response{StopReason: "end_turn", Messages: []core.Message{{Role: "assistant", Content: []core.ContentBlock{{Type: "text", Text: "final visual"}}}}}
	}}
	vision := &fakeProvider{fn: func(req *core.Request) *core.Response {
		if len(req.Messages[0].Content) != 2 || req.Messages[0].Content[1].Type != "image" {
			t.Fatalf("vision content = %+v", req.Messages[0].Content)
		}
		return &core.Response{Messages: []core.Message{{Role: "assistant", Content: []core.ContentBlock{{Type: "text", Text: "a cat"}}}}}
	}}
	wrapped, err := NewVisualWrapper(VisualConfig{Provider: vision, Model: "vision"}).Wrap(context.Background(), upstream)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := wrapped.CreateCore(context.Background(), &core.Request{
		Model: "text",
		Messages: []core.Message{{Role: "user", Content: []core.ContentBlock{
			{Type: "text", Text: "look"},
			{Type: "image", ImageData: "abc", MediaType: "image/png"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if responseText(resp) != "final visual" {
		t.Fatalf("response = %+v", resp)
	}
}

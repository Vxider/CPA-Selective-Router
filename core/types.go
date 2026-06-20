package core

import (
	"context"
	"encoding/json"
)

type Provider interface {
	CreateCore(ctx context.Context, req *Request) (*Response, error)
}

type Request struct {
	Model       string
	Messages    []Message
	System      []ContentBlock
	Tools       []Tool
	ToolChoice  *ToolChoice
	MaxTokens   int
	Temperature *float64
	TopP        *float64
	Stream      bool
	Metadata    map[string]any
	Extensions  map[string]any
}

type Response struct {
	ID         string
	Status     string
	Model      string
	Messages   []Message
	Usage      Usage
	Error      *Error
	StopReason string
	Extensions map[string]any
}

type Message struct {
	Role       string
	Content    []ContentBlock
	Extensions map[string]any
}

type ContentBlock struct {
	Type string

	Text string

	ImageData string
	MediaType string

	ToolUseID string
	ToolName  string
	ToolInput json.RawMessage

	ToolResultContent []ContentBlock

	Extensions map[string]any
}

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Extensions  map[string]any
}

type ToolChoice struct {
	Mode string
	Name string
	Raw  json.RawMessage
}

type Usage struct {
	InputTokens       int
	OutputTokens      int
	TotalTokens       int
	CachedInputTokens int
}

type Error struct {
	Message string
	Type    string
	Code    string
}

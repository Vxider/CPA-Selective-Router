package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"selective-model-router/core"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func (c *Client) CreateCore(ctx context.Context, req *core.Request) (*core.Response, error) {
	if c == nil {
		return nil, fmt.Errorf("openai client is nil")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("openai base_url is required")
	}
	if !strings.HasSuffix(baseURL, "/v1/responses") && !strings.HasSuffix(baseURL, "/responses") {
		baseURL += "/v1/responses"
	}
	body, err := json.Marshal(toResponsesRequest(req))
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai HTTP %d: %s", httpResp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var resp responsesResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}
	return fromResponsesResponse(resp), nil
}

type responsesRequest struct {
	Model           string          `json:"model"`
	Input           json.RawMessage `json:"input"`
	Instructions    string          `json:"instructions,omitempty"`
	MaxOutputTokens int             `json:"max_output_tokens,omitempty"`
	Tools           []tool          `json:"tools,omitempty"`
	ToolChoice      any             `json:"tool_choice,omitempty"`
}

type tool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type responsesResponse struct {
	ID         string       `json:"id"`
	Status     string       `json:"status"`
	Model      string       `json:"model"`
	Output     []outputItem `json:"output"`
	OutputText string       `json:"output_text"`
	Usage      usage        `json:"usage"`
}

type outputItem struct {
	Type      string        `json:"type"`
	Role      string        `json:"role,omitempty"`
	Content   []contentPart `json:"content,omitempty"`
	CallID    string        `json:"call_id,omitempty"`
	Name      string        `json:"name,omitempty"`
	Arguments string        `json:"arguments,omitempty"`
}

type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

func toResponsesRequest(req *core.Request) responsesRequest {
	var input []map[string]any
	for _, msg := range req.Messages {
		input = append(input, messageToInput(msg))
	}
	rawInput, _ := json.Marshal(input)
	return responsesRequest{
		Model:           req.Model,
		Input:           rawInput,
		Instructions:    blocksText(req.System),
		MaxOutputTokens: req.MaxTokens,
		Tools:           toolsToOpenAI(req.Tools),
		ToolChoice:      toolChoice(req.ToolChoice),
	}
}

func messageToInput(msg core.Message) map[string]any {
	if msg.Role == "tool" {
		for _, block := range msg.Content {
			if block.Type == "tool_result" {
				return map[string]any{
					"type":    "function_call_output",
					"call_id": block.ToolUseID,
					"output":  blocksText(block.ToolResultContent),
				}
			}
		}
	}
	var content []map[string]any
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			partType := "input_text"
			if msg.Role == "assistant" {
				partType = "output_text"
			}
			content = append(content, map[string]any{"type": partType, "text": block.Text})
		case "image":
			content = append(content, map[string]any{"type": "input_image", "image_url": imageURL(block)})
		case "tool_use":
			return map[string]any{
				"type":      "function_call",
				"call_id":   block.ToolUseID,
				"name":      block.ToolName,
				"arguments": string(block.ToolInput),
			}
		}
	}
	return map[string]any{"type": "message", "role": msg.Role, "content": content}
}

func toolsToOpenAI(tools []core.Tool) []tool {
	out := make([]tool, 0, len(tools))
	for _, t := range tools {
		if source, _ := t.Extensions["source_type"].(string); source == "web_search" || source == "web_search_preview" {
			out = append(out, tool{Type: source})
			continue
		}
		if t.Name == "web_search" || t.Name == "web_search_preview" {
			out = append(out, tool{Type: t.Name})
			continue
		}
		out = append(out, tool{Type: "function", Name: t.Name, Description: t.Description, Parameters: t.InputSchema})
	}
	return out
}

func toolChoice(choice *core.ToolChoice) any {
	if choice == nil || choice.Mode == "" {
		return nil
	}
	if len(choice.Raw) > 0 {
		var raw any
		if json.Unmarshal(choice.Raw, &raw) == nil {
			return raw
		}
	}
	switch choice.Mode {
	case "auto", "none", "required":
		return choice.Mode
	case "any":
		return "required"
	default:
		if choice.Name != "" {
			return map[string]any{"type": "function", "name": choice.Name}
		}
		return "auto"
	}
}

func fromResponsesResponse(resp responsesResponse) *core.Response {
	out := &core.Response{
		ID:     resp.ID,
		Status: resp.Status,
		Model:  resp.Model,
		Usage: core.Usage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}
	msg := core.Message{Role: "assistant"}
	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if strings.TrimSpace(part.Text) != "" {
					msg.Content = append(msg.Content, core.ContentBlock{Type: "text", Text: part.Text})
				}
			}
		case "function_call":
			msg.Content = append(msg.Content, core.ContentBlock{
				Type:      "tool_use",
				ToolUseID: item.CallID,
				ToolName:  item.Name,
				ToolInput: json.RawMessage(item.Arguments),
			})
			out.StopReason = "tool_use"
		}
	}
	if len(msg.Content) == 0 && strings.TrimSpace(resp.OutputText) != "" {
		msg.Content = append(msg.Content, core.ContentBlock{Type: "text", Text: resp.OutputText})
	}
	if len(msg.Content) > 0 {
		out.Messages = []core.Message{msg}
	}
	return out
}

func blocksText(blocks []core.ContentBlock) string {
	var parts []string
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, strings.TrimSpace(block.Text))
		}
	}
	return strings.Join(parts, "\n")
}

func imageURL(block core.ContentBlock) string {
	if strings.HasPrefix(block.ImageData, "http://") || strings.HasPrefix(block.ImageData, "https://") || strings.HasPrefix(block.ImageData, "data:") {
		return block.ImageData
	}
	mediaType := block.MediaType
	if mediaType == "" {
		mediaType = "image/png"
	}
	return "data:" + mediaType + ";base64," + block.ImageData
}

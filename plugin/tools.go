package plugin

import "selective-model-router/core"

func VisualTools() []core.Tool {
	return []core.Tool{
		{
			Name:        "visual_brief",
			Description: "Analyze the images in the current request and return a concise visual summary.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "visual_qa",
			Description: "Answer a focused question about images in the current request.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{"type": "string"},
				},
				"required": []string{"question"},
			},
		},
	}
}

func WebSearchTools() []core.Tool {
	return []core.Tool{{
		Name:        "web_search",
		Description: "Search the web for up-to-date information.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":       map[string]any{"type": "string"},
				"max_results": map[string]any{"type": "integer"},
			},
			"required": []string{"query"},
		},
		Extensions: map[string]any{"source_type": "web_search"},
	}}
}

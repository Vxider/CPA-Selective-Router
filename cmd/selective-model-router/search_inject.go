package main

import (
	"encoding/json"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func isResponseTransformCandidate(req pluginapi.RequestTransformRequest) bool {
	if len(req.Body) == 0 {
		return false
	}
	if !hasResponsesShape(req.Body) {
		return false
	}
	from := strings.ToLower(strings.TrimSpace(req.FromFormat))
	to := strings.ToLower(strings.TrimSpace(req.ToFormat))
	return isResponseSourceFormat(from) || isResponseSourceFormat(to) || from == "openai" || to == "openai"
}

func injectWebSearchTool(body []byte) ([]byte, bool) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, false
	}
	if hasWebSearchTool(body) || hasWebSearchToolDefinition(root["tools"]) {
		return body, false
	}
	if !hasCurrentSearchIntent(root) {
		return body, false
	}
	tools, _ := root["tools"].([]any)
	root["tools"] = append(tools, map[string]any{"type": "web_search_preview"})
	out, err := json.Marshal(root)
	if err != nil {
		return nil, false
	}
	return out, true
}

func hasWebSearchToolDefinition(value any) bool {
	tools, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range tools {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if isWebSearchType(stringValue(tool["type"])) || isWebSearchType(stringValue(tool["name"])) {
			return true
		}
		if fn, ok := tool["function"].(map[string]any); ok && isWebSearchType(stringValue(fn["name"])) {
			return true
		}
	}
	return false
}

func hasSearchIntent(root map[string]any) bool {
	if boolValue(root["web_search"]) || boolValue(root["search"]) {
		return true
	}
	for _, key := range []string{"tool_choice", "metadata", "client_metadata"} {
		if containsSearchIntent(root[key]) {
			return true
		}
	}
	return containsSearchIntent(root["input"]) || containsSearchIntent(root["instructions"])
}

func hasCurrentSearchIntent(root map[string]any) bool {
	return containsSearchIntent(currentUserInput(root["input"]))
}

func containsSearchIntent(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if isSearchIntentString(key) || containsSearchIntent(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsSearchIntent(child) {
				return true
			}
		}
	case string:
		return isSearchIntentString(typed)
	}
	return false
}

func isSearchIntentString(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if isWebSearchType(lower) {
		return true
	}
	needles := []string{
		"web search",
		"search the web",
		"search online",
		"look up",
		"latest",
		"recent",
		"today",
		"联网",
		"网页搜索",
		"搜索一下",
		"查一下",
		"最新",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func boolValue(value any) bool {
	b, _ := value.(bool)
	return b
}

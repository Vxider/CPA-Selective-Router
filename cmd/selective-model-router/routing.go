package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

func isResponseSourceFormat(source string) bool {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "responses", "openai-response", "openai-responses", "openai_responses":
		return true
	default:
		return false
	}
}

func shouldRouteResponse(req rpcModelRouteRequest, cfg pluginConfig) bool {
	handled, _, _ := routeTargetForRequest(req, cfg)
	return handled
}

func routeTargetForRequest(req rpcModelRouteRequest, cfg pluginConfig) (bool, string, string) {
	if !isResponseRouteCandidate(req) {
		return false, "", ""
	}
	if cfg.RouteCompact && isCompactResponseRequest(req) {
		return true, cfg.RouteProvider, cfg.RouteModel
	}
	if len(req.Body) == 0 {
		return false, "", ""
	}
	if cfg.RouteImageGeneration && cfg.ImageRouteOverride && hasImageGenerationRouteSignal(req.Body) {
		return true, cfg.ImageProvider, cfg.ImageModel
	}
	if cfg.RouteWebSearch && hasWebSearchRouteSignal(req.Body) {
		return true, cfg.RouteProvider, cfg.RouteModel
	}
	if cfg.RouteVision && (hasCurrentImageInput(req.Body) || hasExplicitVisionToolChoice(req.Body) || hasCurrentImagePathMention(req.Body)) {
		return true, cfg.RouteProvider, cfg.RouteModel
	}
	return false, "", ""
}

func isResponseRouteCandidate(req rpcModelRouteRequest) bool {
	if isResponseSourceFormat(req.SourceFormat) || isCompactResponseRequest(req) {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(req.SourceFormat), "openai") {
		return hasResponsesShape(req.Body)
	}
	return false
}

func hasResponsesShape(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	input := gjson.GetBytes(body, "input")
	if !input.Exists() {
		return false
	}
	return gjson.GetBytes(body, "messages").Raw == ""
}

func isCompactResponseRequest(req rpcModelRouteRequest) bool {
	if queryValueEquals(req.Query, "alt", "responses/compact") || queryValueEquals(req.Query, "$alt", "responses/compact") {
		return true
	}
	if metadataStringEquals(req.Metadata, "alt", "responses/compact") {
		return true
	}
	path := metadataString(req.Metadata, "request_path")
	if strings.HasSuffix(strings.TrimSpace(path), "/responses/compact") {
		return true
	}
	return hasCompactProtocolSignal(req.Body)
}

func hasCompactProtocolSignal(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	for _, path := range []string{
		"context_management.type",
		"context_management.action",
		"context_management.mode",
		"context_management.strategy.type",
		"context_management.strategy.action",
		"context_management.strategy.mode",
		"metadata.alt",
		"client_metadata.alt",
	} {
		if isCompactType(gjson.GetBytes(body, path).String()) {
			return true
		}
	}
	for _, item := range gjson.GetBytes(body, "input").Array() {
		if strings.EqualFold(strings.TrimSpace(item.Get("type").String()), "compaction_trigger") {
			return true
		}
	}
	return hasRecentCompactMarker(body, 4)
}

func isCompactType(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return lower == "compact" || lower == "compaction" || lower == "responses/compact"
}

func hasRecentCompactMarker(body []byte, limit int) bool {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	root, ok := payload.(map[string]any)
	if !ok {
		return false
	}
	return containsCheckpointCompactMarker(recentItems(root["input"], limit))
}

func recentItems(value any, limit int) any {
	items, ok := value.([]any)
	if !ok || limit <= 0 || len(items) <= limit {
		return value
	}
	return items[len(items)-limit:]
}

func containsCheckpointCompactMarker(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			if containsCheckpointCompactMarker(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsCheckpointCompactMarker(child) {
				return true
			}
		}
	case string:
		return strings.Contains(typed, "CONTEXT CHECKPOINT COMPACTION")
	}
	return false
}

func hasWebSearchTool(body []byte) bool {
	toolChoiceType := gjson.GetBytes(body, "tool_choice.type").String()
	if isWebSearchType(toolChoiceType) {
		return true
	}
	toolChoiceName := gjson.GetBytes(body, "tool_choice.name").String()
	if toolChoiceName == "" {
		toolChoiceName = gjson.GetBytes(body, "tool_choice.function.name").String()
	}
	if isWebSearchType(toolChoiceName) {
		return true
	}
	for _, item := range gjson.GetBytes(body, "input").Array() {
		if isWebSearchType(item.Get("type").String()) || isWebSearchType(item.Get("name").String()) {
			return true
		}
	}
	return false
}

func hasExplicitWebSearchToolChoice(body []byte) bool {
	toolChoiceType := gjson.GetBytes(body, "tool_choice.type").String()
	if isWebSearchType(toolChoiceType) {
		return true
	}
	toolChoiceName := gjson.GetBytes(body, "tool_choice.name").String()
	if toolChoiceName == "" {
		toolChoiceName = gjson.GetBytes(body, "tool_choice.function.name").String()
	}
	return isWebSearchType(toolChoiceName)
}

func hasWebSearchRouteSignal(body []byte) bool {
	if hasExplicitWebSearchToolChoice(body) {
		return true
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return false
	}
	focus := currentUserInput(root["input"])
	if containsWebSearchInvocation(focus) {
		return true
	}
	return hasWebSearchToolDefinition(root["tools"]) && hasCurrentSearchIntent(root)
}

func isWebSearchType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "web_search", "web_search_preview", "web_search_preview_2025_03_11", "web_search_call":
		return true
	default:
		return false
	}
}

func hasImageGenerationRouteSignal(body []byte) bool {
	if hasExplicitImageGenerationToolChoice(body) {
		return true
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return false
	}
	focus := currentUserInput(root["input"])
	if containsImageGenerationInvocation(focus) {
		return true
	}
	if containsImageGenerationIntent(focus) {
		return true
	}
	return hasImageGenerationToolDefinition(root["tools"]) && containsImageGenerationIntent(root["instructions"])
}

func hasExplicitImageGenerationToolChoice(body []byte) bool {
	toolChoiceType := gjson.GetBytes(body, "tool_choice.type").String()
	if isImageGenerationType(toolChoiceType) {
		return true
	}
	toolChoiceName := gjson.GetBytes(body, "tool_choice.name").String()
	if toolChoiceName == "" {
		toolChoiceName = gjson.GetBytes(body, "tool_choice.function.name").String()
	}
	return isImageGenerationType(toolChoiceName)
}

func hasImageGenerationToolDefinition(value any) bool {
	tools, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range tools {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if isImageGenerationType(stringValue(tool["type"])) || isImageGenerationType(stringValue(tool["name"])) {
			return true
		}
		if fn, ok := tool["function"].(map[string]any); ok && isImageGenerationType(stringValue(fn["name"])) {
			return true
		}
	}
	return false
}

func containsImageGenerationInvocation(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		if isImageGenerationType(stringValue(typed["type"])) || isImageGenerationType(stringValue(typed["name"])) {
			return true
		}
		for _, child := range typed {
			if containsImageGenerationInvocation(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsImageGenerationInvocation(child) {
				return true
			}
		}
	}
	return false
}

func containsImageGenerationIntent(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			if containsImageGenerationIntent(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsImageGenerationIntent(child) {
				return true
			}
		}
	case string:
		return isImageGenerationIntentString(typed)
	}
	return false
}

func isImageGenerationType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image_generation", "image_gen", "imagegen", "generate_image", "gpt-image-2":
		return true
	default:
		return false
	}
}

func isImageGenerationIntentString(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return false
	}
	if containsAny(lower, "报错", "错误", "插件", "tool", "工具", "无法", "不能", "失败", "error", "plugin") {
		return false
	}
	if containsAny(lower, "generate an image", "create an image", "draw an image", "make an image", "image generation", "text-to-image") {
		return true
	}
	if strings.Contains(lower, "生成") && containsAny(lower, "图片", "图像", "一张图", "插画", "海报", "头像") {
		return true
	}
	if strings.Contains(lower, "画") && containsAny(lower, "图片", "图像", "一张", "插画", "海报", "头像") {
		return true
	}
	return false
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func hasVisionTool(body []byte) bool {
	for _, item := range gjson.GetBytes(body, "input").Array() {
		if isVisionToolName(item.Get("name").String()) {
			return true
		}
	}
	toolChoiceName := gjson.GetBytes(body, "tool_choice.name").String()
	if toolChoiceName == "" {
		toolChoiceName = gjson.GetBytes(body, "tool_choice.function.name").String()
	}
	return isVisionToolName(toolChoiceName)
}

func hasExplicitVisionToolChoice(body []byte) bool {
	toolChoiceName := gjson.GetBytes(body, "tool_choice.name").String()
	if toolChoiceName == "" {
		toolChoiceName = gjson.GetBytes(body, "tool_choice.function.name").String()
	}
	return isVisionToolName(toolChoiceName)
}

func isVisionToolName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "view_image", "visual_brief", "visual_qa":
		return true
	default:
		return false
	}
}

func hasImageInput(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	return containsImageValue(payload)
}

func hasCurrentImageInput(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return false
	}
	return containsImageValue(currentUserInput(root["input"]))
}

func hasImagePathMention(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	return containsImagePathString(payload)
}

func hasCurrentImagePathMention(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return false
	}
	return containsImagePathString(currentUserInput(root["input"]))
}

func currentInputFocus(input any, fallbackLimit int) any {
	items, ok := input.([]any)
	if !ok {
		return input
	}
	if len(items) == 0 {
		return items
	}
	start := -1
	for i := len(items) - 1; i >= 0; i-- {
		obj, ok := items[i].(map[string]any)
		if !ok {
			continue
		}
		if isUserMessageObject(obj) {
			start = i
			break
		}
	}
	if start >= 0 {
		return items[start:]
	}
	if fallbackLimit > 0 && len(items) > fallbackLimit {
		return items[len(items)-fallbackLimit:]
	}
	return items
}

func currentUserInput(input any) any {
	items, ok := input.([]any)
	if !ok {
		return input
	}
	for i := len(items) - 1; i >= 0; i-- {
		obj, ok := items[i].(map[string]any)
		if !ok {
			continue
		}
		if isUserMessageObject(obj) {
			return obj
		}
	}
	return nil
}

func isUserMessageObject(obj map[string]any) bool {
	if !strings.EqualFold(strings.TrimSpace(stringValue(obj["role"])), "user") {
		return false
	}
	typ := strings.TrimSpace(stringValue(obj["type"]))
	return typ == "" || strings.EqualFold(typ, "message")
}

func containsWebSearchInvocation(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		if isWebSearchType(stringValue(typed["type"])) || isWebSearchType(stringValue(typed["name"])) {
			return true
		}
		for _, child := range typed {
			if containsWebSearchInvocation(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsWebSearchInvocation(child) {
				return true
			}
		}
	}
	return false
}

func containsImagePathString(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			if containsImagePathString(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsImagePathString(child) {
				return true
			}
		}
	case string:
		return looksLikeImagePath(typed)
	}
	return false
}

func looksLikeImagePath(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".tif", ".tiff"} {
		if strings.Contains(lower, ext) {
			return true
		}
	}
	return false
}

func containsImageValue(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		if isImageObject(typed) {
			return true
		}
		for _, child := range typed {
			if containsImageValue(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsImageValue(child) {
				return true
			}
		}
	case string:
		return strings.HasPrefix(strings.TrimSpace(typed), "data:image/")
	}
	return false
}

func isImageObject(obj map[string]any) bool {
	imageType := strings.ToLower(strings.TrimSpace(stringValue(obj["type"])))
	switch imageType {
	case "input_image", "image", "image_url", "computer_screenshot":
		return true
	}
	if imageSourceLike(obj["image_url"]) || imageSourceLike(obj["source"]) {
		return true
	}
	if mime := strings.ToLower(strings.TrimSpace(stringValue(obj["mime_type"]))); strings.HasPrefix(mime, "image/") {
		return true
	}
	if mime := strings.ToLower(strings.TrimSpace(stringValue(obj["media_type"]))); strings.HasPrefix(mime, "image/") {
		return true
	}
	return false
}

func imageSourceLike(value any) bool {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		return strings.HasPrefix(trimmed, "data:image/") || strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://")
	case map[string]any:
		if imageSourceLike(typed["url"]) {
			return true
		}
		if mime := strings.ToLower(strings.TrimSpace(stringValue(typed["media_type"]))); strings.HasPrefix(mime, "image/") {
			return true
		}
		if mime := strings.ToLower(strings.TrimSpace(stringValue(typed["mime_type"]))); strings.HasPrefix(mime, "image/") {
			return true
		}
		if data := strings.TrimSpace(stringValue(typed["data"])); strings.HasPrefix(data, "data:image/") {
			return true
		}
	}
	return false
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return fmt.Sprint(value)
}

func summarizeBodyShape(body []byte) string {
	if len(body) == 0 {
		return "empty"
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "invalid_json"
	}
	summary := routeShapeSummary{
		TopKeys: mapKeys(payload),
	}
	if root, ok := payload.(map[string]any); ok {
		summary.Model = stringValue(root["model"])
		summary.HasInput = root["input"] != nil
		summary.HasMessages = root["messages"] != nil
		summary.HasTools = root["tools"] != nil
		summary.InputKind = valueKind(root["input"])
		summary.MessageCount = arrayLen(root["messages"])
		summary.ToolTypes = collectObjectTypes(root["tools"])
		summary.ToolChoiceType = objectType(root["tool_choice"])
		summary.InputItemTypes = collectArrayObjectTypes(root["input"])
		summary.RecentInputItemTypes = collectRecentArrayObjectTypes(root["input"], 8)
		summary.ContentTypes = collectNamedTypes(root["input"], "content")
		summary.OutputTypes = collectNamedTypes(root["input"], "output")
		summary.CompactMarkers = collectCompactMarkers(body)
		summary.RecentCompactMarkers = collectCompactMarkersFromValue(recentItems(root["input"], 4))
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		return "summary_error"
	}
	return string(raw)
}

type routeShapeSummary struct {
	TopKeys              []string `json:"top_keys,omitempty"`
	Model                string   `json:"model,omitempty"`
	HasInput             bool     `json:"has_input,omitempty"`
	HasMessages          bool     `json:"has_messages,omitempty"`
	HasTools             bool     `json:"has_tools,omitempty"`
	InputKind            string   `json:"input_kind,omitempty"`
	MessageCount         int      `json:"message_count,omitempty"`
	ToolTypes            []string `json:"tool_types,omitempty"`
	ToolChoiceType       string   `json:"tool_choice_type,omitempty"`
	InputItemTypes       []string `json:"input_item_types,omitempty"`
	RecentInputItemTypes []string `json:"recent_input_item_types,omitempty"`
	ContentTypes         []string `json:"content_types,omitempty"`
	OutputTypes          []string `json:"output_types,omitempty"`
	CompactMarkers       []string `json:"compact_markers,omitempty"`
	RecentCompactMarkers []string `json:"recent_compact_markers,omitempty"`
}

func mapKeys(value any) []string {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	return keys
}

func valueKind(value any) string {
	switch value.(type) {
	case nil:
		return ""
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	default:
		return "unknown"
	}
}

func arrayLen(value any) int {
	items, ok := value.([]any)
	if !ok {
		return 0
	}
	return len(items)
}

func objectType(value any) string {
	obj, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return strings.TrimSpace(stringValue(obj["type"]))
}

func collectObjectTypes(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		typ := objectType(item)
		if typ == "" || seen[typ] {
			continue
		}
		seen[typ] = true
		out = append(out, typ)
	}
	return out
}

func collectArrayObjectTypes(value any) []string {
	return collectObjectTypes(value)
}

func collectRecentArrayObjectTypes(value any, limit int) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}
	return collectObjectTypes(items)
}

func collectCompactMarkers(body []byte) []string {
	if len(body) == 0 {
		return nil
	}
	return collectCompactMarkersFromString(string(body))
}

func collectCompactMarkersFromValue(value any) []string {
	var texts []string
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for _, child := range typed {
				visit(child)
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		case string:
			texts = append(texts, typed)
		}
	}
	visit(value)
	return collectCompactMarkersFromString(strings.Join(texts, "\n"))
}

func collectCompactMarkersFromString(text string) []string {
	var markers []string
	for _, marker := range []string{
		"CONTEXT CHECKPOINT COMPACTION",
		"CONTEXT CHECKPOINT COMPACTION SUMMARY",
		"compaction_trigger",
		"responses/compact",
		"context_management",
	} {
		if strings.Contains(text, marker) {
			markers = append(markers, marker)
		}
	}
	return markers
}

func collectNamedTypes(value any, field string) []string {
	seen := map[string]bool{}
	var out []string
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case []any:
			for _, item := range typed {
				visit(item)
			}
		case map[string]any:
			if child, ok := typed[field]; ok {
				for _, typ := range collectObjectTypes(child) {
					if !seen[typ] {
						seen[typ] = true
						out = append(out, typ)
					}
				}
			}
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(value)
	return out
}

func queryValueEquals(values map[string][]string, key, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	for _, value := range values[key] {
		if strings.ToLower(strings.TrimSpace(value)) == want {
			return true
		}
	}
	return false
}

func metadataStringEquals(metadata map[string]any, key, want string) bool {
	return strings.EqualFold(metadataString(metadata, key), want)
}

func metadataString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	raw, ok := metadata[key]
	if !ok || raw == nil {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case []byte:
		return strings.TrimSpace(string(value))
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

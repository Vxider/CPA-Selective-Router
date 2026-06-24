package main

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
)

// rectifierState tracks tool calls seen in the current SSE stream and
// ensures every tool call has a matching output before the stream ends.
// It is per-request: created on the header-init chunk, used across chunks,
// and discarded after response.completed.
type rectifierState struct {
	mu sync.Mutex
	// pendingCalls maps call_id -> call_type (function_call, tool_search_call, custom_tool_call).
	pendingCalls map[string]string
	// completedCalls tracks call_ids that have received a matching output.
	completedCalls map[string]bool
}

func newRectifierState() *rectifierState {
	return &rectifierState{
		pendingCalls:   make(map[string]string),
		completedCalls: make(map[string]bool),
	}
}

// toolCallTypes that need a matching output.
var toolCallTypes = map[string]bool{
	"function_call":    true,
	"tool_search_call": true,
	"custom_tool_call": true,
}

// toolOutputTypes maps output type -> call type.
var toolOutputTypes = map[string]string{
	"function_call_output":    "function_call",
	"tool_search_output":      "tool_search_call",
	"custom_tool_call_output": "custom_tool_call",
}

// observeChunk inspects a single SSE chunk body (JSON) and records
// tool call / output pairs. It returns a list of synthetic output
// events to inject for any orphaned tool calls detected so far.
// This is called only when the chunk contains a response.output_item.done
// or response.completed event.
func (r *rectifierState) observeChunk(body []byte) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Parse the SSE data field from the chunk.
	// CLIProxyAPI delivers raw SSE lines; the chunk body is the JSON after "data: ".
	data := body
	if len(data) == 0 {
		return nil
	}

	eventType := gjson.GetBytes(data, "type").String()
	if eventType == "" {
		return nil
	}

	switch eventType {
	case "response.output_item.done":
		item := gjson.GetBytes(data, "item")
		if !item.Exists() {
			return nil
		}
		itemType := item.Get("type").String()
		callID := item.Get("call_id").String()

		if _, isCall := toolCallTypes[itemType]; isCall && callID != "" {
			r.pendingCalls[callID] = itemType
		}
		if _, isOutput := toolOutputTypes[itemType]; isOutput && callID != "" {
			r.completedCalls[callID] = true
		}

	case "response.completed":
		return r.synthesizeOrphanOutputs()
	}

	return nil
}

// synthesizeOrphanOutputs returns SSE event JSON strings for any tool
// calls that have not received a matching output.
func (r *rectifierState) synthesizeOrphanOutputs() []string {
	var injections []string
	for callID, callType := range r.pendingCalls {
		if r.completedCalls[callID] {
			continue
		}
		injection := r.synthesizeOutput(callID, callType)
		if injection != "" {
			injections = append(injections, injection)
		}
	}
	return injections
}

func (r *rectifierState) synthesizeOutput(callID, callType string) string {
	switch callType {
	case "function_call":
		return synthesizeFunctionCallOutput(callID)
	case "tool_search_call":
		return synthesizeToolSearchOutput(callID)
	case "custom_tool_call":
		return synthesizeCustomToolCallOutput(callID)
	default:
		return ""
	}
}

func synthesizeFunctionCallOutput(callID string) string {
	item := map[string]any{
		"type":    "function_call_output",
		"call_id": callID,
		"output":  "Tool call failed: no output was produced by the runtime. This output was synthesized by the transcript rectifier.",
	}
	event := map[string]any{
		"type": "response.output_item.done",
		"item": item,
	}
	raw, _ := json.Marshal(event)
	return string(raw)
}

func synthesizeToolSearchOutput(callID string) string {
	item := map[string]any{
		"type":      "tool_search_output",
		"call_id":   callID,
		"status":    "completed",
		"execution": "client",
		"tools":     []any{},
	}
	event := map[string]any{
		"type": "response.output_item.done",
		"item": item,
	}
	raw, _ := json.Marshal(event)
	return string(raw)
}

func synthesizeCustomToolCallOutput(callID string) string {
	item := map[string]any{
		"type":    "custom_tool_call_output",
		"call_id": callID,
		"output":  "Tool call failed: no output was produced by the runtime. This output was synthesized by the transcript rectifier.",
	}
	event := map[string]any{
		"type": "response.output_item.done",
		"item": item,
	}
	raw, _ := json.Marshal(event)
	return string(raw)
}

// rectifierInterceptor is the top-level per-request state machine.
// It is created on the header-init chunk and used for the rest of the request.
var (
	rectifierMu     sync.Mutex
	rectifierStates = make(map[string]*rectifierState) // keyed by request metadata id
)

func getOrCreateRectifier(requestKey string) *rectifierState {
	rectifierMu.Lock()
	defer rectifierMu.Unlock()
	if s, ok := rectifierStates[requestKey]; ok {
		return s
	}
	s := newRectifierState()
	rectifierStates[requestKey] = s
	return s
}

func cleanupRectifier(requestKey string) {
	rectifierMu.Lock()
	defer rectifierMu.Unlock()
	delete(rectifierStates, requestKey)
}

// interceptStreamChunk implements the stream_chunk_interceptor capability.
// It inspects each SSE chunk, tracks tool calls, and on response.completed
// injects synthetic outputs for any orphaned tool calls.
func interceptStreamChunk(raw []byte, chunkIndex int, metadata map[string]any) ([]byte, []string, bool) {
	if len(raw) == 0 {
		return raw, nil, false
	}

	// Derive a per-request key from metadata.
	requestKey := metadataKey(metadata)

	// Header-init chunk: initialize state.

	if requestKey == "" {
		return raw, nil, false
	}
	if chunkIndex == -1 {
		getOrCreateRectifier(requestKey)
		return raw, nil, false
	}

	state := getOrCreateRectifier(requestKey)
	if state == nil {
		return raw, nil, false
	}

	// Only process chunks that look like SSE data events.
	if !isSSEDataEvent(raw) {
		return raw, nil, false
	}

	// Parse the JSON payload after "data: ".
	jsonPayload := extractSSEDataPayload(raw)
	if len(jsonPayload) == 0 {
		return raw, nil, false
	}

	eventType := gjson.GetBytes(jsonPayload, "type").String()

	// Track tool calls and outputs.
	if eventType == "response.output_item.done" {
		item := gjson.GetBytes(jsonPayload, "item")
		if item.Exists() {
			itemType := item.Get("type").String()
			callID := item.Get("call_id").String()

			state.mu.Lock()
			if _, isCall := toolCallTypes[itemType]; isCall && callID != "" {
				state.pendingCalls[callID] = itemType
			}
			if _, isOutput := toolOutputTypes[itemType]; isOutput && callID != "" {
				state.completedCalls[callID] = true
			}
			state.mu.Unlock()
		}
	}

	// On response.completed, synthesize missing outputs.
	if eventType == "response.completed" {
		injections := state.synthesizeOrphanOutputs()
		cleanupRectifier(requestKey)
		if len(injections) > 0 {
			// Return the original chunk unchanged, plus synthetic chunks to emit.
			return raw, injections, false
		}
		return raw, nil, false
	}

	return raw, nil, false
}

func metadataKey(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	// Use request_id or a combination of available fields.
	for _, key := range []string{"request_id", "stream_id", "conversation_id"} {
		if v, ok := metadata[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	// No reliable key found; return empty to disable rectifier for this request.
	return ""
}

func isSSEDataEvent(raw []byte) bool {
	return len(raw) > 5 && (raw[0] == 'd' && raw[1] == 'a' && raw[2] == 't' && raw[3] == 'a')
}

func extractSSEDataPayload(raw []byte) []byte {
	s := string(raw)
	if !strings.HasPrefix(s, "data: ") && !strings.HasPrefix(s, "data:") {
		return nil
	}
	s = strings.TrimPrefix(s, "data: ")
	s = strings.TrimPrefix(s, "data:")
	s = strings.TrimRight(s, "\n\r")
	if s == "[DONE]" {
		return nil
	}
	return []byte(s)
}

// injectSyntheticChunks appends synthetic SSE data events to the stream
// by modifying the current chunk body to include additional events.
// Returns the modified body.
func injectSyntheticChunks(originalBody []byte, injections []string) []byte {
	if len(injections) == 0 {
		return originalBody
	}
	// Build the additional SSE lines.
	var sb strings.Builder
	for _, injection := range injections {
		sb.WriteString("data: ")
		sb.WriteString(injection)
		sb.WriteString("\n\n")
	}
	return append([]byte(sb.String()), originalBody...)
}

// patchChunkBody applies the rectifier to a single chunk body.
// It returns the (possibly modified) body and whether to drop the chunk.
func patchChunkBody(body []byte, chunkIndex int, metadata map[string]any) ([]byte, int, bool) {
	modifiedBody, injections, drop := interceptStreamChunk(body, chunkIndex, metadata)
	if drop {
		return nil, 0, true
	}
	if len(injections) > 0 {
		return injectSyntheticChunks(modifiedBody, injections), len(injections), false
	}
	return modifiedBody, 0, false
}

// ensureNoOrphanCallsInNonStreamResponse handles the non-streaming case

// ensureNoOrphanCallsInNonStreamResponse handles the non-streaming case
// where the full response body is available as a single JSON object.
func ensureNoOrphanCallsInNonStreamResponse(body []byte) ([]byte, int) {
	if len(body) == 0 {
		return body, 0
	}

	// Check if this is a Responses API response with output items.
	output := gjson.GetBytes(body, "output")
	if !output.Exists() || !output.IsArray() {
		return body, 0
	}

	pendingCalls := make(map[string]string)
	completedCalls := make(map[string]bool)

	for _, item := range output.Array() {
		itemType := item.Get("type").String()
		callID := item.Get("call_id").String()
		if _, isCall := toolCallTypes[itemType]; isCall && callID != "" {
			pendingCalls[callID] = itemType
		}
		if _, isOutput := toolOutputTypes[itemType]; isOutput && callID != "" {
			completedCalls[callID] = true
		}
	}

	// Check for orphaned calls.
	var orphans []string
	for callID, callType := range pendingCalls {
		if !completedCalls[callID] {
			orphans = append(orphans, callID)
			_ = callType // used below
		}
	}
	if len(orphans) == 0 {
		return body, 0
	}

	// Inject synthetic outputs into the response JSON.
	// Use map[string]any to avoid sjson array append issues.
	var respMap map[string]any
	if err := json.Unmarshal(body, &respMap); err != nil {
		return body, 0
	}
	outputAny, ok := respMap["output"]
	if !ok {
		return body, 0
	}
	outputSlice, ok := outputAny.([]any)
	if !ok {
		return body, 0
	}

	for _, callID := range orphans {
		callType := pendingCalls[callID]
		var syntheticItem map[string]any
		switch callType {
		case "function_call":
			syntheticItem = map[string]any{
				"type":    "function_call_output",
				"call_id": callID,
				"output":  "Tool call failed: no output was produced. Synthesized by transcript rectifier.",
			}
		case "tool_search_call":
			syntheticItem = map[string]any{
				"type":      "tool_search_output",
				"call_id":   callID,
				"status":    "completed",
				"execution": "client",
				"tools":     []any{},
			}
		case "custom_tool_call":
			syntheticItem = map[string]any{
				"type":    "custom_tool_call_output",
				"call_id": callID,
				"output":  "Tool call failed: no output was produced. Synthesized by transcript rectifier.",
			}
		default:
			continue
		}
		outputSlice = append(outputSlice, syntheticItem)
	}
	respMap["output"] = outputSlice
	modified, err := json.Marshal(respMap)
	if err != nil {
		return body, 0
	}
	return modified, len(orphans)
}

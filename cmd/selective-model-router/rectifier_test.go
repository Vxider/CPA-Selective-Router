package main

import (
	"encoding/json"
	"testing"
)

func TestRectifierDetectsOrphanedFunctionCall(t *testing.T) {
	state := newRectifierState()

	// Simulate a function_call output_item.done.
	callEvent := map[string]any{
		"type": "response.output_item.done",
		"item": map[string]any{
			"type":    "function_call",
			"call_id": "call-1",
		},
	}
	callJSON, _ := json.Marshal(callEvent)
	injections := state.observeChunk(callJSON)
	if len(injections) != 0 {
		t.Fatalf("expected no injections before completed, got %d", len(injections))
	}

	// Simulate response.completed without a matching output.
	completedEvent := map[string]any{
		"type": "response.completed",
	}
	completedJSON, _ := json.Marshal(completedEvent)
	injections = state.observeChunk(completedJSON)
	if len(injections) != 1 {
		t.Fatalf("expected 1 injection, got %d", len(injections))
	}

	var injected map[string]any
	if err := json.Unmarshal([]byte(injections[0]), &injected); err != nil {
		t.Fatalf("failed to parse injection: %v", err)
	}
	item := injected["item"].(map[string]any)
	if item["type"] != "function_call_output" {
		t.Errorf("expected function_call_output, got %v", item["type"])
	}
	if item["call_id"] != "call-1" {
		t.Errorf("expected call_id=call-1, got %v", item["call_id"])
	}
}

func TestRectifierNoInjectionWhenOutputPresent(t *testing.T) {
	state := newRectifierState()

	// Simulate function_call.
	callEvent := map[string]any{
		"type": "response.output_item.done",
		"item": map[string]any{
			"type":    "function_call",
			"call_id": "call-2",
		},
	}
	callJSON, _ := json.Marshal(callEvent)
	state.observeChunk(callJSON)

	// Simulate matching function_call_output.
	outputEvent := map[string]any{
		"type": "response.output_item.done",
		"item": map[string]any{
			"type":    "function_call_output",
			"call_id": "call-2",
		},
	}
	outputJSON, _ := json.Marshal(outputEvent)
	state.observeChunk(outputJSON)

	// response.completed should produce no injections.
	completedEvent := map[string]any{"type": "response.completed"}
	completedJSON, _ := json.Marshal(completedEvent)
	injections := state.observeChunk(completedJSON)
	if len(injections) != 0 {
		t.Fatalf("expected 0 injections, got %d", len(injections))
	}
}

func TestRectifierToolSearchCall(t *testing.T) {
	state := newRectifierState()

	callEvent := map[string]any{
		"type": "response.output_item.done",
		"item": map[string]any{
			"type":    "tool_search_call",
			"call_id": "search-1",
		},
	}
	callJSON, _ := json.Marshal(callEvent)
	state.observeChunk(callJSON)

	completedEvent := map[string]any{"type": "response.completed"}
	completedJSON, _ := json.Marshal(completedEvent)
	injections := state.observeChunk(completedJSON)
	if len(injections) != 1 {
		t.Fatalf("expected 1 injection, got %d", len(injections))
	}

	var injected map[string]any
	if err := json.Unmarshal([]byte(injections[0]), &injected); err != nil {
		t.Fatalf("failed to parse injection: %v", err)
	}
	item := injected["item"].(map[string]any)
	if item["type"] != "tool_search_output" {
		t.Errorf("expected tool_search_output, got %v", item["type"])
	}
	if item["call_id"] != "search-1" {
		t.Errorf("expected call_id=search-1, got %v", item["call_id"])
	}
	tools, ok := item["tools"].([]any)
	if !ok || len(tools) != 0 {
		t.Errorf("expected empty tools array, got %v", item["tools"])
	}
}

func TestRectifierCustomToolCall(t *testing.T) {
	state := newRectifierState()

	callEvent := map[string]any{
		"type": "response.output_item.done",
		"item": map[string]any{
			"type":    "custom_tool_call",
			"call_id": "custom-1",
		},
	}
	callJSON, _ := json.Marshal(callEvent)
	state.observeChunk(callJSON)

	completedEvent := map[string]any{"type": "response.completed"}
	completedJSON, _ := json.Marshal(completedEvent)
	injections := state.observeChunk(completedJSON)
	if len(injections) != 1 {
		t.Fatalf("expected 1 injection, got %d", len(injections))
	}

	var injected map[string]any
	if err := json.Unmarshal([]byte(injections[0]), &injected); err != nil {
		t.Fatalf("failed to parse injection: %v", err)
	}
	item := injected["item"].(map[string]any)
	if item["type"] != "custom_tool_call_output" {
		t.Errorf("expected custom_tool_call_output, got %v", item["type"])
	}
}

func TestRectifierMultipleOrphanedCalls(t *testing.T) {
	state := newRectifierState()

	for _, id := range []string{"call-a", "call-b", "call-c"} {
		callEvent := map[string]any{
			"type": "response.output_item.done",
			"item": map[string]any{
				"type":    "function_call",
				"call_id": id,
			},
		}
		callJSON, _ := json.Marshal(callEvent)
		state.observeChunk(callJSON)
	}

	// Only call-b gets an output.
	outputEvent := map[string]any{
		"type": "response.output_item.done",
		"item": map[string]any{
			"type":    "function_call_output",
			"call_id": "call-b",
		},
	}
	outputJSON, _ := json.Marshal(outputEvent)
	state.observeChunk(outputJSON)

	completedEvent := map[string]any{"type": "response.completed"}
	completedJSON, _ := json.Marshal(completedEvent)
	injections := state.observeChunk(completedJSON)
	if len(injections) != 2 {
		t.Fatalf("expected 2 injections (call-a, call-c), got %d", len(injections))
	}
}

func TestNonStreamResponseRectifier(t *testing.T) {
	// Build a response with an orphaned function_call.
	response := map[string]any{
		"id":     "resp-1",
		"object": "response",
		"output": []any{
			map[string]any{
				"type":    "function_call",
				"call_id": "fc-1",
				"name":    "some_tool",
			},
			map[string]any{
				"type": "message",
				"role": "assistant",
			},
		},
	}
	body, _ := json.Marshal(response)

	modified, _ := ensureNoOrphanCallsInNonStreamResponse(body)

	// Verify the output array now has 3 items (original 2 + 1 synthetic).
	outputRaw := string(modified)
	if len(outputRaw) <= len(string(body)) {
		t.Error("expected modified body to be larger than original")
	}
	// Quick check: the synthetic output should contain the call_id.
	var parsed map[string]any
	if err := json.Unmarshal(modified, &parsed); err != nil {
		t.Fatalf("failed to parse modified: %v", err)
	}
	output := parsed["output"].([]any)
	if len(output) != 3 {
		t.Fatalf("expected 3 output items, got %d", len(output))
	}
	synthetic := output[2].(map[string]any)
	if synthetic["type"] != "function_call_output" {
		t.Errorf("expected function_call_output, got %v", synthetic["type"])
	}
	if synthetic["call_id"] != "fc-1" {
		t.Errorf("expected call_id=fc-1, got %v", synthetic["call_id"])
	}
}

func TestPatchChunkBodySSEFormat(t *testing.T) {
	// Initialize state via header-init.
	metadata := map[string]any{"request_id": "test-req"}
	patched, _, drop := patchChunkBody(nil, -1, metadata)
	if drop {
		t.Error("header-init should not be dropped")
	}
	_ = patched

	// Simulate a function_call in SSE format.
	sseChunk := []byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"call_id\":\"sse-1\"}}\n\n")
	patched, _, drop = patchChunkBody(sseChunk, 0, metadata)
	if drop {
		t.Error("function_call chunk should not be dropped")
	}

	// Simulate response.completed.
	completedChunk := []byte("data: {\"type\":\"response.completed\"}\n\n")
	patched, _, drop = patchChunkBody(completedChunk, 1, metadata)
	if drop {
		t.Error("completed chunk should not be dropped")
	}
	// The patched body should contain the synthetic output appended.
	if len(patched) <= len(completedChunk) {
		t.Error("expected patched completed chunk to include synthetic output")
	}
}

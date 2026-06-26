package main

import (
	"encoding/json"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
	"github.com/tidwall/gjson"
)

func TestNormalizeRequestInjectsWebSearchToolForSearchIntent(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:        true,
		RouteWebSearch: true,
		Models:         []string{"gpt-5.4-mini"},
	})

	body := normalizeForTest(t, pluginapi.RequestTransformRequest{
		FromFormat: "openai-response",
		ToFormat:   "openai-response",
		Model:      "gpt-5.4-mini",
		Body:       []byte(`{"model":"gpt-5.4-mini","input":"查一下今天 OpenAI 有什么最新消息"}`),
	})

	if gjson.GetBytes(body, "tools").Exists() {
		t.Fatalf("tools should not be injected based on user text; body=%s", body)
	}
}

func TestNormalizeRequestDoesNotInjectForOrdinaryText(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:        true,
		RouteWebSearch: true,
		Models:         []string{"gpt-5.4-mini"},
	})

	body := normalizeForTest(t, pluginapi.RequestTransformRequest{
		FromFormat: "openai-response",
		ToFormat:   "openai-response",
		Model:      "gpt-5.4-mini",
		Body:       []byte(`{"model":"gpt-5.4-mini","input":"写一个排序函数"}`),
	})

	if gjson.GetBytes(body, "tools").Exists() {
		t.Fatalf("tools exists for ordinary text; body=%s", body)
	}
}

func TestInjectedWebSearchToolRoutesWithSearchIntent(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:        true,
		RouteProvider:  "codex",
		RouteModel:     "gpt-5.5",
		RouteWebSearch: true,
		Models:         []string{"gpt-5.4-mini"},
	})

	body := normalizeForTest(t, pluginapi.RequestTransformRequest{
		FromFormat: "openai-response",
		ToFormat:   "openai-response",
		Model:      "gpt-5.4-mini",
		Body:       []byte(`{"model":"gpt-5.4-mini","input":"look up the latest Go release"}`),
	})
	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body:               body,
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for user-text intent without explicit tool choice; body=%s", body)
	}
	if resp.TargetModel != "gpt-5.5" {
		t.Fatalf("TargetModel = %q, want gpt-5.5", resp.TargetModel)
	}
}

func TestNormalizeRequestInjectsImageGenerationToolForImageIntent(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:              true,
		RouteImageGeneration: true,
		ImageToolModel:       "gpt-image-2",
		Models:               []string{"gpt-5.4-mini"},
	})

	body := normalizeForTest(t, pluginapi.RequestTransformRequest{
		FromFormat: "openai-response",
		ToFormat:   "openai-response",
		Model:      "gpt-5.4-mini",
		Body:       []byte(`{"model":"gpt-5.4-mini","input":"生成一张红色小猫图片"}`),
	})

	if got := gjson.GetBytes(body, "tools.0.type").String(); got != "image_generation" {
		t.Fatalf("tools.0.type = %q, want image_generation; body=%s", got, body)
	}
	if got := gjson.GetBytes(body, "tools.0.model").String(); got != "gpt-image-2" {
		t.Fatalf("tools.0.model = %q, want gpt-image-2; body=%s", got, body)
	}
}

func TestNormalizeRequestKeepsImageGenerationToolForFollowup(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:              true,
		RouteImageGeneration: true,
		ImageToolModel:       "gpt-image-2",
		Models:               []string{"gpt-5.4-mini"},
	})

	body := normalizeForTest(t, pluginapi.RequestTransformRequest{
		FromFormat: "openai-response",
		ToFormat:   "openai-response",
		Model:      "gpt-5.4-mini",
		Body: []byte(`{
			"model":"gpt-5.4-mini",
			"input":[
				{"type":"message","role":"user","content":[{"type":"input_text","text":"生成一张红色小猫图片"}]},
				{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]},
				{"type":"message","role":"user","content":[{"type":"input_text","text":"再来一张，换成蓝色"}]}
			]
		}`),
	})

	if got := gjson.GetBytes(body, "tools.0.type").String(); got != "image_generation" {
		t.Fatalf("tools.0.type = %q, want image_generation; body=%s", got, body)
	}
}

func TestNormalizeRequestDoesNotInjectImageGenerationToolForTroubleshooting(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:              true,
		RouteImageGeneration: true,
		ImageToolModel:       "gpt-image-2",
		Models:               []string{"gpt-5.4-mini"},
	})

	body := normalizeForTest(t, pluginapi.RequestTransformRequest{
		FromFormat: "openai-response",
		ToFormat:   "openai-response",
		Model:      "gpt-5.4-mini",
		Body:       []byte(`{"model":"gpt-5.4-mini","input":"生成图片报错，插件能不能解决 image_gen 工具不可用"}`),
	})

	if gjson.GetBytes(body, "tools").Exists() {
		t.Fatalf("tools exists for troubleshooting text; body=%s", body)
	}
}

func normalizeForTest(t *testing.T, req pluginapi.RequestTransformRequest) []byte {
	t.Helper()
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	out, err := normalizeRequest(raw)
	if err != nil {
		t.Fatal(err)
	}
	var env envelope
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("envelope error: %#v", env.Error)
	}
	var resp pluginapi.PayloadResponse
	if err := json.Unmarshal(env.Result, &resp); err != nil {
		t.Fatal(err)
	}
	return resp.Body
}

func TestNormalizeRequestInjectsImageGenerationToolWithRouteProvider(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:              true,
		RouteImageGeneration: true,
		ImageRouteProvider:   "image-capable-codex",
		ImageToolModel:       "gpt-image-2",
		Models:               []string{"gpt-5.4-mini"},
	})

	body := normalizeForTest(t, pluginapi.RequestTransformRequest{
		FromFormat: "openai-response",
		ToFormat:   "openai-response",
		Model:      "gpt-5.4-mini",
		Body:       []byte(`{"model":"gpt-5.4-mini","input":"生成一张红色小猫图片"}`),
	})

	if got := gjson.GetBytes(body, "tools.0.type").String(); got != "image_generation" {
		t.Fatalf("tools.0.type = %q, want image_generation; body=%s", got, body)
	}
	if got := gjson.GetBytes(body, "tools.0.model").String(); got != "gpt-image-2" {
		t.Fatalf("tools.0.model = %q, want gpt-image-2; body=%s", got, body)
	}
}

func TestNormalizeRequestInjectsImageGenerationToolWithoutRouteProvider(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:              true,
		RouteImageGeneration: true,
		ImageToolModel:       "gpt-image-2",
		Models:               []string{"gpt-5.4-mini"},
	})

	body := normalizeForTest(t, pluginapi.RequestTransformRequest{
		FromFormat: "openai-response",
		ToFormat:   "openai-response",
		Model:      "gpt-5.4-mini",
		Body:       []byte(`{"model":"gpt-5.4-mini","input":"生成一张红色小猫图片"}`),
	})

	if got := gjson.GetBytes(body, "tools.0.type").String(); got != "image_generation" {
		t.Fatalf("tools.0.type = %q, want image_generation; body=%s", got, body)
	}
	if got := gjson.GetBytes(body, "tools.0.model").String(); got != "gpt-image-2" {
		t.Fatalf("tools.0.model = %q, want gpt-image-2; body=%s", got, body)
	}
}

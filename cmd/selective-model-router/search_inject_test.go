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

	if got := gjson.GetBytes(body, "tools.0.type").String(); got != "web_search_preview" {
		t.Fatalf("tools.0.type = %q, want web_search_preview; body=%s", got, body)
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

	if !resp.Handled {
		t.Fatalf("Handled = false, want true; body=%s", body)
	}
	if resp.TargetModel != "gpt-5.5" {
		t.Fatalf("TargetModel = %q, want gpt-5.5", resp.TargetModel)
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

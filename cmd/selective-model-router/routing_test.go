package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestRouteModelOpenAIResponsesAutoReviewGuardianHeader(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:         true,
		RouteProvider:   "codex",
		RouteModel:      "gpt-5.5",
		RouteAutoReview: true,
		Models:          []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Headers:            http.Header{"X-Openai-Subagent": []string{"guardian"}},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"review this approval"}]}]}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetKind != pluginapi.ModelRouteTargetProvider || resp.Target != "codex" || resp.TargetModel != "gpt-5.5" {
		t.Fatalf("route = %#v, want provider codex/gpt-5.5", resp)
	}
}

func TestRouteModelOpenAIResponsesAutoReviewModelFallback(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:         true,
		RouteProvider:   "codex",
		RouteModel:      "gpt-5.5",
		RouteAutoReview: true,
		Models:          []string{"codex-auto-review"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "codex-auto-review",
			AvailableProviders: []string{"codex"},
			Body:               []byte(`{"model":"codex-auto-review","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"assess approval request"}]}]}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetKind != pluginapi.ModelRouteTargetProvider || resp.Target != "codex" || resp.TargetModel != "gpt-5.5" {
		t.Fatalf("route = %#v, want provider codex/gpt-5.5", resp)
	}
}

func TestRouteModelOpenAIResponsesAutoReviewDisabled(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:         true,
		RouteProvider:   "codex",
		RouteModel:      "gpt-5.5",
		RouteAutoReview: false,
		Models:          []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Headers:            http.Header{"X-Openai-Subagent": []string{"guardian"}},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"review this approval"}]}]}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false when route_auto_review is disabled")
	}
}

func TestRouteModelOpenAIResponsesAutoReviewHeaderBypassesAllowlist(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:         true,
		RouteProvider:   "codex",
		RouteModel:      "gpt-5.5",
		RouteAutoReview: true,
		Models:          []string{"gpt-5.4"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Headers:            http.Header{"X-Openai-Subagent": []string{"guardian"}},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"review this approval"}]}]}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true for guardian header even when requested model is outside allowlist")
	}
}

func TestRouteModelOpenAIResponsesAutoReviewHeaderRespectsDenylist(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:         true,
		RouteProvider:   "codex",
		RouteModel:      "gpt-5.5",
		RouteAutoReview: true,
		Models:          []string{"gpt-5.4"},
		ExcludedModels:  []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Headers:            http.Header{"X-Openai-Subagent": []string{"guardian"}},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"review this approval"}]}]}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false when requested model is explicitly denied")
	}
}

func TestRouteModelOpenAIResponsesWebSearch(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:        true,
		RouteProvider:  "codex",
		RouteModel:     "gpt-5.5",
		RouteWebSearch: true,
		Models:         []string{"gpt-5.4"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai",
			RequestedModel:     "gpt-5.4",
			AvailableProviders: []string{"codex"},
			Body:               []byte(`{"model":"gpt-5.4","input":"search","tools":[{"type":"web_search_preview"}],"tool_choice":{"type":"web_search_preview"}}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetKind != pluginapi.ModelRouteTargetProvider || resp.Target != "codex" || resp.TargetModel != "gpt-5.5" {
		t.Fatalf("route = %#v, want provider codex/gpt-5.5", resp)
	}
}

func TestRouteModelOpenAIResponseSourceFormatWebSearchToolChoice(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:        true,
		RouteProvider:  "codex",
		RouteModel:     "gpt-5.5",
		RouteWebSearch: true,
		Models:         []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":"search","tools":[{"type":"web_search"}],"tool_choice":{"type":"web_search"}}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetKind != pluginapi.ModelRouteTargetProvider || resp.Target != "codex" || resp.TargetModel != "gpt-5.5" {
		t.Fatalf("route = %#v, want provider codex/gpt-5.5", resp)
	}
}

func TestRouteModelOpenAIResponseSourceFormatWebSearchDefinitionNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:        true,
		RouteProvider:  "codex",
		RouteModel:     "gpt-5.5",
		RouteWebSearch: true,
		Models:         []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":"hello","tools":[{"type":"web_search"}]}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for available web_search tool definition")
	}
}

func TestRouteModelOpenAIResponseSourceFormatWebSearchCallNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:        true,
		RouteProvider:  "codex",
		RouteModel:     "gpt-5.5",
		RouteWebSearch: true,
		Models:         []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":[{"type":"web_search_call","status":"completed"}],"tools":[{"type":"web_search"}]}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for historical web search call")
	}
}

func TestRouteModelOpenAIResponsesOldWebSearchHistoryNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:        true,
		RouteProvider:  "codex",
		RouteModel:     "gpt-5.5",
		RouteWebSearch: true,
		Models:         []string{"gpt-5.4"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4",
				"tools":[{"type":"web_search"}],
				"input":[
					{"type":"message","role":"user","content":[{"type":"input_text","text":"查一下今天的天气"}]},
					{"type":"web_search_call","status":"completed"},
					{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]},
					{"type":"message","role":"user","content":[{"type":"input_text","text":"用一句话解释二分查找"}]}
				],
				"stream":true
			}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for old web search history")
	}
}

func TestRouteModelOpenAIResponsesCurrentSearchIntentRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:        true,
		RouteProvider:  "codex",
		RouteModel:     "gpt-5.5",
		RouteWebSearch: true,
		Models:         []string{"gpt-5.4"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4",
				"tools":[{"type":"web_search"}],
				"input":[
					{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]},
					{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]},
					{"type":"message","role":"user","content":[{"type":"input_text","text":"查一下今天的天气"}]}
				],
				"stream":true
			}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true for current search intent")
	}
}

func TestRouteModelOpenAIResponsesClientMetadataSearchCapabilityNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:        true,
		RouteProvider:  "codex",
		RouteModel:     "gpt-5.5",
		RouteWebSearch: true,
		RouteVision:    true,
		Models:         []string{"gpt-5.4"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4",
				"client_metadata":{"web_search":true,"vision":true},
				"tools":[{"type":"web_search"},{"type":"function","name":"view_image"}],
				"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"继续"}]}],
				"stream":true
			}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for metadata capability without current user search intent")
	}
}

func TestRouteModelOpenAIResponsesVision(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteVision:   true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4-mini",
				"input":[{"role":"user","content":[{"type":"input_text","text":"what is this"},{"type":"input_image","image_url":"data:image/png;base64,abc"}]}]
			}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetModel != "gpt-5.5" {
		t.Fatalf("TargetModel = %q, want gpt-5.5", resp.TargetModel)
	}
}

func TestRouteModelOpenAIResponsesVisionImageURLObject(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteVision:   true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4-mini",
				"input":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,abc"}}]}]
			}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetModel != "gpt-5.5" {
		t.Fatalf("TargetModel = %q, want gpt-5.5", resp.TargetModel)
	}
}

func TestRouteModelOpenAIResponsesVisionToolOutputNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteVision:   true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4-mini",
				"input":[{"type":"function_call_output","call_id":"call_1","output":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc"}}]}]
			}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for historical vision tool output")
	}
}

func TestRouteModelOpenAIResponsesCodexContinuationWithHistoricalToolSignalsNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:        true,
		RouteProvider:  "codex",
		RouteModel:     "gpt-5.5",
		RouteWebSearch: true,
		RouteVision:    true,
		Models:         []string{"gpt-5.4"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4",
				"tools":[{"type":"function","name":"view_image"},{"type":"web_search"}],
				"input":[
					{"type":"message","role":"user","content":[{"type":"input_text","text":"你看下 ~/zDesktop/1.png"}]},
					{"type":"function_call","name":"view_image"},
					{"type":"function_call_output","call_id":"call_1","output":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc"}}]},
					{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]},
					{"type":"message","role":"user","content":[{"type":"input_text","text":"继续处理目标任务"}]},
					{"type":"custom_tool_call","name":"exec_command"},
					{"type":"custom_tool_call_output","output":"ok"}
				],
				"stream":true
			}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for continuation with only historical web/vision signals")
	}
}

func TestRouteModelOpenAIResponsesVisionScreenshot(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteVision:   true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4-mini",
				"input":[{"role":"user","content":[{"type":"computer_screenshot","image_url":"data:image/png;base64,abc"}]}]
			}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetModel != "gpt-5.5" {
		t.Fatalf("TargetModel = %q, want gpt-5.5", resp.TargetModel)
	}
}

func TestRouteModelOpenAIResponsesVisionImagePathMention(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteVision:   true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4-mini",
				"input":"你看下 ~/zDesktop/1.png"
			}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetModel != "gpt-5.5" {
		t.Fatalf("TargetModel = %q, want gpt-5.5", resp.TargetModel)
	}
}

func TestRouteModelOpenAIResponsesVisionToolDefinitionNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteVision:   true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4-mini",
				"input":"inspect the image",
				"tools":[{"type":"function","name":"view_image","description":"View a local image"}]
			}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for available view_image tool definition")
	}
}

func TestRouteModelOpenAIResponsesVisionToolChoice(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteVision:   true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4-mini",
				"input":"inspect the image",
				"tools":[{"type":"function","name":"view_image","description":"View a local image"}],
				"tool_choice":{"name":"view_image"}
			}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetModel != "gpt-5.5" {
		t.Fatalf("TargetModel = %q, want gpt-5.5", resp.TargetModel)
	}
}

func TestRouteModelOpenAIResponsesImageGenerationIntentDoesNotRouteByDefault(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:              true,
		RouteProvider:        "codex",
		RouteModel:           "gpt-5.5",
		ImageToolModel:       "gpt-image-2",
		RouteImageGeneration: true,
		Models:               []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"cliproxyapi", "codex"},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":"生成一张赛博朋克城市夜景图片"}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false when image_route_provider is empty")
	}
}

func TestRouteModelOpenAIResponsesImageGenerationIntentRoutesWithImageProvider(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:              true,
		RouteProvider:        "cliproxyapi",
		RouteModel:           "gpt-5.4",
		ImageRouteProvider:   "image-capable-codex",
		ImageToolModel:       "gpt-image-2",
		RouteImageGeneration: true,
		Models:               []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"cliproxyapi", "image-capable-codex"},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":"生成一张赛博朋克城市夜景图片"}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.Target != "image-capable-codex" || resp.TargetModel != "gpt-5.4" {
		t.Fatalf("route = %#v, want provider image-capable-codex/gpt-5.4", resp)
	}
}

func TestRouteModelOpenAIResponsesImageGenerationFollowupRoutesWithImageProvider(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:              true,
		RouteProvider:        "codex",
		RouteModel:           "gpt-5.5",
		ImageRouteProvider:   "image-capable-codex",
		ImageToolModel:       "gpt-image-2",
		RouteImageGeneration: true,
		Models:               []string{"gpt-5.4"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4",
			AvailableProviders: []string{"codex", "image-capable-codex"},
			Body: []byte(`{
				"model":"gpt-5.4",
				"input":[
					{"type":"message","role":"user","content":[{"type":"input_text","text":"生成一张红色小猫图片"}]},
					{"type":"message","role":"assistant","content":[{"type":"output_text","text":"已生成。"}]},
					{"type":"message","role":"user","content":[{"type":"input_text","text":"再来一张，换成蓝色"}]}
				]
			}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.Target != "image-capable-codex" || resp.TargetModel != "gpt-5.5" {
		t.Fatalf("route = %#v, want provider image-capable-codex/gpt-5.5", resp)
	}
}

func TestRouteModelImageGenerationOverrideUsesImageRouteTarget(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:              true,
		RouteModel:           "gpt-5.5",
		ImageRouteProvider:   "image-capable-codex",
		RouteImageGeneration: true,
		Models:               []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"image-capable-codex"},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":"生成一张产品头像图片"}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.Target != "image-capable-codex" || resp.TargetModel != "gpt-5.5" {
		t.Fatalf("route = %#v, want provider image-capable-codex/gpt-5.5", resp)
	}
}

func TestConfigureMapsLegacyImageProviderAlias(t *testing.T) {
	raw, err := json.Marshal(lifecycleRequest{ConfigYAML: []byte(`
enabled: true
image_provider: image-capable-codex
`)})
	if err != nil {
		t.Fatal(err)
	}
	if err := configure(raw); err != nil {
		t.Fatal(err)
	}
	cfg := loadedConfig()
	if cfg.ImageRouteProvider != "image-capable-codex" {
		t.Fatalf("image route provider = %q, want image-capable-codex", cfg.ImageRouteProvider)
	}
}

func TestRouteModelOpenAIResponsesImageGenerationTroubleshootingNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:              true,
		RouteProvider:        "cliproxyapi",
		RouteModel:           "gpt-5.4",
		ImageToolModel:       "gpt-image-2",
		RouteImageGeneration: true,
		Models:               []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"cliproxyapi"},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":"生成图片报错，插件能不能解决 image_gen 工具不可用"}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for troubleshooting discussion")
	}
}

func TestRouteModelOpenAIResponsesOldImageHistoryNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteVision:   true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4-mini",
				"input":[
					{"type":"message","role":"user","content":[{"type":"input_text","text":"你看下 ~/zDesktop/1.png"}]},
					{"type":"function_call_output","call_id":"call_1","output":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc"}}]},
					{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]},
					{"type":"message","role":"user","content":[{"type":"input_text","text":"继续写代码"}]}
				],
				"stream":true
			}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for old image history")
	}
}

func TestRouteModelOpenAIResponsesCompactContextManagement(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteCompact:  true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4-mini",
				"context_management":{"type":"compact"},
				"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Please preserve important implementation details."}]}],
				"tools":[{"type":"web_search"}],
				"stream":true
			}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetModel != "gpt-5.5" {
		t.Fatalf("TargetModel = %q, want gpt-5.5", resp.TargetModel)
	}
}

func TestRouteModelOpenAIResponsesCompactInstructionNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteCompact:  true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4-mini",
				"instructions":"Summarize the conversation so far and compact the context for continuation.",
				"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Please preserve important implementation details."}]}],
				"stream":true
			}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for natural-language compact instruction without protocol signal")
	}
}

func TestRouteModelOpenAIResponsesCompactAlt(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteCompact:  true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":"x","stream":false}`),
			Query:              map[string][]string{"alt": {"responses/compact"}},
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetModel != "gpt-5.5" {
		t.Fatalf("TargetModel = %q, want gpt-5.5", resp.TargetModel)
	}
}

func TestRouteModelOpenAIResponsesCompactionTrigger(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteCompact:  true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":[{"type":"message","role":"user","content":"before"},{"type":"compaction_trigger"}],"stream":true}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetModel != "gpt-5.5" {
		t.Fatalf("TargetModel = %q, want gpt-5.5", resp.TargetModel)
	}
}

func TestRouteModelOpenAIResponsesCompactionReplayNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteCompact:  true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":[{"type":"compaction","encrypted_content":"summary"},{"type":"message","role":"user","content":"continue"}],"stream":true}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for post-compaction replay")
	}
}

func TestRouteModelOpenAIResponsesCodexCheckpointCompactionMarker(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteCompact:  true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4-mini",
				"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"You are performing a CONTEXT CHECKPOINT COMPACTION. Create a handoff summary."}]}],
				"stream":true
			}`),
		},
	})

	if !resp.Handled {
		t.Fatalf("Handled = false, want true")
	}
	if resp.TargetModel != "gpt-5.5" {
		t.Fatalf("TargetModel = %q, want gpt-5.5", resp.TargetModel)
	}
}

func TestRouteModelOpenAIResponsesOldCheckpointCompactionMarkerNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteCompact:  true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4-mini",
				"input":[
					{"type":"message","role":"user","content":[{"type":"input_text","text":"You are performing a CONTEXT CHECKPOINT COMPACTION. Create a handoff summary."}]},
					{"type":"message","role":"assistant","content":[{"type":"output_text","text":"summary"}]},
					{"type":"function_call","name":"tool","call_id":"call_1"},
					{"type":"function_call_output","call_id":"call_1","output":"ok"},
					{"type":"reasoning","summary":[]},
					{"type":"message","role":"user","content":[{"type":"input_text","text":"continue normal work"}]}
				],
				"stream":true
			}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for old compact marker outside recent input")
	}
}

func TestRouteModelOpenAIResponsesOrdinarySummaryNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteCompact:  true,
		Models:        []string{"gpt-5.4-mini"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4-mini",
			AvailableProviders: []string{"codex"},
			Body:               []byte(`{"model":"gpt-5.4-mini","input":"Summarize this paragraph: hello world","stream":true}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for ordinary summary request")
	}
}

func TestRouteModelOpenAIChatShapeNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:        true,
		RouteProvider:  "codex",
		RouteModel:     "gpt-5.5",
		RouteWebSearch: true,
		Models:         []string{"gpt-5.4"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai",
			RequestedModel:     "gpt-5.4",
			AvailableProviders: []string{"codex"},
			Body:               []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"web_search_preview"}]}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for chat-completions shape")
	}
}

func routeForTest(t *testing.T, req rpcModelRouteRequest) pluginapi.ModelRouteResponse {
	t.Helper()
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	out, err := routeModel(raw)
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
	var resp pluginapi.ModelRouteResponse
	if err := json.Unmarshal(env.Result, &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

// Regression: a plain UI edit instruction containing "改成" must NOT be treated
// as an image-generation continuation, even when recent history holds an
// image_generation tool call. Previously "改成只有输入框悬浮的样式" rerouted
// ordinary requests to route_model (gpt-5.5).
func TestRouteModelImageGenContinuationAmbiguousUIEditNotRouted(t *testing.T) {
	currentConfig.Store(pluginConfig{
		Enabled:              true,
		RouteProvider:        "codex",
		RouteModel:           "gpt-5.5",
		ImageRouteProvider:   "codex",
		RouteImageGeneration: true,
		Models:               []string{"gpt-5.4"},
	})

	resp := routeForTest(t, rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.4",
			AvailableProviders: []string{"codex"},
			Body: []byte(`{
				"model":"gpt-5.4",
				"input":[
					{"type":"message","role":"user","content":[{"type":"input_text","text":"生成一张红色小猫图片"}]},
					{"type":"function_call","name":"image_generation"},
					{"type":"function_call_output","call_id":"c1","output":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc"}}]},
					{"type":"message","role":"assistant","content":[{"type":"output_text","text":"已生成。"}]},
					{"type":"message","role":"user","content":[{"type":"input_text","text":"改成只有输入框悬浮的样式，然后样式和搜索框的样式统一对齐行。"}]}
				]
			}`),
		},
	})

	if resp.Handled {
		t.Fatalf("Handled = true, want false for ordinary UI edit; route=%#v", resp)
	}
}

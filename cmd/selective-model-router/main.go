package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	cliproxy_host_call_fn call;
	cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
	"gopkg.in/yaml.v3"
)

const pluginIdentifier = "selective-router"
const pluginDisplayName = "Selective Router"

var currentConfig atomic.Value

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type lifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
}

type pluginConfig struct {
	Enabled              bool     `yaml:"enabled"`
	RouteProvider        string   `yaml:"route_provider"`
	RouteModel           string   `yaml:"route_model"`
	ImageRouteProvider   string   `yaml:"image_route_provider"`
	ImageProvider        string   `yaml:"image_provider"`
	ImageToolModel       string   `yaml:"image_tool_model"`
	RouteCompact         bool     `yaml:"route_compact"`
	RouteAutoReview      bool     `yaml:"route_auto_review"`
	RouteWebSearch       bool     `yaml:"route_web_search"`
	RouteVision          bool     `yaml:"route_vision"`
	RouteImageGeneration bool     `yaml:"route_image_generation"`
	DebugRoutes          bool     `yaml:"debug_routes"`
	Models               []string `yaml:"models"`
	ExcludedModels       []string `yaml:"excluded_models"`
	RectifyTranscript    bool     `yaml:"rectify_transcript"`
}

type registration struct {
	SchemaVersion uint32                 `json:"schema_version"`
	Metadata      pluginapi.Metadata     `json:"metadata"`
	Capabilities  registrationCapability `json:"capabilities"`
}

type registrationCapability struct {
	ModelRouter            bool `json:"model_router"`
	RequestNormalizer      bool `json:"request_normalizer"`
	ManagementAPI          bool `json:"management_api"`
	StreamChunkInterceptor bool `json:"response_stream_interceptor"`
	ResponseInterceptor    bool `json:"response_interceptor"`
}

type rpcModelRouteRequest struct {
	pluginapi.ModelRouteRequest
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
	_ = host
	plugin.abi_version = C.uint32_t(pluginabi.ABIVersion)
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeResponse(response, errorEnvelope("invalid_method", "method is required"))
		return 1
	}
	var requestBytes []byte
	if request != nil && requestLen > 0 {
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}
	raw, errHandle := handleMethod(C.GoString(method), requestBytes)
	if errHandle != nil {
		writeResponse(response, errorEnvelope("plugin_error", errHandle.Error()))
		return 1
	}
	writeResponse(response, raw)
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, _ C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {}

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		if err := configure(request); err != nil {
			return nil, err
		}
		return okEnvelope(pluginRegistration())
	case pluginabi.MethodModelRoute:
		return routeModel(request)
	case pluginabi.MethodRequestNormalize:
		return normalizeRequest(request)
	case pluginabi.MethodResponseInterceptStreamChunk:
		return interceptStreamChunkRequest(request)
	case pluginabi.MethodResponseInterceptAfter:
		return interceptResponseRequest(request)
	case pluginabi.MethodManagementRegister:
		return okEnvelope(managementRegistration())
	case pluginabi.MethodManagementHandle:
		return handleManagement(request)
	case pluginabi.MethodExecutorIdentifier:
		return okEnvelope(map[string]string{"identifier": pluginIdentifier})
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func configure(raw []byte) error {
	var req lifecycleRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			return err
		}
	}
	cfg := defaultPluginConfig()
	if len(req.ConfigYAML) > 0 {
		if err := yaml.Unmarshal(req.ConfigYAML, &cfg); err != nil {
			return err
		}
	}
	cfg.RouteProvider = strings.TrimSpace(cfg.RouteProvider)
	cfg.RouteModel = strings.TrimSpace(cfg.RouteModel)
	cfg.ImageRouteProvider = strings.TrimSpace(cfg.ImageRouteProvider)
	cfg.ImageProvider = strings.TrimSpace(cfg.ImageProvider)
	cfg.ImageToolModel = strings.TrimSpace(cfg.ImageToolModel)
	if cfg.RouteProvider == "" {
		cfg.RouteProvider = "codex"
	}
	if cfg.RouteModel == "" {
		cfg.RouteModel = "gpt-5.5"
	}
	if cfg.ImageRouteProvider == "" {
		cfg.ImageRouteProvider = cfg.ImageProvider
	}
	if cfg.ImageToolModel == "" {
		cfg.ImageToolModel = "gpt-image-2"
	}
	currentConfig.Store(cfg)
	return nil
}

func defaultPluginConfig() pluginConfig {
	return pluginConfig{
		Enabled:              true,
		RouteProvider:        "codex",
		RouteModel:           "gpt-5.5",
		ImageToolModel:       "gpt-image-2",
		RouteCompact:         true,
		RouteAutoReview:      true,
		RouteWebSearch:       true,
		RouteVision:          true,
		RouteImageGeneration: true,
		RectifyTranscript:    true,
	}
}

func loadedConfig() pluginConfig {
	if raw := currentConfig.Load(); raw != nil {
		if cfg, ok := raw.(pluginConfig); ok {
			return cfg
		}
	}
	return defaultPluginConfig()
}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: pluginabi.SchemaVersion,
		Metadata: pluginapi.Metadata{
			Name:             pluginDisplayName,
			Version:          "0.1.0",
			Author:           "vxider",
			Logo:             "https://raw.githubusercontent.com/Vxider/CPA-Selective-Router/main/assets/icon.svg",
			GitHubRepository: "https://github.com/router-for-me/CLIProxyAPI",
			ConfigFields: []pluginapi.ConfigField{
				{Name: "enabled", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Enable capability-based route conversion."},
				{Name: "route_provider", Type: pluginapi.ConfigFieldTypeString, Description: "Provider used for direct model_router route conversion."},
				{Name: "route_model", Type: pluginapi.ConfigFieldTypeString, Description: "Target model used for direct model_router route conversion."},
				{Name: "image_route_provider", Type: pluginapi.ConfigFieldTypeString, Description: "Provider used only for image-generation route override. Legacy alias: image_provider."},
				{Name: "image_tool_model", Type: pluginapi.ConfigFieldTypeString, Description: "Model used by the injected image_generation tool. Default: gpt-image-2."},
				{Name: "models", Type: pluginapi.ConfigFieldTypeArray, Description: "Requested model allowlist. Empty means all models. Supports '*' wildcards, e.g. model-*."},
				{Name: "excluded_models", Type: pluginapi.ConfigFieldTypeArray, Description: "Requested model denylist. Takes precedence over models. Supports '*' wildcards, e.g. model-*."},
				{Name: "route_compact", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Route matching compact response requests to route_provider/route_model. Default: true."},
				{Name: "route_auto_review", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Route Codex auto-review reviewer requests to route_provider/route_model. Matches X-OpenAI-Subagent: guardian and codex-auto-review. Default: true."},
				{Name: "route_web_search", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Route matching web search requests to route_provider/route_model. Also injects a native web_search tool for matching response requests with search intent."},
				{Name: "route_vision", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Route matching requests with image input to the configured capability route."},
				{Name: "route_image_generation", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Inject image_generation tool for explicit image generation requests. When image_route_provider is set, image requests are routed to that provider using route_model."},
				{Name: "rectify_transcript", Type: pluginapi.ConfigFieldTypeBoolean, Description: "Synthesize missing tool call outputs in streamed responses. Fixes orphaned function_call/tool_search_call/custom_tool_call items that have no matching output. Default: true."},
			},
		},
		Capabilities: registrationCapability{
			ModelRouter:            true,
			RequestNormalizer:      true,
			ManagementAPI:          true,
			StreamChunkInterceptor: true,
			ResponseInterceptor:    true,
		},
	}
}

func handleManagement(raw []byte) ([]byte, error) {
	var req pluginapi.ManagementRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	resp := handleManagementRequest(req)
	return okEnvelope(resp)
}

func normalizeRequest(raw []byte) ([]byte, error) {
	var req pluginapi.RequestTransformRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	cfg := loadedConfig()
	body := req.Body
	if cfg.Enabled && cfg.RouteImageGeneration && imageGenerationInjectionAllowed(cfg) && modelAllowed(req.Model, cfg) && isResponseTransformCandidate(req) {
		if injected, changed := injectImageGenerationTool(body, cfg); changed {
			body = injected
		}
	}
	return okEnvelope(pluginapi.PayloadResponse{Body: body})
}

func interceptStreamChunkRequest(raw []byte) ([]byte, error) {
	var req pluginapi.StreamChunkInterceptRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	cfg := loadedConfig()
	if !cfg.Enabled || !cfg.RectifyTranscript {
		return okEnvelope(pluginapi.StreamChunkInterceptResponse{})
	}
	modifiedBody, injectedCount, drop := patchChunkBody(req.Body, req.ChunkIndex, req.Metadata)
	if drop {
		return okEnvelope(pluginapi.StreamChunkInterceptResponse{DropChunk: true})
	}
	if injectedCount > 0 {
		recordRectifierDecision("stream", injectedCount)
	}
	return okEnvelope(pluginapi.StreamChunkInterceptResponse{Body: modifiedBody})
}

func interceptResponseRequest(raw []byte) ([]byte, error) {
	var req pluginapi.ResponseInterceptRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	cfg := loadedConfig()
	if !cfg.Enabled || !cfg.RectifyTranscript {
		return okEnvelope(pluginapi.ResponseInterceptResponse{})
	}
	modified, injectedCount := ensureNoOrphanCallsInNonStreamResponse(req.Body)
	if injectedCount > 0 {
		recordRectifierDecision("non_stream", injectedCount)
	}
	return okEnvelope(pluginapi.ResponseInterceptResponse{Body: modified})
}

func imageGenerationInjectionAllowed(cfg pluginConfig) bool {
	return true
}

func routeModel(raw []byte) ([]byte, error) {
	var req rpcModelRouteRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	cfg := loadedConfig()
	if !cfg.Enabled {
		debugRoute(req, cfg, "disabled", false)
		recordRouteDecision(req, cfg, routeDecision{}, "disabled", false)
		return okEnvelope(pluginapi.ModelRouteResponse{Handled: false})
	}
	if !routeModelAllowed(req, cfg) {
		debugRoute(req, cfg, "no_match", false)
		recordRouteDecision(req, cfg, routeDecision{}, "no_match", false)
		return okEnvelope(pluginapi.ModelRouteResponse{Handled: false})
	}
	decision := routeTargetForRequest(req, cfg)
	if decision.Handled {
		if decision.Provider == "" || !hasProvider(req.AvailableProviders, decision.Provider) {
			debugRoute(req, cfg, "route_provider_unavailable", false)
			recordRouteDecision(req, cfg, decision, "route_provider_unavailable", false)
			return okEnvelope(pluginapi.ModelRouteResponse{
				Handled: false,
				Reason:  "route_provider_unavailable",
			})
		}
		debugRoute(req, cfg, "capability_route_conversion", true)
		recordRouteDecision(req, cfg, decision, "capability_route_conversion", true)
		return okEnvelope(pluginapi.ModelRouteResponse{
			Handled:     true,
			TargetKind:  pluginapi.ModelRouteTargetProvider,
			Target:      decision.Provider,
			TargetModel: decision.Model,
			Reason:      "capability_route_conversion",
		})
	}
	debugRoute(req, cfg, "no_match", false)
	recordRouteDecision(req, cfg, decision, "no_match", false)
	return okEnvelope(pluginapi.ModelRouteResponse{Handled: false})
}

func debugRoute(req rpcModelRouteRequest, cfg pluginConfig, reason string, handled bool) {
	if !cfg.DebugRoutes {
		return
	}
	decision := routeTargetForRequest(req, cfg)
	targetProvider := decision.Provider
	targetModel := decision.Model
	if !decision.Handled {
		targetProvider = ""
		targetModel = ""
	}
	_, _ = fmt.Fprintf(os.Stderr, "selective-router route_debug handled=%t reason=%s requested_model=%s target_provider=%s target_model=%s source=%s stream=%t response_candidate=%t compact=%t auto_review=%t web_search=%t vision=%t body_shape=%s\n",
		handled,
		reason,
		strings.TrimSpace(req.RequestedModel),
		targetProvider,
		targetModel,
		strings.TrimSpace(req.SourceFormat),
		req.Stream,
		isResponseRouteCandidate(req),
		isCompactResponseRequest(req),
		isAutoReviewRequest(req),
		hasWebSearchRouteSignal(req.Body),
		hasImageInput(req.Body) || hasVisionTool(req.Body) || hasImagePathMention(req.Body),
		summarizeBodyShape(req.Body),
	)
}

func hasProvider(providers []string, key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, provider := range providers {
		if strings.ToLower(strings.TrimSpace(provider)) == key {
			return true
		}
	}
	return false
}

func modelAllowed(model string, cfg pluginConfig) bool {
	model = strings.TrimSpace(model)
	if modelExcluded(model, cfg) {
		return false
	}
	return modelAllowedByAllowlist(model, cfg)
}

func routeModelAllowed(req rpcModelRouteRequest, cfg pluginConfig) bool {
	model := strings.TrimSpace(req.RequestedModel)
	if modelAllowed(model, cfg) {
		return true
	}
	return cfg.RouteAutoReview && isGuardianSubagent(req.Headers) && !modelExcluded(model, cfg)
}

func modelExcluded(model string, cfg pluginConfig) bool {
	for _, pattern := range cfg.ExcludedModels {
		if wildcardMatch(strings.TrimSpace(pattern), model) {
			return true
		}
	}
	return false
}

func modelAllowedByAllowlist(model string, cfg pluginConfig) bool {
	allowPatterns := trimmedNonEmpty(cfg.Models)
	if len(allowPatterns) == 0 {
		return true
	}
	for _, pattern := range allowPatterns {
		if wildcardMatch(pattern, model) {
			return true
		}
	}
	return false
}

func trimmedNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func wildcardMatch(pattern, value string) bool {
	pattern = strings.TrimSpace(pattern)
	value = strings.TrimSpace(value)
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}
	parts := strings.Split(pattern, "*")
	pos := 0
	if parts[0] != "" {
		if !strings.HasPrefix(value, parts[0]) {
			return false
		}
		pos = len(parts[0])
	}
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		if part == "" {
			continue
		}
		idx := strings.Index(value[pos:], part)
		if idx < 0 {
			return false
		}
		pos += idx + len(part)
	}
	last := parts[len(parts)-1]
	return last == "" || strings.HasSuffix(value, last)
}

func okEnvelope(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{OK: true, Result: raw})
}

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	ptr := C.CBytes(raw)
	if ptr == nil {
		return
	}
	response.ptr = ptr
	response.len = C.size_t(len(raw))
}

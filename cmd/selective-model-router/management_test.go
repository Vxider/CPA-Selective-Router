package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
)

func TestManagementRegistrationDeclaresResources(t *testing.T) {
	reg := managementRegistration()
	if len(reg.Resources) == 0 {
		t.Fatalf("no resources declared")
	}
	var hasDashboard, hasStats, hasLogs, hasClear bool
	for _, r := range reg.Resources {
		switch r.Path {
		case managementResourceDashboard:
			hasDashboard = r.Menu != ""
		case managementResourceStats:
			hasStats = true
		case managementResourceLogs:
			hasLogs = true
		case managementResourceClear:
			hasClear = true
		}
	}
	if !hasDashboard || !hasStats || !hasLogs || !hasClear {
		t.Fatalf("missing resources: dashboard=%v stats=%v logs=%v clear=%v", hasDashboard, hasStats, hasLogs, hasClear)
	}
}

func TestManagementRegistrationJSONKeys(t *testing.T) {
	// The host unmarshals into pluginapi.ResourceRoute (no json tags => PascalCase).
	raw, err := json.Marshal(managementRegistration())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"Path"`) || !strings.Contains(string(raw), `"Menu"`) {
		t.Fatalf("resource fields must marshal PascalCase, got: %s", raw)
	}
	if !strings.Contains(string(raw), `"resources"`) {
		t.Fatalf("envelope key must be lowercase resources, got: %s", raw)
	}
}

func TestHandleManagementRoutesByPath(t *testing.T) {
	routeLog.clear()

	resp := handleManagementRequest(pluginapi.ManagementRequest{Method: http.MethodGet, Path: "/v0/resource/plugins/selective-router/api/stats"})
	if resp.StatusCode != 0 && resp.StatusCode != http.StatusOK {
		t.Fatalf("stats status=%d", resp.StatusCode)
	}
	if ct := resp.Headers.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("stats content-type=%q", ct)
	}

	dash := handleManagementRequest(pluginapi.ManagementRequest{Method: http.MethodGet, Path: "/v0/resource/plugins/selective-router/dashboard"})
	if ct := dash.Headers.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("dashboard content-type=%q", ct)
	}
	if !strings.Contains(string(dash.Body), "Selective Router") {
		t.Fatalf("dashboard body missing title")
	}

	missing := handleManagementRequest(pluginapi.ManagementRequest{Method: http.MethodGet, Path: "/v0/resource/plugins/selective-router/missing"})
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing status=%d", missing.StatusCode)
	}
}

func TestHandleManagementLogsReturnOnlyHandledRoutes(t *testing.T) {
	routeLog.clear()
	routeLog.record(routeLogEvent{Phase: "route", Category: "normal", Reason: "no_match", Handled: false})
	routeLog.record(routeLogEvent{Phase: "route", Category: "web_search", Reason: "capability_route_conversion", Handled: true})

	resp := handleManagementRequest(pluginapi.ManagementRequest{Method: http.MethodGet, Path: "/v0/resource/plugins/selective-router/api/logs"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logs status=%d", resp.StatusCode)
	}
	var payload struct {
		Events []routeLogEvent `json:"events"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Events) != 1 || payload.Events[0].Category != "web_search" {
		t.Fatalf("logs events=%#v, want only handled web_search event", payload.Events)
	}
}

func TestHandleManagementClearGuardsAndResets(t *testing.T) {
	routeLog.clear()
	routeLog.record(routeLogEvent{Phase: "route", Category: "compact", Handled: true})

	noConfirm := handleManagementRequest(pluginapi.ManagementRequest{Method: http.MethodGet, Path: "/v0/resource/plugins/selective-router/api/clear"})
	if noConfirm.StatusCode != http.StatusBadRequest {
		t.Fatalf("clear without confirm status=%d", noConfirm.StatusCode)
	}

	clearReq := pluginapi.ManagementRequest{Method: http.MethodGet, Path: "/v0/resource/plugins/selective-router/api/clear", Query: url.Values{"confirm": []string{"1"}}}
	ok := handleManagementRequest(clearReq)
	if ok.StatusCode != http.StatusOK {
		t.Fatalf("clear status=%d", ok.StatusCode)
	}
	events := routeLog.snapshot()
	if len(events) != 0 {
		t.Fatalf("expected empty log after clear, got events=%d", len(events))
	}
}

func TestRouteModelRecordsToLog(t *testing.T) {
	routeLog.clear()
	currentConfig.Store(pluginConfig{
		Enabled:       true,
		RouteProvider: "codex",
		RouteModel:    "gpt-5.5",
		RouteCompact:  true,
	})
	raw, _ := json.Marshal(rpcModelRouteRequest{
		ModelRouteRequest: pluginapi.ModelRouteRequest{
			SourceFormat:       "openai-response",
			RequestedModel:     "gpt-5.5",
			AvailableProviders: []string{"codex"},
			Query:              url.Values{"alt": []string{"responses/compact"}},
			Body:               []byte(`{"model":"gpt-5.5","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`),
		},
	})
	if _, err := routeModel(raw); err != nil {
		t.Fatal(err)
	}
	stats := routeLog.stats()
	if stats.TotalRoutes != 1 || stats.HandledRoutes != 1 {
		t.Fatalf("stats=%+v", stats)
	}
	if stats.ByCategory["compact"] != 1 {
		t.Fatalf("compact category not recorded: %+v", stats.ByCategory)
	}
}

func TestManagementHandleRPCRoundTrip(t *testing.T) {
	// management.register -> envelope {ok:true, result:{resources:[...]}}
	regRaw, err := handleMethod(pluginabi.MethodManagementRegister, nil)
	if err != nil {
		t.Fatal(err)
	}
	var regEnv envelope
	if err := json.Unmarshal(regRaw, &regEnv); err != nil || !regEnv.OK {
		t.Fatalf("register envelope: %s err=%v", regRaw, err)
	}
	if !strings.Contains(string(regEnv.Result), `"resources"`) {
		t.Fatalf("register result missing resources key: %s", regEnv.Result)
	}

	// management.handle -> envelope wrapping pluginapi.ManagementResponse (PascalCase keys).
	reqBody, _ := json.Marshal(pluginapi.ManagementRequest{Method: http.MethodGet, Path: "/v0/resource/plugins/selective-router/dashboard"})
	hRaw, err := handleMethod(pluginabi.MethodManagementHandle, reqBody)
	if err != nil {
		t.Fatal(err)
	}
	var hEnv envelope
	if err := json.Unmarshal(hRaw, &hEnv); err != nil || !hEnv.OK {
		t.Fatalf("handle envelope: %s err=%v", hRaw, err)
	}
	if !strings.Contains(string(hEnv.Result), `"StatusCode"`) || !strings.Contains(string(hEnv.Result), `"Body"`) {
		t.Fatalf("management response must use PascalCase StatusCode/Body, got: %s", hEnv.Result)
	}
}

func TestRouteLogBucketsFixedThreeHourWindow(t *testing.T) {
	routeLog.clear()
	// Events in the recent past so they fall inside the rolling 3-hour window.
	now := time.Now()
	base := now.Add(-2 * time.Hour).Truncate(10 * time.Minute)
	routeLog.record(routeLogEvent{Time: base, Phase: "route", Category: "normal", Handled: false})
	routeLog.record(routeLogEvent{Time: base.Add(3 * time.Minute), Phase: "route", Category: "web_search", Handled: true})
	routeLog.record(routeLogEvent{Time: base.Add(7 * time.Minute), Phase: "route", Category: "normal", Handled: false})
	routeLog.record(routeLogEvent{Time: base.Add(15 * time.Minute), Phase: "route", Category: "vision", Handled: true})

	stats := routeLog.stats()
	const wantBuckets = 18 // 3h / 10m
	if len(stats.Buckets) != wantBuckets {
		t.Fatalf("expected %d fixed buckets, got %d", wantBuckets, len(stats.Buckets))
	}
	// Events outside the 3h window must be ignored.
	routeLog.clear()
	routeLog.record(routeLogEvent{Time: now.Add(-4 * time.Hour), Phase: "route", Category: "normal"})
	stats = routeLog.stats()
	for _, b := range stats.Buckets {
		if b.Total != 0 {
			t.Fatalf("event outside 3h window should not appear, bucket %+v has total %d", b, b.Total)
		}
	}
	// Empty buckets are still rendered as placeholders.
	if len(stats.Buckets) != wantBuckets {
		t.Fatalf("expected %d buckets even when empty, got %d", wantBuckets, len(stats.Buckets))
	}
}

func TestCumulativeCountersSurviveRingBufferEviction(t *testing.T) {
	routeLog.clear()
	// Record more than routeLogCapacity events so the ring buffer wraps.
	for i := 0; i < routeLogCapacity+50; i++ {
		routeLog.record(routeLogEvent{Phase: "route", Category: "normal", Reason: "no_match", Handled: false})
	}
	stats := routeLog.stats()
	if stats.TotalRoutes != routeLogCapacity+50 {
		t.Fatalf("cumulative total=%d, want %d (ring buffer eviction should not lose counts)", stats.TotalRoutes, routeLogCapacity+50)
	}
	if stats.ByCategory["normal"] != routeLogCapacity+50 {
		t.Fatalf("cumulative by_category normal=%d, want %d", stats.ByCategory["normal"], routeLogCapacity+50)
	}
	// Ring buffer should only hold the last 500 for the log table.
	events := routeLog.snapshot()
	if len(events) != routeLogCapacity {
		t.Fatalf("ring buffer events=%d, want %d", len(events), routeLogCapacity)
	}
}

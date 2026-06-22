package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

const (
	managementResourceDashboard = "/dashboard"
	managementResourceStats     = "/api/stats"
	managementResourceLogs      = "/api/logs"
	managementResourceClear     = "/api/clear"
)

type mgmtRegistration struct {
	Routes    []mgmtRoute    `json:"routes,omitempty"`
	Resources []mgmtResource `json:"resources,omitempty"`
}

type mgmtRoute struct {
	Method      string
	Path        string
	Description string
}

type mgmtResource struct {
	Path        string
	Menu        string
	Description string
}

// managementRegistration builds the resource routes exposed by the plugin.
// All routes are browser-navigable GET resources under
// /v0/resource/plugins/selective-router/.
func managementRegistration() mgmtRegistration {
	return mgmtRegistration{
		Resources: []mgmtResource{
			{Path: managementResourceDashboard, Menu: "路由日志", Description: "Selective Router 路由触发日志与统计面板。"},
			{Path: managementResourceStats, Description: "路由统计 JSON。"},
			{Path: managementResourceLogs, Description: "路由日志 JSON。"},
			{Path: managementResourceClear, Description: "清空路由日志缓冲 (需 confirm=1)。"},
		},
	}
}

// handleManagementRequest dispatches a management.handle request to the right responder.
func handleManagementRequest(req pluginapi.ManagementRequest) pluginapi.ManagementResponse {
	path := strings.TrimSpace(req.Path)
	switch {
	case strings.HasSuffix(path, managementResourceStats):
		return serveJSON(routeLog.stats())
	case strings.HasSuffix(path, managementResourceLogs):
		events := routeLog.snapshot()
		return serveJSON(map[string]any{"events": handledRouteEvents(events)})
	case strings.HasSuffix(path, managementResourceClear):
		if !strings.EqualFold(req.Method, http.MethodGet) {
			return pluginapi.ManagementResponse{StatusCode: http.StatusMethodNotAllowed, Body: []byte(`{"ok":false,"error":"method not allowed"}`)}
		}
		if strings.TrimSpace(req.Query.Get("confirm")) != "1" {
			return pluginapi.ManagementResponse{StatusCode: http.StatusBadRequest, Body: []byte(`{"ok":false,"error":"missing confirm=1"}`)}
		}
		routeLog.clear()
		return serveJSON(map[string]bool{"ok": true})
	case strings.HasSuffix(path, managementResourceDashboard),
		strings.HasSuffix(path, "/selective-router"),
		strings.HasSuffix(path, "/selective-router/"):
		return serveHTML(dashboardHTML())
	default:
		return pluginapi.ManagementResponse{StatusCode: http.StatusNotFound, Body: []byte("not found")}
	}
}

func handledRouteEvents(events []routeLogEvent) []routeLogEvent {
	out := make([]routeLogEvent, 0, len(events))
	for _, ev := range events {
		if ev.Handled {
			out = append(out, ev)
		}
	}
	return out
}

func serveJSON(v any) pluginapi.ManagementResponse {
	raw, err := json.Marshal(v)
	if err != nil {
		return pluginapi.ManagementResponse{StatusCode: http.StatusInternalServerError, Body: []byte(`{"ok":false,"error":"marshal failed"}`)}
	}
	return pluginapi.ManagementResponse{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
		Body:       raw,
	}
}

func serveHTML(html string) pluginapi.ManagementResponse {
	return pluginapi.ManagementResponse{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       []byte(html),
	}
}

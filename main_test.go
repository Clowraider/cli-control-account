//go:build cgo

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"control-account/internal/version"
)

func TestGetDispatcher_Singleton(t *testing.T) {
	d1 := GetDispatcher()
	d2 := GetDispatcher()

	if d1 == nil || d2 == nil {
		t.Fatal("expected non-nil dispatcher")
	}

	if d1 != d2 {
		t.Errorf("expected GetDispatcher() to return identical singleton instance")
	}

	meta := d1.GetMetadata()
	if meta.ID != "control-account" {
		t.Errorf("expected plugin ID 'control-account', got %q", meta.ID)
	}

	// Test reconfiguration on singleton
	err := d1.OnPluginReconfigure(context.Background(), map[string]any{"env": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, ok := d2.GetConfigValue("env")
	if !ok || val != "test" {
		t.Errorf("expected env 'test' in singleton, got %v", val)
	}
}

func TestHandlePluginMethod(t *testing.T) {
	// 1. Test plugin.register
	rawReg, err := handlePluginMethod("plugin.register", nil)
	if err != nil {
		t.Fatalf("plugin.register failed: %v", err)
	}
	var envReg envelope
	if err := json.Unmarshal(rawReg, &envReg); err != nil || !envReg.OK {
		t.Fatalf("expected OK envelope for plugin.register, got: %s", string(rawReg))
	}
	var registration struct {
		SchemaVersion uint32 `json:"schema_version"`
		Metadata      struct {
			Version string `json:"Version"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(envReg.Result, &registration); err != nil {
		t.Fatalf("failed to unmarshal registration metadata: %v", err)
	}
	if registration.SchemaVersion != 6 {
		t.Fatalf("expected schema version 6, got %d", registration.SchemaVersion)
	}
	if registration.Metadata.Version != version.Version {
		t.Fatalf("expected plugin version %q, got %q", version.Version, registration.Metadata.Version)
	}

	// 2. Test management.register
	rawMgmt, err := handlePluginMethod("management.register", nil)
	if err != nil {
		t.Fatalf("management.register failed: %v", err)
	}
	var envMgmt envelope
	if err := json.Unmarshal(rawMgmt, &envMgmt); err != nil || !envMgmt.OK {
		t.Fatalf("expected OK envelope for management.register, got: %s", string(rawMgmt))
	}
	var regMgmt struct {
		Resources []map[string]any `json:"resources"`
		Routes    []map[string]any `json:"routes"`
	}
	if err := json.Unmarshal(envMgmt.Result, &regMgmt); err != nil {
		t.Fatalf("failed to unmarshal management registration: %v", err)
	}
	if len(regMgmt.Resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(regMgmt.Resources))
	}
	if len(regMgmt.Routes) != 10 {
		t.Errorf("expected 10 routes, got %d", len(regMgmt.Routes))
	}

	// 3. Test management.handle (GET /quota)
	reqJSON := []byte(`{"Method":"GET","Path":"/v0/resource/plugins/control-account/quota"}`)
	rawHandle, err := handlePluginMethod("management.handle", reqJSON)
	if err != nil {
		t.Fatalf("management.handle failed: %v", err)
	}
	var envHandle envelope
	if err := json.Unmarshal(rawHandle, &envHandle); err != nil || !envHandle.OK {
		t.Fatalf("expected OK envelope for management.handle, got: %s", string(rawHandle))
	}
	var respPayload managementResponsePayload
	if err := json.Unmarshal(envHandle.Result, &respPayload); err != nil {
		t.Fatalf("failed to unmarshal management response payload: %v", err)
	}
	if respPayload.StatusCode != 200 {
		t.Errorf("expected status code 200, got %d", respPayload.StatusCode)
	}

	// 4. Test usage.handle hook
	usageEvent := []byte(`{"Provider":"openai","Model":"gpt-4o","Detail":{"InputTokens":10,"OutputTokens":5}}`)
	rawUsage, err := handlePluginMethod("usage.handle", usageEvent)
	if err != nil {
		t.Fatalf("usage.handle failed: %v", err)
	}
	var envUsage envelope
	if err := json.Unmarshal(rawUsage, &envUsage); err != nil || !envUsage.OK {
		t.Fatalf("expected OK envelope for usage.handle, got: %s", string(rawUsage))
	}

	// 5. Test management.handle for Ego API
	reqEgoAPI := []byte(`{"Method":"GET","Path":"/v0/management/ego/settings"}`)
	rawEgo, err := handlePluginMethod("management.handle", reqEgoAPI)
	if err != nil {
		t.Fatalf("management.handle for Ego API failed: %v", err)
	}
	var envEgo envelope
	if err := json.Unmarshal(rawEgo, &envEgo); err != nil || !envEgo.OK {
		t.Fatalf("expected OK envelope for Ego API, got: %s", string(rawEgo))
	}
	var respEgo managementResponsePayload
	if err := json.Unmarshal(envEgo.Result, &respEgo); err != nil || respEgo.StatusCode != 200 {
		t.Fatalf("expected 200 from Ego API, got %d", respEgo.StatusCode)
	}

	// 6. Test management.handle for Ego API via authenticated route
	reqEgoStats := managementRequestPayload{
		Method: "GET",
		Path:   "/v0/management/ego/stats",
	}
	reqEgoStatsJSON, err := json.Marshal(reqEgoStats)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	rawEgoStats, err := handlePluginMethod("management.handle", reqEgoStatsJSON)
	if err != nil {
		t.Fatalf("management.handle for Ego API stats failed: %v", err)
	}
	var envEgoStats envelope
	if err := json.Unmarshal(rawEgoStats, &envEgoStats); err != nil || !envEgoStats.OK {
		t.Fatalf("expected OK envelope for Ego API stats, got: %s", string(rawEgoStats))
	}
	var respEgoStats managementResponsePayload
	if err := json.Unmarshal(envEgoStats.Result, &respEgoStats); err != nil || respEgoStats.StatusCode != 200 {
		t.Fatalf("expected 200 from Ego API stats, got %d", respEgoStats.StatusCode)
	}

	// 7. Test management.handle for Ego API /v0/management/ego/reset
	// 7a. GET /v0/management/ego/reset -> StatusCode: 405
	reqResetGet := managementRequestPayload{
		Method: "GET",
		Path:   "/v0/management/ego/reset",
	}
	reqResetGetJSON, err := json.Marshal(reqResetGet)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	rawResetGet, err := handlePluginMethod("management.handle", reqResetGetJSON)
	if err != nil {
		t.Fatalf("management.handle for GET reset failed: %v", err)
	}
	var envResetGet envelope
	if err := json.Unmarshal(rawResetGet, &envResetGet); err != nil || !envResetGet.OK {
		t.Fatalf("expected OK envelope for GET reset, got: %s", string(rawResetGet))
	}
	var respResetGet managementResponsePayload
	if err := json.Unmarshal(envResetGet.Result, &respResetGet); err != nil || respResetGet.StatusCode != 405 {
		t.Fatalf("expected 405 from GET /v0/management/ego/reset, got %d", respResetGet.StatusCode)
	}

	// 7b. POST /v0/management/ego/reset -> StatusCode: 200
	reqResetPost := managementRequestPayload{
		Method: "POST",
		Path:   "/v0/management/ego/reset",
	}
	reqResetPostJSON, err := json.Marshal(reqResetPost)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	rawResetPost, err := handlePluginMethod("management.handle", reqResetPostJSON)
	if err != nil {
		t.Fatalf("management.handle for POST reset failed: %v", err)
	}
	var envResetPost envelope
	if err := json.Unmarshal(rawResetPost, &envResetPost); err != nil || !envResetPost.OK {
		t.Fatalf("expected OK envelope for POST reset, got: %s", string(rawResetPost))
	}
	var respResetPost managementResponsePayload
	if err := json.Unmarshal(envResetPost.Result, &respResetPost); err != nil || respResetPost.StatusCode != 200 {
		t.Fatalf("expected 200 from POST /v0/management/ego/reset, got %d", respResetPost.StatusCode)
	}

	// 8. Test that calling the resource route with ?api=/reset does NOT execute reset and returns 200 with HTML content
	reqResourceReset := managementRequestPayload{
		Method: "POST",
		Path:   "/v0/resource/plugins/control-account/ego",
		Query:  map[string][]string{"api": {"/reset"}},
	}
	reqResourceResetJSON, err := json.Marshal(reqResourceReset)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	rawResourceReset, err := handlePluginMethod("management.handle", reqResourceResetJSON)
	if err != nil {
		t.Fatalf("management.handle for resource route with ?api=/reset failed: %v", err)
	}
	var envResourceReset envelope
	if err := json.Unmarshal(rawResourceReset, &envResourceReset); err != nil || !envResourceReset.OK {
		t.Fatalf("expected OK envelope for resource route with ?api=/reset, got: %s", string(rawResourceReset))
	}
	var respResourceReset managementResponsePayload
	if err := json.Unmarshal(envResourceReset.Result, &respResourceReset); err != nil {
		t.Fatalf("failed to unmarshal response payload: %v", err)
	}
	if respResourceReset.StatusCode != 200 {
		t.Errorf("expected status code 200, got %d", respResourceReset.StatusCode)
	}
	bodyBytes, err := base64.StdEncoding.DecodeString(respResourceReset.Body)
	if err != nil {
		t.Fatalf("failed to decode base64 body: %v", err)
	}
	if !strings.Contains(string(bodyBytes), "<!DOCTYPE html>") {
		t.Errorf("expected HTML content when hitting resource route with ?api=/reset")
	}

	// 9. Test unknown method
	rawUnknown, err := handlePluginMethod("unknown.event", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var envUnknown envelope
	if err := json.Unmarshal(rawUnknown, &envUnknown); err != nil {
		t.Fatalf("failed to unmarshal unknown envelope: %v", err)
	}
	if envUnknown.OK {
		t.Errorf("expected OK=false for unknown method")
	}
}

// TestHandleManagementHTTP_RejectsMalformedEgoAPIEndpoint covers authenticated
// /v0/management/ego/<endpoint> routes: the value is client controlled and used to build a
// httptest.NewRequest target, which panics by design on malformed input. A panic
// here would take the whole CLIProxyAPI process down, so every one of these must
// come back as a plain 404 instead.
func TestHandleManagementHTTP_RejectsMalformedEgoAPIEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
	}{
		{name: "space", endpoint: "a b"},
		{name: "stray percent", endpoint: "%"},
		{name: "null byte", endpoint: "\x00x"},
		{name: "newline", endpoint: "\nx"},
		{name: "traversal", endpoint: "../../etc/passwd"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqPayload := managementRequestPayload{
				Method: "GET",
				Path:   "/v0/management/ego/" + tc.endpoint,
			}
			reqJSON, err := json.Marshal(reqPayload)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			raw, err := handlePluginMethod("management.handle", reqJSON)
			if err != nil {
				t.Fatalf("management.handle for %s failed: %v", tc.endpoint, err)
			}
			var env envelope
			if err := json.Unmarshal(raw, &env); err != nil || !env.OK {
				t.Fatalf("expected OK envelope for %s, got: %s", tc.endpoint, string(raw))
			}
			var resp managementResponsePayload
			if err := json.Unmarshal(env.Result, &resp); err != nil {
				t.Fatalf("failed to unmarshal management response payload: %v", err)
			}
			if resp.StatusCode != 404 {
				t.Errorf("expected status code 404 for %s, got %d", tc.endpoint, resp.StatusCode)
			}
			bodyBytes, err := base64.StdEncoding.DecodeString(resp.Body)
			if err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if !strings.Contains(string(bodyBytes), "endpoint not found") {
				t.Errorf("expected 'endpoint not found' error message, got: %s", string(bodyBytes))
			}
		})
	}
}

// TestHandleManagementHTTP_InvalidPayload ensures a corrupt request payload is
// reported instead of silently falling through to the dashboard asset.
func TestHandleManagementHTTP_InvalidPayload(t *testing.T) {
	raw, err := handlePluginMethod("management.handle", []byte("{not json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("failed to unmarshal envelope: %v", err)
	}
	if env.OK {
		t.Fatalf("expected OK=false for malformed payload, got: %s", string(raw))
	}
	if env.Error == nil || env.Error.Code != "invalid_request" {
		t.Errorf("expected error code 'invalid_request', got: %s", string(raw))
	}
}

//go:build cgo

package main

import (
	"context"
	"encoding/json"
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
	reqEgoAPI := []byte(`{"Method":"GET","Path":"/v0/resource/plugins/control-account/ego/api/settings"}`)
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

	// 6. Test management.handle for Ego API via ?api= query parameter
	reqEgoQuery := managementRequestPayload{
		Method: "GET",
		Path:   "/v0/resource/plugins/control-account/ego",
		Query:  map[string][]string{"api": {"settings"}},
	}
	reqEgoQueryJSON, err := json.Marshal(reqEgoQuery)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	rawEgoQuery, err := handlePluginMethod("management.handle", reqEgoQueryJSON)
	if err != nil {
		t.Fatalf("management.handle for Ego API via query param failed: %v", err)
	}
	var envEgoQuery envelope
	if err := json.Unmarshal(rawEgoQuery, &envEgoQuery); err != nil || !envEgoQuery.OK {
		t.Fatalf("expected OK envelope for Ego API via query param, got: %s", string(rawEgoQuery))
	}
	var respEgoQuery managementResponsePayload
	if err := json.Unmarshal(envEgoQuery.Result, &respEgoQuery); err != nil || respEgoQuery.StatusCode != 200 {
		t.Fatalf("expected 200 from Ego API via query param, got %d", respEgoQuery.StatusCode)
	}

	// 7. Test management.handle for Ego API with encoded URI ?api=%2Fpricing.
	// The host decodes the query string before handing it to the plugin, so the
	// wire value %2Fpricing arrives here already decoded as "/pricing".
	reqEgoEncoded := managementRequestPayload{
		Method: "GET",
		Path:   "/v0/resource/plugins/control-account-linux-amd64/ego",
		Query:  map[string][]string{"api": {"/pricing"}},
	}
	reqEgoEncodedJSON, err := json.Marshal(reqEgoEncoded)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	rawEgoEncoded, err := handlePluginMethod("management.handle", reqEgoEncodedJSON)
	if err != nil {
		t.Fatalf("management.handle for encoded API failed: %v", err)
	}
	var envEgoEncoded envelope
	if err := json.Unmarshal(rawEgoEncoded, &envEgoEncoded); err != nil || !envEgoEncoded.OK {
		t.Fatalf("expected OK envelope for Ego API encoded, got: %s", string(rawEgoEncoded))
	}
	var respEgoEncoded managementResponsePayload
	if err := json.Unmarshal(envEgoEncoded.Result, &respEgoEncoded); err != nil || respEgoEncoded.StatusCode != 200 {
		t.Fatalf("expected 200 from Ego API encoded, got %d", respEgoEncoded.StatusCode)
	}

	// 8. Test unknown method
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

// TestHandleManagementHTTP_RejectsMalformedEgoAPIEndpoint covers the unauthenticated
// ?api= route: the value is fully client controlled and used to build a
// httptest.NewRequest target, which panics by design on malformed input. A panic
// here would take the whole CLIProxyAPI process down, so every one of these must
// come back as a plain 404 instead.
func TestHandleManagementHTTP_RejectsMalformedEgoAPIEndpoint(t *testing.T) {
	// Values are what the host hands over after decoding the query string.
	cases := []struct {
		name     string
		wire     string
		endpoint string
	}{
		{name: "space", wire: "?api=a+b", endpoint: "a b"},
		{name: "stray percent", wire: "?api=%25", endpoint: "%"},
		{name: "null byte", wire: "?api=%00x", endpoint: "\x00x"},
		{name: "newline", wire: "?api=%0Ax", endpoint: "\nx"},
		{name: "traversal", wire: "?api=../../etc/passwd", endpoint: "../../etc/passwd"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqPayload := managementRequestPayload{
				Method: "GET",
				Path:   "/v0/resource/plugins/control-account-linux-amd64/ego",
				Query:  map[string][]string{"api": {tc.endpoint}},
			}
			reqJSON, err := json.Marshal(reqPayload)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			raw, err := handlePluginMethod("management.handle", reqJSON)
			if err != nil {
				t.Fatalf("management.handle for %s failed: %v", tc.wire, err)
			}
			var env envelope
			if err := json.Unmarshal(raw, &env); err != nil || !env.OK {
				t.Fatalf("expected OK envelope for %s, got: %s", tc.wire, string(raw))
			}
			var resp managementResponsePayload
			if err := json.Unmarshal(env.Result, &resp); err != nil {
				t.Fatalf("failed to unmarshal management response payload: %v", err)
			}
			if resp.StatusCode != 404 {
				t.Errorf("expected status code 404 for %s, got %d", tc.wire, resp.StatusCode)
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

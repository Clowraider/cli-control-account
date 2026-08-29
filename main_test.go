package main

import (
	"context"
	"encoding/json"
	"testing"
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
		Metadata struct {
			Version string `json:"Version"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(envReg.Result, &registration); err != nil {
		t.Fatalf("failed to unmarshal registration metadata: %v", err)
	}
	if registration.Metadata.Version != "0.3.1" {
		t.Fatalf("expected plugin version 0.3.1, got %q", registration.Metadata.Version)
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

	// 4. Test unknown method
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

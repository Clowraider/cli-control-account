package main

import (
	"context"
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

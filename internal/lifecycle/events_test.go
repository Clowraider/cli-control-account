package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestDispatcher_DefaultRouteServesQuota(t *testing.T) {
	d := NewDispatcher()
	ctx := context.Background()

	router := NewMapManagementRouter()
	if err := d.OnManagementRegister(ctx, router); err != nil {
		t.Fatalf("failed to register default routes: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, DefaultManagementResourcePath, nil)
	rec := httptest.NewRecorder()

	handled := d.OnManagementHandle(ctx, req, rec)
	if !handled {
		t.Fatalf("expected request to default route %q to be handled", DefaultManagementResourcePath)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "Quota Management") {
		t.Errorf("expected default handler body to contain 'Quota Management', got: %s", rec.Body.String())
	}
}

func TestDispatcher_DefaultMetadata(t *testing.T) {
	d := NewDispatcher()
	meta := d.GetMetadata()

	if meta.ID != "control-account" {
		t.Errorf("expected ID 'control-account', got %q", meta.ID)
	}
	if meta.Name != "Control Account Quota Plugin" {
		t.Errorf("expected Name 'Control Account Quota Plugin', got %q", meta.Name)
	}
	if len(meta.Menus) != 1 {
		t.Fatalf("expected 1 menu item, got %d", len(meta.Menus))
	}
	if meta.Menus[0].Path != DefaultManagementResourcePath {
		t.Errorf("expected menu path %q, got %q", DefaultManagementResourcePath, meta.Menus[0].Path)
	}
}

func TestDispatcher_OnPluginRegister(t *testing.T) {
	d := NewDispatcher()
	ctx := context.Background()

	customReg := &PluginRegistration{
		ID:            "control-account-custom",
		Name:          "Custom Account Plugin",
		Version:       "2.1.0",
		Author:        "Custom Author",
		Description:   "Custom Description",
		SupportsOAuth: true,
		OAuthProvider: "claude",
		ConfigFields: []PluginConfigField{
			{Name: "refresh_interval", Type: "number", Required: true, Default: 60},
		},
		Menus: []PluginMenu{
			{Path: "/custom/path", Menu: "Custom Menu", Description: "Custom Menu Desc"},
		},
	}

	if err := d.OnPluginRegister(ctx, customReg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	meta := d.GetMetadata()
	if meta.ID != "control-account-custom" {
		t.Errorf("expected ID 'control-account-custom', got %q", meta.ID)
	}
	if meta.Version != "2.1.0" {
		t.Errorf("expected Version '2.1.0', got %q", meta.Version)
	}
	if !meta.SupportsOAuth || meta.OAuthProvider != "claude" {
		t.Errorf("expected OAuth support enabled for claude, got %v / %s", meta.SupportsOAuth, meta.OAuthProvider)
	}
	if len(meta.ConfigFields) != 1 || meta.ConfigFields[0].Name != "refresh_interval" {
		t.Errorf("expected config field 'refresh_interval', got %+v", meta.ConfigFields)
	}
}

func TestDispatcher_OnPluginReconfigure_Concurrency(t *testing.T) {
	d := NewDispatcher()
	ctx := context.Background()

	var wg sync.WaitGroup
	workers := 10
	iterations := 100

	// Concurrent writes
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				cfg := map[string]any{
					"worker": workerID,
					"iter":   j,
					"active": true,
				}
				if err := d.OnPluginReconfigure(ctx, cfg); err != nil {
					t.Errorf("reconfigure error: %v", err)
				}
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = d.GetConfigValue("active")
			}
		}()
	}

	wg.Wait()

	val, ok := d.GetConfigValue("active")
	if !ok || val != true {
		t.Errorf("expected active config key to be true, got %v", val)
	}
}

func TestDispatcher_OnPluginReconfigure_Nil(t *testing.T) {
	d := NewDispatcher()
	ctx := context.Background()

	_ = d.OnPluginReconfigure(ctx, map[string]any{"key": "val"})
	if _, ok := d.GetConfigValue("key"); !ok {
		t.Fatal("expected key to exist")
	}

	_ = d.OnPluginReconfigure(ctx, nil)
	if _, ok := d.GetConfigValue("key"); ok {
		t.Fatal("expected key to be cleared")
	}
}

func TestDispatcher_ManagementRegistrationAndRouting(t *testing.T) {
	d := NewDispatcher()
	ctx := context.Background()

	testHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("ok-quota"))
	})

	d.RegisterRoute(DefaultManagementResourcePath, testHandler)

	router := NewMapManagementRouter()
	if err := d.OnManagementRegister(ctx, router); err != nil {
		t.Fatalf("unexpected OnManagementRegister error: %v", err)
	}

	h, ok := router.GetHandler(DefaultManagementResourcePath)
	if !ok || h == nil {
		t.Fatalf("expected handler registered at %q", DefaultManagementResourcePath)
	}

	// Test OnManagementHandle
	req := httptest.NewRequest(http.MethodGet, DefaultManagementResourcePath, nil)
	rec := httptest.NewRecorder()

	handled := d.OnManagementHandle(ctx, req, rec)
	if !handled {
		t.Fatal("expected request to be handled")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok-quota" {
		t.Errorf("expected body 'ok-quota', got %q", rec.Body.String())
	}

	// Test unhandled route
	reqUnmapped := httptest.NewRequest(http.MethodGet, "/v0/resource/plugins/unknown", nil)
	recUnmapped := httptest.NewRecorder()
	if d.OnManagementHandle(ctx, reqUnmapped, recUnmapped) {
		t.Fatal("expected unmapped request to return false")
	}
}

func TestDispatcher_OnManagementRegister_NilRouter(t *testing.T) {
	d := NewDispatcher()
	ctx := context.Background()

	if err := d.OnManagementRegister(ctx, nil); err == nil {
		t.Fatal("expected error on nil management router")
	}
}

func TestDispatcher_DispatchEvent(t *testing.T) {
	d := NewDispatcher()
	ctx := context.Background()

	t.Run("dispatch plugin.register JSON payload", func(t *testing.T) {
		payload := `{"id":"control-account-dispatched","name":"Dispatched Name"}`
		res, err := d.DispatchEvent(ctx, EventPluginRegister, payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		meta, ok := res.(PluginRegistration)
		if !ok || meta.ID != "control-account-dispatched" {
			t.Errorf("expected updated metadata, got %+v", res)
		}
	})

	t.Run("dispatch plugin.register byte payload", func(t *testing.T) {
		payloadBytes, _ := json.Marshal(PluginRegistration{ID: "bytes-id", Name: "Bytes Name"})
		res, err := d.DispatchEvent(ctx, EventPluginRegister, payloadBytes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		meta, ok := res.(PluginRegistration)
		if !ok || meta.ID != "bytes-id" {
			t.Errorf("expected updated metadata, got %+v", res)
		}
	})

	t.Run("dispatch plugin.reconfigure JSON payload", func(t *testing.T) {
		payload := `{"refresh_interval":30,"theme":"dark"}`
		res, err := d.DispatchEvent(ctx, EventPluginReconfigure, payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != true {
			t.Errorf("expected true, got %v", res)
		}

		val, ok := d.GetConfigValue("theme")
		if !ok || val != "dark" {
			t.Errorf("expected theme 'dark', got %v", val)
		}
	})

	t.Run("dispatch management.register", func(t *testing.T) {
		router := NewMapManagementRouter()
		res, err := d.DispatchEvent(ctx, EventManagementRegister, router)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != true {
			t.Errorf("expected true, got %v", res)
		}
	})

	t.Run("dispatch management.register invalid payload", func(t *testing.T) {
		_, err := d.DispatchEvent(ctx, EventManagementRegister, "invalid-router")
		if err == nil {
			t.Fatal("expected error with invalid router payload")
		}
	})

	t.Run("dispatch unknown event", func(t *testing.T) {
		_, err := d.DispatchEvent(ctx, "unsupported.event", nil)
		if err == nil {
			t.Fatal("expected error for unsupported event")
		}
	})
}

type errRouter struct{}

func (e *errRouter) RegisterResource(path string, handler http.Handler) error {
	return errors.New("registration failed")
}

func TestDispatcher_OnManagementRegister_RouterError(t *testing.T) {
	d := NewDispatcher()
	ctx := context.Background()
	d.RegisterRoute("/test", http.NotFoundHandler())

	err := d.OnManagementRegister(ctx, &errRouter{})
	if err == nil {
		t.Fatal("expected error from failing router")
	}
}

func TestMapManagementRouter_Validation(t *testing.T) {
	router := NewMapManagementRouter()

	if err := router.RegisterResource("", http.NotFoundHandler()); err == nil {
		t.Error("expected error for empty path")
	}
	if err := router.RegisterResource("/valid", nil); err == nil {
		t.Error("expected error for nil handler")
	}
}

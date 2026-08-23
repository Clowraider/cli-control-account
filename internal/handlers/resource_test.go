package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"control-account/internal/handlers"
)

func TestResourceHandler_PathTraversalAnd404(t *testing.T) {
	handler := handlers.NewResourceHandler()

	tests := []struct {
		name string
		path string
	}{
		{
			name: "relative parent traversal",
			path: "/v0/resource/plugins/control-account/quota/../secret.txt",
		},
		{
			name: "deep root traversal",
			path: "/v0/resource/plugins/control-account/quota/../../../../etc/passwd",
		},
		{
			name: "encoded slash traversal",
			path: "/v0/resource/plugins/control-account/quota/..%2f..%2fsecret.txt",
		},
		{
			name: "unmapped asset name",
			path: "/v0/resource/plugins/control-account/quota/unmapped_image.png",
		},
		{
			name: "arbitrary subpath not in assets",
			path: "/v0/resource/plugins/control-account/quota/admin/config.json",
		},
		{
			name: "wrong base prefix path",
			path: "/v0/resource/plugins/other-plugin/quota",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rw := httptest.NewRecorder()

			handler.ServeHTTP(rw, req)

			if rw.Code != http.StatusNotFound {
				t.Fatalf("expected HTTP 404 for path %q, got %d", tc.path, rw.Code)
			}

			// Verify structured JSON error payload
			contentType := rw.Header().Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				t.Errorf("expected Content-Type application/json, got %q", contentType)
			}

			var errBody map[string]any
			if err := json.Unmarshal(rw.Body.Bytes(), &errBody); err != nil {
				t.Fatalf("expected valid JSON error body, got error: %v", err)
			}

			if _, ok := errBody["error"]; !ok {
				t.Errorf("expected 'error' field in JSON response: %v", errBody)
			}
		})
	}
}

func TestResourceHandler_ServeEmbeddedAssets(t *testing.T) {
	handler := handlers.NewResourceHandler()

	tests := []struct {
		name                string
		path                string
		expectedContentType string
		bodyContains        string
	}{
		{
			name:                "root quota dashboard",
			path:                "/v0/resource/plugins/control-account/quota",
			expectedContentType: "text/html; charset=utf-8",
			bodyContains:        "Quota Management",
		},
		{
			name:                "root quota dashboard with trailing slash",
			path:                "/v0/resource/plugins/control-account/quota/",
			expectedContentType: "text/html; charset=utf-8",
			bodyContains:        "Quota Management",
		},
		{
			name:                "explicit index.html",
			path:                "/v0/resource/plugins/control-account/quota/index.html",
			expectedContentType: "text/html; charset=utf-8",
			bodyContains:        "cards-grid",
		},
		{
			name:                "stylesheet styles.css",
			path:                "/v0/resource/plugins/control-account/quota/styles.css",
			expectedContentType: "text/css; charset=utf-8",
			bodyContains:        "dark-theme",
		},
		{
			name:                "client script app.js",
			path:                "/v0/resource/plugins/control-account/quota/app.js",
			expectedContentType: "application/javascript; charset=utf-8",
			bodyContains:        "fetchQuotaData",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rw := httptest.NewRecorder()

			handler.ServeHTTP(rw, req)

			if rw.Code != http.StatusOK {
				t.Fatalf("expected HTTP 200 for path %q, got %d (body: %s)", tt.path, rw.Code, rw.Body.String())
			}

			ct := rw.Header().Get("Content-Type")
			if ct != tt.expectedContentType {
				t.Errorf("expected Content-Type %q, got %q", tt.expectedContentType, ct)
			}

			cc := rw.Header().Get("Cache-Control")
			if cc != "no-cache, max-age=0, must-revalidate" {
				t.Errorf("expected Cache-Control 'no-cache, max-age=0, must-revalidate', got %q", cc)
			}

			body := rw.Body.String()
			if !strings.Contains(body, tt.bodyContains) {
				t.Errorf("expected body to contain %q", tt.bodyContains)
			}
		})
	}
}

func TestResourceHandler_DashboardHasNoExternalAntigravitySubscriptionDependency(t *testing.T) {
	handler := handlers.NewResourceHandler()
	req := httptest.NewRequest(http.MethodGet, "/v0/resource/plugins/control-account/quota", nil)
	rw := httptest.NewRecorder()

	handler.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("expected dashboard HTTP 200, got %d", rw.Code)
	}
	body := rw.Body.String()
	if strings.Contains(body, "AntigravitySubscription") {
		t.Fatal("served dashboard must not depend on an AntigravitySubscription global")
	}
	if strings.Contains(body, "antigravity-subscription.js") {
		t.Fatal("served dashboard must not request a second Antigravity subscription asset")
	}
	if !strings.Contains(body, "function fetchAntigravityTierSummary(authIndex)") {
		t.Fatal("served dashboard must contain its Antigravity subscription behavior")
	}
}

func TestResourceHandler_DisallowedMethods(t *testing.T) {
	handler := handlers.NewResourceHandler()
	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, m := range methods {
		t.Run(m, func(t *testing.T) {
			req := httptest.NewRequest(m, "/v0/resource/plugins/control-account/quota", nil)
			rw := httptest.NewRecorder()

			handler.ServeHTTP(rw, req)

			if rw.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected HTTP 405 Method Not Allowed for %s, got %d", m, rw.Code)
			}
		})
	}
}

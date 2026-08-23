package web_test

import (
	"strings"
	"testing"

	"control-account/internal/web"
)

func TestGetAsset_Success(t *testing.T) {
	assets := []struct {
		name         string
		expectedMime string
		contentSub   string
	}{
		{
			name:         "index.html",
			expectedMime: "text/html; charset=utf-8",
			contentSub:   "<!DOCTYPE html>",
		},
		{
			name:         "styles.css",
			expectedMime: "text/css; charset=utf-8",
			contentSub:   "dark-theme",
		},
		{
			name:         "app.js",
			expectedMime: "application/javascript; charset=utf-8",
			contentSub:   "Quota Dashboard",
		},
	}

	for _, tt := range assets {
		t.Run(tt.name, func(t *testing.T) {
			data, mimeType, err := web.GetAsset(tt.name)
			if err != nil {
				t.Fatalf("expected asset %q to load without error, got: %v", tt.name, err)
			}

			if mimeType != tt.expectedMime {
				t.Errorf("expected mime %q, got %q", tt.expectedMime, mimeType)
			}

			if !strings.Contains(string(data), tt.contentSub) {
				t.Errorf("expected asset %q to contain %q", tt.name, tt.contentSub)
			}
		})
	}
}

func TestGetAsset_PathTraversalAndInvalid(t *testing.T) {
	invalidPaths := []string{
		"../secret.txt",
		"../../etc/passwd",
		"assets/../../secret.txt",
		"",
		".",
		"/",
		"nonexistent.png",
	}

	for _, p := range invalidPaths {
		t.Run("path_"+p, func(t *testing.T) {
			data, mimeType, err := web.GetAsset(p)
			if err == nil {
				t.Fatalf("expected error for invalid path %q, got data len %d", p, len(data))
			}
			if mimeType != "" {
				t.Errorf("expected empty mimeType on error, got %q", mimeType)
			}
		})
	}
}

func TestResolveMIMEType(t *testing.T) {
	cases := []struct {
		file     string
		expected string
	}{
		{"index.html", "text/html; charset=utf-8"},
		{"main.htm", "text/html; charset=utf-8"},
		{"style.css", "text/css; charset=utf-8"},
		{"app.js", "application/javascript; charset=utf-8"},
		{"module.mjs", "application/javascript; charset=utf-8"},
		{"data.json", "application/json; charset=utf-8"},
		{"icon.svg", "image/svg+xml"},
		{"logo.png", "image/png"},
		{"photo.jpeg", "image/jpeg"},
		{"photo.jpg", "image/jpeg"},
		{"favicon.ico", "image/x-icon"},
		{"unknown.xyz123", "application/octet-stream"},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			res := web.ResolveMIMEType(tc.file)
			if res != tc.expected {
				t.Errorf("for %q expected %q, got %q", tc.file, tc.expected, res)
			}
		})
	}
}

func TestEmbeddedDashboard_ParsesCodexUsageInsteadOfHardcodingFullQuota(t *testing.T) {
	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)
	if !strings.Contains(html, "function buildCodexQuotaRows(payload)") {
		t.Fatal("expected a Codex usage normalizer function buildCodexQuotaRows(payload)")
	}
	if !strings.Contains(html, "100 - usedPercent") {
		t.Fatal("expected remaining quota to derive from 100 - usedPercent")
	}
	if !strings.Contains(html, "auth-warning-box") {
		t.Fatal("expected auth-warning-box for missing management key guidance")
	}
}

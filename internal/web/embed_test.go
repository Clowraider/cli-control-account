package web_test

import (
	"strings"
	"testing"

	"control-account/internal/version"
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

func TestGetAsset_VersionReplacement(t *testing.T) {
	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatalf("expected index.html to load without error, got: %v", err)
	}

	content := string(data)
	expectedBadge := "v" + version.Version
	if !strings.Contains(content, expectedBadge) {
		t.Errorf("expected index.html to contain version badge %q", expectedBadge)
	}
	if strings.Contains(content, "__PLUGIN_VERSION__") {
		t.Errorf("expected index.html not to contain placeholder '__PLUGIN_VERSION__'")
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

func TestEmbeddedDashboard_ContainsRealAntigravitySubscriptionDetection(t *testing.T) {
	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)
	if !strings.Contains(html, "function fetchAntigravityTierSummary(authIndex)") ||
		!strings.Contains(html, "v1internal:loadCodeAssist") {
		t.Fatal("expected dashboard to contain the Antigravity subscription transport")
	}
	if !strings.Contains(html, "parsed.currentTier") || !strings.Contains(html, "parsed.paidTier") ||
		!strings.Contains(html, "parsed.current_tier") || !strings.Contains(html, "parsed.paid_tier") {
		t.Fatal("expected dashboard to parse camelCase and snake_case subscription tiers")
	}
	if strings.Contains(html, "/v0/management/antigravity-subscription") {
		t.Fatal("dashboard must not use the obsolete Antigravity subscription endpoint")
	}
	if strings.Contains(html, "quota.plan || 'Pro'") || strings.Contains(html, "let subPlan = 'Pro'") {
		t.Fatal("dashboard must not invent a Pro plan when subscription evidence is absent")
	}
}

func TestEmbeddedDashboard_HasNoRuntimeSubscriptionAssetDependency(t *testing.T) {
	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)
	if strings.Contains(html, "AntigravitySubscription") {
		t.Fatal("dashboard must not depend on an AntigravitySubscription global")
	}
	if strings.Contains(html, "antigravity-subscription.js") {
		t.Fatal("dashboard must not load a second Antigravity subscription asset")
	}
	if _, _, err := web.GetAsset("antigravity-subscription.js"); err == nil {
		t.Fatal("separate Antigravity subscription asset must not remain embedded")
	}
}

func TestEmbeddedDashboard_CPAMCQuotaStandardsSynchronization(t *testing.T) {
	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)

	// Scope 1: Codex headers and account ID extraction
	codexRequirements := []string{
		"codex-tui/0.153.3 (Mac OS 26.5.1; arm64) iTerm.app/3.6.11 (codex-tui; 0.153.3)",
		"'OpenAI-Beta': 'codex-1'",
		"'Originator': 'Codex Desktop'",
		"function parseIdTokenPayload(value)",
		"extractCodexChatgptAccountId",
		"Chatgpt-Account-Id",
	}
	for _, req := range codexRequirements {
		if !strings.Contains(html, req) {
			t.Errorf("expected index.html to contain %q", req)
		}
	}

	// Scope 2: Codex rate limit reset credits
	creditRequirements := []string{
		"https://chatgpt.com/backend-api/wham/rate-limit-reset-credits",
		"function parseCodexResetCredits(payload)",
		"resetCredits",
		"reset-credits-badge",
	}
	for _, req := range creditRequirements {
		if !strings.Contains(html, req) {
			t.Errorf("expected index.html to contain %q", req)
		}
	}

	// Scope 3: Claude profile and specific model windows
	claudeRequirements := []string{
		"https://api.anthropic.com/api/oauth/profile",
		"function parseClaudePlan(profile)",
		"has_claude_max",
		"has_claude_pro",
		"claude_team",
		"seven_day_opus",
		"seven_day_sonnet",
		"seven_day_cowork",
		"seven_day_oauth_apps",
		"iguana_necktie",
	}
	for _, req := range claudeRequirements {
		if !strings.Contains(html, req) {
			t.Errorf("expected index.html to contain %q", req)
		}
	}

	// Scope 4: Server time clock skew synchronization
	timeRequirements := []string{
		"serverTimeOffsetMs",
		"function syncServerTimeOffset(result)",
		"Date.now() + serverTimeOffsetMs",
	}
	for _, req := range timeRequirements {
		if !strings.Contains(html, req) {
			t.Errorf("expected index.html to contain %q", req)
		}
	}
}

func TestEmbeddedDashboard_ContainsCheckUpdateButton(t *testing.T) {
	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)
	if !strings.Contains(html, "btn-check-update") {
		t.Fatal("expected index.html to contain btn-check-update")
	}
	if !strings.Contains(html, "checkForPluginUpdates") {
		t.Fatal("expected index.html to contain checkForPluginUpdates")
	}
}

func TestEmbeddedDashboard_CoreParityExtensions(t *testing.T) {
	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)

	// Parity 1: Codex interactive reset credit consumption
	codexRequirements := []string{
		"consumeCodexResetCredit",
		"createCodexRedeemRequestId",
		"https://chatgpt.com/backend-api/wham/rate-limit-reset-credits/consume",
		"btn-consume-credit",
		"/v0/management/reset-quota",
	}
	for _, req := range codexRequirements {
		if !strings.Contains(html, req) {
			t.Errorf("expected index.html to contain Codex consume requirement %q", req)
		}
	}

	// Parity 2: xAI real paid health check
	xaiRequirements := []string{
		"https://api.x.ai/v1/chat/completions",
		"grok-4.5",
	}
	for _, req := range xaiRequirements {
		if !strings.Contains(html, req) {
			t.Errorf("expected index.html to contain xAI requirement %q", req)
		}
	}

	// Parity 3: Claude extra usage overages
	claudeRequirements := []string{
		"extra_usage",
		"used_credits",
		"monthly_limit",
		"Extra Usage",
	}
	for _, req := range claudeRequirements {
		if !strings.Contains(html, req) {
			t.Errorf("expected index.html to contain Claude requirement %q", req)
		}
	}

	// Parity 4: Safe identity derivation and non-live quota handling
	identityRequirements := []string{
		"function deriveIdentity(file)",
		"isLiveQuotaProvider",
		"Standard Credential",
	}
	for _, req := range identityRequirements {
		if !strings.Contains(html, req) {
			t.Errorf("expected index.html to contain identity requirement %q", req)
		}
	}
}

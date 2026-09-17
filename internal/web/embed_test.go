package web_test

import (
	"os/exec"
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
		"codex-tui/0.154.0 (Mac OS 26.5.2; arm64) iTerm.app/3.6.11 (codex-tui; 0.154.0)",
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
		"extractCodexPlanType",
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

func TestEmbeddedDashboard_EgoTimelineChart(t *testing.T) {
	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)
	requiredElements := []string{
		"ego-timeline-card",
		"ego-chart-card",
		"ego-chart-head",
		"ego-chart-prompt-tokens",
		"ego-chart-completion-tokens",
		"ego-chart-peak-tokens",
		"ego-chart-legend",
		"ego-chart-svg",
		"ego-chart-xaxis",
		"ego-chart-tooltip",
		"ego-chart-empty",
		"renderEgoTimelineChart",
		"zeroFillEgoTimeline",
		"formatEgoTimelineLabel",
		"ego-chart-bar-col",
	}

	for _, elem := range requiredElements {
		if !strings.Contains(html, elem) {
			t.Errorf("expected index.html to contain timeline component %q", elem)
		}
	}
}

func TestEmbeddedDashboard_EgoRetailPricing(t *testing.T) {
	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)
	requiredElements := []string{
		"Estimated Retail Value",
		"ego-kpi-cost",
		"ego-kpi-prompt-cost",
		"ego-kpi-output-cost",
		"formatUSD",
		"estimated_cost_usd",
		"Est. Value",
	}

	for _, elem := range requiredElements {
		if !strings.Contains(html, elem) {
			t.Errorf("expected index.html to contain retail pricing element %q", elem)
		}
	}
}

func TestEmbeddedDashboard_JavaScriptSyntax(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available on host, skipping JS syntax check")
	}

	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)
	start := strings.Index(html, "<script>")
	end := strings.LastIndex(html, "</script>")
	if start == -1 || end == -1 || start >= end {
		t.Fatal("could not extract script block from index.html")
	}

	jsCode := html[start+len("<script>") : end]

	cmd := exec.Command(nodePath, "--check")
	cmd.Stdin = strings.NewReader(jsCode)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("embedded JavaScript has syntax error: %v\nOutput:\n%s", err, string(output))
	}
}

func TestEmbeddedDashboard_CardActivityAndQuotaRefresh(t *testing.T) {
	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)

	// refreshFileStats should query /v0/management/auth-files?name=
	if !strings.Contains(html, "function refreshFileStats") {
		t.Fatal("expected index.html to define refreshFileStats")
	}
	if !strings.Contains(html, "apiFetch(`/v0/management/auth-files?name=${encodeURIComponent(file.name)}`)") {
		t.Fatal("expected refreshFileStats to query auth-files with filename parameter")
	}

	// refreshAllStats should query auth-files and api-key-usage in bulk
	if !strings.Contains(html, "function refreshAllStats") {
		t.Fatal("expected index.html to define refreshAllStats")
	}
	if !strings.Contains(html, "apiFetch('/v0/management/api-key-usage')") {
		t.Fatal("expected refreshAllStats to query api-key-usage")
	}

	// fetchFileQuota should invoke refreshFileStats
	if !strings.Contains(html, "refreshFileStats(file)") {
		t.Fatal("expected fetchFileQuota to invoke refreshFileStats")
	}

	// refreshAll should invoke refreshAllStats
	if !strings.Contains(html, "refreshAllStats()") {
		t.Fatal("expected refreshAll to invoke refreshAllStats")
	}
}

func TestEmbeddedDashboard_RobustnessAndHardening(t *testing.T) {
	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)

	// Exactly one definition of function updateLoadedCount
	countUpdateLoadedCount := strings.Count(html, "function updateLoadedCount")
	if countUpdateLoadedCount != 1 {
		t.Fatalf("expected exactly 1 definition of function updateLoadedCount in index.html, found %d", countUpdateLoadedCount)
	}

	// AbortController and 30s timeout
	if !strings.Contains(html, "new AbortController()") {
		t.Fatal("expected index.html to contain AbortController")
	}
	if !strings.Contains(html, "30000") {
		t.Fatal("expected index.html to contain 30s timeout (30000)")
	}

	// 3-minute (180000 ms) auto-refresh interval
	if !strings.Contains(html, "180000") {
		t.Fatal("expected index.html to contain 3-minute auto-refresh interval (180000)")
	}

	// .fill.no-data class in CSS
	if !strings.Contains(html, ".fill.no-data") {
		t.Fatal("expected index.html to contain .fill.no-data CSS class")
	}

	// Issue #35: xAI token consumption warnings and unattended refresh protections
	if !strings.Contains(html, ".xai-token-warn-badge") || !strings.Contains(html, ".xai-token-warn-note") {
		t.Fatal("expected index.html to contain xAI token warning CSS classes")
	}
	if !strings.Contains(html, "document.hidden") {
		t.Fatal("expected index.html auto-refresh loop to check document.hidden")
	}
}

func TestEmbeddedDashboard_EgoAuthHandlingAndCleanups(t *testing.T) {
	data, _, err := web.GetAsset("index.html")
	if err != nil {
		t.Fatal(err)
	}

	html := string(data)

	// Auth warning banner container in Ego view
	if !strings.Contains(html, `id="ego-auth-warning"`) {
		t.Fatal("expected index.html to contain ego-auth-warning container")
	}

	// Dead helper getPluginBaseResourcePath should be removed
	if strings.Contains(html, "getPluginBaseResourcePath") {
		t.Fatal("expected dead helper getPluginBaseResourcePath to be removed from index.html")
	}

	// loadEgoOverview checks for getManagementKey and 401
	if !strings.Contains(html, "const key = getManagementKey()") {
		t.Fatal("expected loadEgoOverview to verify managementKey")
	}
}

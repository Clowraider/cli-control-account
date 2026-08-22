package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestProviderType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		expected bool
	}{
		{name: "all", provider: ProviderAll, expected: true},
		{name: "antigravity", provider: ProviderAntigravity, expected: true},
		{name: "claude", provider: ProviderClaude, expected: true},
		{name: "codex", provider: ProviderCodex, expected: true},
		{name: "kimi", provider: ProviderKimi, expected: true},
		{name: "xai", provider: ProviderXAI, expected: true},
		{name: "uppercase claude", provider: "CLAUDE", expected: true},
		{name: "unknown provider", provider: "invalid_provider", expected: false},
		{name: "empty provider", provider: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.provider.IsValid(); got != tt.expected {
				t.Errorf("ProviderType(%q).IsValid() = %v; want %v", tt.provider, got, tt.expected)
			}
		})
	}
}

func TestQuotaWindow_UsagePercentage(t *testing.T) {
	tests := []struct {
		name     string
		window   QuotaWindow
		expected float64
	}{
		{
			name:     "zero limit",
			window:   QuotaWindow{Limit: 0, Used: 10},
			expected: 0.0,
		},
		{
			name:     "negative limit",
			window:   QuotaWindow{Limit: -100, Used: 10},
			expected: 0.0,
		},
		{
			name:     "zero used",
			window:   QuotaWindow{Limit: 1000, Used: 0},
			expected: 0.0,
		},
		{
			name:     "half used",
			window:   QuotaWindow{Limit: 1000, Used: 500},
			expected: 50.0,
		},
		{
			name:     "fully used",
			window:   QuotaWindow{Limit: 1000, Used: 1000},
			expected: 100.0,
		},
		{
			name:     "overflow usage clamped",
			window:   QuotaWindow{Limit: 1000, Used: 1500},
			expected: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.window.UsagePercentage(); got != tt.expected {
				t.Errorf("UsagePercentage() = %v; want %v", got, tt.expected)
			}
		})
	}
}

func TestQuotaWindow_TimeUntilReset(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		resetAt  time.Time
		expected time.Duration
	}{
		{
			name:     "zero time",
			resetAt:  time.Time{},
			expected: 0,
		},
		{
			name:     "past time",
			resetAt:  now.Add(-10 * time.Minute),
			expected: 0,
		},
		{
			name:     "future time",
			resetAt:  now.Add(30 * time.Minute),
			expected: 30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qw := QuotaWindow{ResetAt: tt.resetAt}
			if got := qw.TimeUntilReset(now); got != tt.expected {
				t.Errorf("TimeUntilReset() = %v; want %v", got, tt.expected)
			}
		})
	}
}

func TestAccountQuota_FormattedPrefix(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		expected string
		hasPfx   bool
	}{
		{
			name:     "configured prefix",
			prefix:   "team-alpha",
			expected: "team-alpha",
			hasPfx:   true,
		},
		{
			name:     "prefix with spaces",
			prefix:   "  team-beta  ",
			expected: "team-beta",
			hasPfx:   true,
		},
		{
			name:     "empty prefix fallback",
			prefix:   "",
			expected: "-",
			hasPfx:   false,
		},
		{
			name:     "whitespace only fallback",
			prefix:   "   ",
			expected: "-",
			hasPfx:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aq := AccountQuota{Prefix: tt.prefix}
			if got := aq.FormattedPrefix(); got != tt.expected {
				t.Errorf("FormattedPrefix() = %q; want %q", got, tt.expected)
			}
			if got := aq.HasPrefix(); got != tt.hasPfx {
				t.Errorf("HasPrefix() = %v; want %v", got, tt.hasPfx)
			}
		})
	}
}

func TestQuotaSummary_FilterByProvider(t *testing.T) {
	accounts := []AccountQuota{
		{ID: "1", Name: "acc-claude", Provider: ProviderClaude, Prefix: "team-1"},
		{ID: "2", Name: "acc-codex", Provider: ProviderCodex, Prefix: "team-2"},
		{ID: "3", Name: "acc-kimi", Provider: ProviderKimi, Prefix: ""},
		{ID: "4", Name: "acc-xai", Provider: ProviderXAI, Prefix: "x-team"},
	}
	summary := QuotaSummary{
		TotalAccounts:  len(accounts),
		ActiveAccounts: len(accounts),
		Accounts:      accounts,
		GeneratedAt:   time.Now(),
	}

	t.Run("filter by claude", func(t *testing.T) {
		res := summary.FilterByProvider(ProviderClaude)
		if len(res) != 1 || res[0].ID != "1" {
			t.Fatalf("expected 1 claude account, got %v", res)
		}
	})

	t.Run("filter by all returns all", func(t *testing.T) {
		res := summary.FilterByProvider(ProviderAll)
		if len(res) != 4 {
			t.Fatalf("expected 4 accounts for 'all', got %d", len(res))
		}
	})

	t.Run("filter by empty string returns all", func(t *testing.T) {
		res := summary.FilterByProvider("")
		if len(res) != 4 {
			t.Fatalf("expected 4 accounts for empty string, got %d", len(res))
		}
	})

	t.Run("filter by antigravity returns empty", func(t *testing.T) {
		res := summary.FilterByProvider(ProviderAntigravity)
		if len(res) != 0 {
			t.Fatalf("expected 0 accounts for antigravity, got %d", len(res))
		}
	})
}

func TestQuotaSummary_Serialization(t *testing.T) {
	summary := QuotaSummary{
		TotalAccounts:  1,
		ActiveAccounts: 1,
		Accounts: []AccountQuota{
			{
				ID:       "acc-1",
				Name:     "Test Account",
				Provider: ProviderClaude,
				Prefix:   "prefix-1",
				Status:   "active",
				Priority: 10,
				Quota: QuotaWindow{
					Limit:     1000,
					Used:      250,
					Remaining: 750,
				},
			},
		},
		GeneratedAt: time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC),
	}

	data, err := summary.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error marshaling to JSON: %v", err)
	}

	var parsed QuotaSummary
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unexpected error unmarshaling JSON: %v", err)
	}

	if parsed.TotalAccounts != 1 || len(parsed.Accounts) != 1 {
		t.Errorf("unmarshaled mismatch: %+v", parsed)
	}
	if parsed.Accounts[0].FormattedPrefix() != "prefix-1" {
		t.Errorf("expected prefix-1, got %q", parsed.Accounts[0].FormattedPrefix())
	}
}

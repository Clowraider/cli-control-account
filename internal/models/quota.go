package models

import (
	"encoding/json"
	"strings"
	"time"
)

// ProviderType represents supported LLM/OAuth providers.
type ProviderType string

const (
	ProviderAll         ProviderType = "all"
	ProviderAntigravity ProviderType = "antigravity"
	ProviderClaude      ProviderType = "claude"
	ProviderCodex       ProviderType = "codex"
	ProviderKimi        ProviderType = "kimi"
	ProviderXAI         ProviderType = "xai"
)

// IsValidProvider validates whether a provider string is supported.
func (p ProviderType) IsValid() bool {
	switch strings.ToLower(string(p)) {
	case string(ProviderAll),
		string(ProviderAntigravity),
		string(ProviderClaude),
		string(ProviderCodex),
		string(ProviderKimi),
		string(ProviderXAI):
		return true
	default:
		return false
	}
}

// QuotaWindow represents usage and limits within a specific window.
type QuotaWindow struct {
	Limit         int64     `json:"limit"`
	Used          int64     `json:"used"`
	Remaining     int64     `json:"remaining"`
	ResetAt       time.Time `json:"reset_at"`
	WindowSeconds int64     `json:"window_seconds,omitempty"`
}

// UsagePercentage calculates the percentage of quota consumed (0.0 to 100.0).
func (qw QuotaWindow) UsagePercentage() float64 {
	if qw.Limit <= 0 {
		return 0.0
	}
	if qw.Used <= 0 {
		return 0.0
	}
	pct := (float64(qw.Used) / float64(qw.Limit)) * 100.0
	if pct > 100.0 {
		return 100.0
	}
	return pct
}

// TimeUntilReset returns remaining duration until quota reset.
func (qw QuotaWindow) TimeUntilReset(now time.Time) time.Duration {
	if qw.ResetAt.IsZero() || qw.ResetAt.Before(now) {
		return 0
	}
	return qw.ResetAt.Sub(now)
}

// AccountQuota represents an account's metadata and quota state.
type AccountQuota struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Provider  ProviderType `json:"provider"`
	Prefix    string       `json:"prefix"`
	Status    string       `json:"status"`
	Priority  int          `json:"priority"`
	Quota     QuotaWindow  `json:"quota"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// FormattedPrefix returns the prefix or "-" when empty to ensure uniform alignment.
func (aq AccountQuota) FormattedPrefix() string {
	trimmed := strings.TrimSpace(aq.Prefix)
	if trimmed == "" {
		return "-"
	}
	return trimmed
}

// HasPrefix returns true if the account has a non-empty prefix configured.
func (aq AccountQuota) HasPrefix() bool {
	return strings.TrimSpace(aq.Prefix) != ""
}

// SanitizePrefix trims whitespace and replaces internal whitespace sequences with single hyphens.
func SanitizePrefix(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}
	fields := strings.Fields(trimmed)
	return strings.Join(fields, "-")
}

// QuotaSummary represents an aggregated response of multiple accounts and their quota.
type QuotaSummary struct {
	TotalAccounts int            `json:"total_accounts"`
	ActiveAccounts int           `json:"active_accounts"`
	Accounts      []AccountQuota `json:"accounts"`
	GeneratedAt   time.Time      `json:"generated_at"`
}

// FilterByProvider returns a new slice containing only accounts matching the given provider.
// If provider is "all" or empty, all accounts are returned.
func (qs QuotaSummary) FilterByProvider(provider ProviderType) []AccountQuota {
	if provider == "" || strings.EqualFold(string(provider), string(ProviderAll)) {
		out := make([]AccountQuota, len(qs.Accounts))
		copy(out, qs.Accounts)
		return out
	}

	var filtered []AccountQuota
	for _, acc := range qs.Accounts {
		if strings.EqualFold(string(acc.Provider), string(provider)) {
			filtered = append(filtered, acc)
		}
	}
	return filtered
}

// ToJSON serializes the QuotaSummary into JSON bytes.
func (qs QuotaSummary) ToJSON() ([]byte, error) {
	return json.Marshal(qs)
}

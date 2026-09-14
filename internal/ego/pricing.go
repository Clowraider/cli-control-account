package ego

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	urlPkg "net/url"
	"regexp"
	"strings"
	"time"
)

// DefaultLiteLLMPricingURL is the default remote upstream endpoint for LiteLLM pricing metadata.
const DefaultLiteLLMPricingURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

// DefaultCuratedPricing provides offline embedded rates per token in USD for mainstream models.
var DefaultCuratedPricing = []ModelPricing{
	// Claude Models (Anthropic)
	{Model: "claude-3-7-sonnet", InputCostPerToken: 0.000003, OutputCostPerToken: 0.000015, CacheReadInputTokenCost: 0.0000003},
	{Model: "claude-3.7-sonnet", InputCostPerToken: 0.000003, OutputCostPerToken: 0.000015, CacheReadInputTokenCost: 0.0000003},
	{Model: "claude-3-5-sonnet", InputCostPerToken: 0.000003, OutputCostPerToken: 0.000015, CacheReadInputTokenCost: 0.0000003},
	{Model: "claude-3.5-sonnet", InputCostPerToken: 0.000003, OutputCostPerToken: 0.000015, CacheReadInputTokenCost: 0.0000003},
	{Model: "claude-3-5-haiku", InputCostPerToken: 0.0000008, OutputCostPerToken: 0.000004, CacheReadInputTokenCost: 0.00000008},
	{Model: "claude-3.5-haiku", InputCostPerToken: 0.0000008, OutputCostPerToken: 0.000004, CacheReadInputTokenCost: 0.00000008},
	{Model: "claude-3-opus", InputCostPerToken: 0.000015, OutputCostPerToken: 0.000075, CacheReadInputTokenCost: 0.0000015},

	// OpenAI Models
	{Model: "gpt-4o", InputCostPerToken: 0.0000025, OutputCostPerToken: 0.00001, CacheReadInputTokenCost: 0.00000125},
	{Model: "gpt-4o-mini", InputCostPerToken: 0.00000015, OutputCostPerToken: 0.0000006, CacheReadInputTokenCost: 0.000000075},
	{Model: "o1", InputCostPerToken: 0.000015, OutputCostPerToken: 0.00006, CacheReadInputTokenCost: 0.0000075},
	{Model: "o1-preview", InputCostPerToken: 0.000015, OutputCostPerToken: 0.00006, CacheReadInputTokenCost: 0.0000075},
	{Model: "o1-mini", InputCostPerToken: 0.0000011, OutputCostPerToken: 0.0000044, CacheReadInputTokenCost: 0.00000055},
	{Model: "o3-mini", InputCostPerToken: 0.0000011, OutputCostPerToken: 0.0000044, CacheReadInputTokenCost: 0.00000055},

	// Gemini Models (Google)
	{Model: "gemini-3.8-flash-high", InputCostPerToken: 0.0000001, OutputCostPerToken: 0.0000004, CacheReadInputTokenCost: 0.000000025},
	{Model: "gemini-3.8-flash", InputCostPerToken: 0.0000001, OutputCostPerToken: 0.0000004, CacheReadInputTokenCost: 0.000000025},
	{Model: "gemini-3.8-pro", InputCostPerToken: 0.00000125, OutputCostPerToken: 0.000005, CacheReadInputTokenCost: 0.0000003125},
	{Model: "gemini-2.5-flash", InputCostPerToken: 0.0000001, OutputCostPerToken: 0.0000004, CacheReadInputTokenCost: 0.000000025},
	{Model: "gemini-2.5-pro", InputCostPerToken: 0.00000125, OutputCostPerToken: 0.000005, CacheReadInputTokenCost: 0.0000003125},
	{Model: "gemini-2.0-flash-thinking", InputCostPerToken: 0.0000001, OutputCostPerToken: 0.0000004, CacheReadInputTokenCost: 0.000000025},
	{Model: "gemini-2.0-flash", InputCostPerToken: 0.0000001, OutputCostPerToken: 0.0000004, CacheReadInputTokenCost: 0.000000025},
	{Model: "gemini-2.0-flash-exp", InputCostPerToken: 0.0000001, OutputCostPerToken: 0.0000004, CacheReadInputTokenCost: 0.000000025},
	{Model: "gemini-2.0-pro", InputCostPerToken: 0.00000125, OutputCostPerToken: 0.000005, CacheReadInputTokenCost: 0.0000003125},
	{Model: "gemini-2.0-pro-exp", InputCostPerToken: 0.00000125, OutputCostPerToken: 0.000005, CacheReadInputTokenCost: 0.0000003125},
	{Model: "gemini-1.5-pro", InputCostPerToken: 0.00000125, OutputCostPerToken: 0.000005, CacheReadInputTokenCost: 0.0000003125},
	{Model: "gemini-1.5-flash", InputCostPerToken: 0.000000075, OutputCostPerToken: 0.0000003, CacheReadInputTokenCost: 0.00000001875},
	{Model: "gemini-flash", InputCostPerToken: 0.0000001, OutputCostPerToken: 0.0000004, CacheReadInputTokenCost: 0.000000025},
	{Model: "gemini-pro", InputCostPerToken: 0.00000125, OutputCostPerToken: 0.000005, CacheReadInputTokenCost: 0.0000003125},

	// DeepSeek Models
	{Model: "deepseek-chat", InputCostPerToken: 0.00000014, OutputCostPerToken: 0.00000028, CacheReadInputTokenCost: 0.000000014},
	{Model: "deepseek-v3", InputCostPerToken: 0.00000014, OutputCostPerToken: 0.00000028, CacheReadInputTokenCost: 0.000000014},
	{Model: "deepseek-reasoner", InputCostPerToken: 0.00000055, OutputCostPerToken: 0.00000219, CacheReadInputTokenCost: 0.00000014},
	{Model: "deepseek-r1", InputCostPerToken: 0.00000055, OutputCostPerToken: 0.00000219, CacheReadInputTokenCost: 0.00000014},
}

var dateSuffixRegex = regexp.MustCompile(`-(?:20\d{2}[01]\d[0-3]\d|20\d{2}-[01]\d-[0-3]\d|\d{4})$`)

// NormalizeModelName normalizes a model string by removing provider prefixes,
// tag/version specifiers, and lowercasing.
func NormalizeModelName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	if s == "" {
		return ""
	}

	// Strip provider prefix if present (e.g. anthropic/claude-3-7-sonnet or models/gemini-2.0-flash)
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}

	// Strip version/tag suffixes after : or @ if present
	if idx := strings.IndexAny(s, "@:"); idx >= 0 {
		s = s[:idx]
	}

	return strings.TrimSpace(s)
}

// FindModelPricing looks up the pricing for a model in the pricing map, performing
// exact match, dot-to-dash normalization, date suffix stripping, and longest-prefix fuzzy matching.
func FindModelPricing(pricingMap map[string]ModelPricing, modelName string) (ModelPricing, bool) {
	if len(pricingMap) == 0 {
		return ModelPricing{Model: modelName}, false
	}

	normalized := NormalizeModelName(modelName)
	if normalized == "" {
		return ModelPricing{Model: modelName}, false
	}

	// 1. Direct exact match on normalized name
	if p, ok := pricingMap[normalized]; ok {
		return p, true
	}

	// 2. Try normalized with dots converted to hyphens
	withHyphens := strings.ReplaceAll(normalized, ".", "-")
	if p, ok := pricingMap[withHyphens]; ok {
		return p, true
	}

	// 3. Try stripped date suffix (e.g. claude-3-7-sonnet-20250219 -> claude-3-7-sonnet)
	dateStripped := dateSuffixRegex.ReplaceAllString(normalized, "")
	if dateStripped != normalized {
		if p, ok := pricingMap[dateStripped]; ok {
			return p, true
		}
		dateStrippedHyphens := strings.ReplaceAll(dateStripped, ".", "-")
		if p, ok := pricingMap[dateStrippedHyphens]; ok {
			return p, true
		}
	}

	// 4. Fuzzy longest prefix match
	// Find all entries in pricingMap that are a prefix of normalized (or dateStripped)
	var bestMatch ModelPricing
	var bestLen int

	searchTargets := []string{normalized, withHyphens}
	if dateStripped != normalized {
		searchTargets = append(searchTargets, dateStripped)
	}

	for k, v := range pricingMap {
		kNorm := strings.ToLower(k)
		for _, target := range searchTargets {
			if strings.HasPrefix(target, kNorm) {
				if len(kNorm) > bestLen {
					bestLen = len(kNorm)
					bestMatch = v
				}
			}
		}
	}

	if bestLen > 0 {
		return bestMatch, true
	}

	// 5. Family / keyword fallback heuristic for variants or unlisted sub-models
	target := normalized
	switch {
	case strings.Contains(target, "gemini"):
		if strings.Contains(target, "flash") {
			if p, ok := pricingMap["gemini-3.8-flash-high"]; ok {
				return p, true
			}
			if p, ok := pricingMap["gemini-flash"]; ok {
				return p, true
			}
			if p, ok := pricingMap["gemini-2.0-flash"]; ok {
				return p, true
			}
		}
		if strings.Contains(target, "pro") {
			if p, ok := pricingMap["gemini-pro"]; ok {
				return p, true
			}
			if p, ok := pricingMap["gemini-2.0-pro"]; ok {
				return p, true
			}
		}
	case strings.Contains(target, "claude"):
		if strings.Contains(target, "sonnet") {
			if p, ok := pricingMap["claude-3-5-sonnet"]; ok {
				return p, true
			}
			if p, ok := pricingMap["claude-3-7-sonnet"]; ok {
				return p, true
			}
		} else if strings.Contains(target, "haiku") {
			if p, ok := pricingMap["claude-3-5-haiku"]; ok {
				return p, true
			}
		} else if strings.Contains(target, "opus") {
			if p, ok := pricingMap["claude-3-opus"]; ok {
				return p, true
			}
		}
	case strings.Contains(target, "gpt-4o-mini"):
		if p, ok := pricingMap["gpt-4o-mini"]; ok {
			return p, true
		}
	case strings.Contains(target, "gpt-4o"):
		if p, ok := pricingMap["gpt-4o"]; ok {
			return p, true
		}
	case strings.Contains(target, "o3"):
		if p, ok := pricingMap["o3-mini"]; ok {
			return p, true
		}
	case strings.Contains(target, "o1"):
		if p, ok := pricingMap["o1"]; ok {
			return p, true
		}
	case strings.Contains(target, "deepseek"):
		if strings.Contains(target, "r1") || strings.Contains(target, "reasoner") {
			if p, ok := pricingMap["deepseek-r1"]; ok {
				return p, true
			}
		}
		if p, ok := pricingMap["deepseek-v3"]; ok {
			return p, true
		}
	}

	return ModelPricing{Model: modelName}, false
}

// CalculateTokenCost calculates the retail cost equivalent in USD according to the formula:
// (prompt - cached) * in_rate + cached * cache_rate + completion * out_rate
func CalculateTokenCost(pricing ModelPricing, promptTokens, completionTokens, reasoningTokens, cachedTokens int64) (totalCost, promptCost, outputCost float64) {
	uncachedPrompt := promptTokens - cachedTokens
	if uncachedPrompt < 0 {
		uncachedPrompt = 0
	}

	cacheRate := pricing.CacheReadInputTokenCost
	// If cache read rate is not explicitly set but input cost is, default cache rate is input cost
	if cacheRate == 0 && pricing.InputCostPerToken > 0 {
		cacheRate = pricing.InputCostPerToken
	}

	outTokens := completionTokens
	if outTokens < reasoningTokens {
		outTokens = reasoningTokens
	}

	promptCost = (float64(uncachedPrompt) * pricing.InputCostPerToken) + (float64(cachedTokens) * cacheRate)
	outputCost = float64(outTokens) * pricing.OutputCostPerToken
	totalCost = promptCost + outputCost

	return totalCost, promptCost, outputCost
}

// liteLLMRawEntry matches an entry in LiteLLM's model_prices_and_context_window.json.
type liteLLMRawEntry struct {
	InputCostPerToken       *float64 `json:"input_cost_per_token"`
	OutputCostPerToken      *float64 `json:"output_cost_per_token"`
	CacheReadInputTokenCost *float64 `json:"cache_read_input_token_cost"`
}

// ParseLiteLLMPricing parses JSON from LiteLLM's model_prices_and_context_window.json payload.
func ParseLiteLLMPricing(data []byte) ([]ModelPricing, error) {
	var rawMap map[string]liteLLMRawEntry
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return nil, fmt.Errorf("failed to parse litellm pricing json: %w", err)
	}

	now := time.Now().UnixMilli()
	results := make([]ModelPricing, 0, len(rawMap))

	for modelKey, entry := range rawMap {
		if modelKey == "sample_spec" || strings.TrimSpace(modelKey) == "" {
			continue
		}

		var inRate, outRate, cacheRate float64
		if entry.InputCostPerToken != nil {
			inRate = *entry.InputCostPerToken
		}
		if entry.OutputCostPerToken != nil {
			outRate = *entry.OutputCostPerToken
		}
		if entry.CacheReadInputTokenCost != nil {
			cacheRate = *entry.CacheReadInputTokenCost
		}

		if inRate == 0 && outRate == 0 && cacheRate == 0 {
			continue
		}

		results = append(results, ModelPricing{
			Model:                   strings.ToLower(strings.TrimSpace(modelKey)),
			InputCostPerToken:       inRate,
			OutputCostPerToken:      outRate,
			CacheReadInputTokenCost: cacheRate,
			UpdatedAt:               now,
		})
	}

	return results, nil
}

// SyncLiteLLMPricing fetches model pricing from LiteLLM's repository and upserts into SQLite storage.
func (s *Storage) SyncLiteLLMPricing(url string) (int, error) {
	if url == "" {
		url = DefaultLiteLLMPricingURL
	}

	parsedURL, err := urlPkg.Parse(url)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return 0, fmt.Errorf("invalid pricing sync url: %s", url)
	}

	hostname := strings.ToLower(parsedURL.Hostname())
	allowedHost := hostname == "raw.githubusercontent.com" || hostname == "githubusercontent.com" || hostname == "localhost" || hostname == "127.0.0.1"
	if !allowedHost {
		return 0, fmt.Errorf("untrusted pricing host: %s", hostname)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch pricing from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status fetching pricing: %d %s", resp.StatusCode, resp.Status)
	}

	// Read with limit (25 MB max)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 25*1024*1024))
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %w", err)
	}

	pricingEntries, err := ParseLiteLLMPricing(body)
	if err != nil {
		return 0, err
	}

	if err := s.UpsertPricingBatch(pricingEntries); err != nil {
		return 0, fmt.Errorf("failed to save pricing entries to database: %w", err)
	}

	return len(pricingEntries), nil
}

// SyncLiteLLMPricing is a package-level helper that invokes SyncLiteLLMPricing on the active worker storage.
func SyncLiteLLMPricing(url string) (int, error) {
	w := GetWorker()
	if w == nil || w.Storage() == nil {
		return 0, fmt.Errorf("ego storage is not initialized")
	}
	return w.Storage().SyncLiteLLMPricing(url)
}

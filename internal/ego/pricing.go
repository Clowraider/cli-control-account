package ego

import (
	"regexp"
	"strings"
)

// DefaultCuratedPricing provides offline embedded rates per token in USD for mainstream models.
var DefaultCuratedPricing = []ModelPricing{
	// Claude Models (Anthropic) - Cache creation is 1.25x input token cost
	{Model: "claude-3-7-sonnet", InputCostPerToken: 0.000003, OutputCostPerToken: 0.000015, CacheReadInputTokenCost: 0.0000003, CacheCreationInputTokenCost: 0.00000375},
	{Model: "claude-3.7-sonnet", InputCostPerToken: 0.000003, OutputCostPerToken: 0.000015, CacheReadInputTokenCost: 0.0000003, CacheCreationInputTokenCost: 0.00000375},
	{Model: "claude-3-5-sonnet", InputCostPerToken: 0.000003, OutputCostPerToken: 0.000015, CacheReadInputTokenCost: 0.0000003, CacheCreationInputTokenCost: 0.00000375},
	{Model: "claude-3.5-sonnet", InputCostPerToken: 0.000003, OutputCostPerToken: 0.000015, CacheReadInputTokenCost: 0.0000003, CacheCreationInputTokenCost: 0.00000375},
	{Model: "claude-3-5-haiku", InputCostPerToken: 0.0000008, OutputCostPerToken: 0.000004, CacheReadInputTokenCost: 0.00000008, CacheCreationInputTokenCost: 0.000001},
	{Model: "claude-3.5-haiku", InputCostPerToken: 0.0000008, OutputCostPerToken: 0.000004, CacheReadInputTokenCost: 0.00000008, CacheCreationInputTokenCost: 0.000001},
	{Model: "claude-3-opus", InputCostPerToken: 0.000015, OutputCostPerToken: 0.000075, CacheReadInputTokenCost: 0.0000015, CacheCreationInputTokenCost: 0.00001875},

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

// TokenAccountingSemantics defines the formula semantics used by different AI providers.
type TokenAccountingSemantics uint8

const (
	SemanticsSubset TokenAccountingSemantics = iota
	SemanticsIndependent
	SemanticsSeparateReasoning
)

// ResolveTokenSemantics classifies provider accounting semantics according to CLIProxyAPI standards.
// Order of precedence matches CLIProxyAPI host runtime:
// 1. OpenAI compatibility (via executorType or provider prefix/name) -> SemanticsSubset
// 2. Claude / Anthropic -> SemanticsIndependent
// 3. Gemini / Vertex / Interaction -> SemanticsSeparateReasoning
// 4. Default -> SemanticsSubset
func ResolveTokenSemantics(provider string, executorType ...string) TokenAccountingSemantics {
	p := strings.ToLower(strings.TrimSpace(provider))
	var cleanExec string
	if len(executorType) > 0 {
		cleanExec = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(executorType[0])), "-", "")
	}

	// 1. OpenAI-compatibility check (by executor or provider name/prefix)
	if strings.Contains(cleanExec, "openaicompat") ||
		p == "openai-compatibility" || strings.HasPrefix(p, "openai-compatible-") {
		return SemanticsSubset
	}

	// 2. Claude / Anthropic check (independent counters)
	if strings.Contains(p, "claude") || strings.Contains(p, "anthropic") {
		return SemanticsIndependent
	}

	// 3. Gemini / Vertex / Interaction family (separate reasoning, prompt includes cache)
	for _, marker := range []string{"gemini", "aistudio", "antigravity", "vertex", "interaction"} {
		if strings.Contains(p, marker) {
			return SemanticsSeparateReasoning
		}
	}

	// 4. Default subset (OpenAI, DeepSeek, etc.)
	return SemanticsSubset
}

// CalculateTokenCost calculates the retail cost equivalent in USD according to provider semantics:
// - independent (Claude): input is already uncached (not subtracting cached), reasoning is additive to completion, cache creation is 1.25x.
// - separateReasoning (Gemini): input includes cache (subtracts cached), reasoning is additive to completion.
// - subset (OpenAI/others): input includes cache, reasoning is a subset of completion (max).
func CalculateTokenCost(pricing ModelPricing, provider string, promptTokens, completionTokens, reasoningTokens, cacheReadTokens, cacheCreationTokens int64) (totalCost, promptCost, outputCost float64) {
	semantics := ResolveTokenSemantics(provider)

	var uncachedPrompt int64
	switch semantics {
	case SemanticsIndependent:
		uncachedPrompt = promptTokens
		if uncachedPrompt < 0 {
			uncachedPrompt = 0
		}
	default:
		uncachedPrompt = promptTokens - (cacheReadTokens + cacheCreationTokens)
		if uncachedPrompt < 0 {
			uncachedPrompt = 0
		}
	}

	cacheReadRate := pricing.CacheReadInputTokenCost
	if cacheReadRate == 0 && pricing.InputCostPerToken > 0 {
		cacheReadRate = pricing.InputCostPerToken
	}

	cacheCreationRate := pricing.CacheCreationInputTokenCost
	if cacheCreationRate == 0 && pricing.InputCostPerToken > 0 {
		if semantics == SemanticsIndependent {
			cacheCreationRate = pricing.InputCostPerToken * 1.25
		} else {
			cacheCreationRate = pricing.InputCostPerToken
		}
	}

	var outTokens int64
	switch semantics {
	case SemanticsIndependent, SemanticsSeparateReasoning:
		outTokens = completionTokens + reasoningTokens
	default: // Subset
		outTokens = completionTokens
		if outTokens < reasoningTokens {
			outTokens = reasoningTokens
		}
	}

	promptCost = (float64(uncachedPrompt) * pricing.InputCostPerToken) +
		(float64(cacheReadTokens) * cacheReadRate) +
		(float64(cacheCreationTokens) * cacheCreationRate)
	outputCost = float64(outTokens) * pricing.OutputCostPerToken
	totalCost = promptCost + outputCost

	return totalCost, promptCost, outputCost
}

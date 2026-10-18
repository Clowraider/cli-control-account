package ego

import "strings"

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

	// xAI (Grok) Models
	{Model: "grok-2", InputCostPerToken: 0.000002, OutputCostPerToken: 0.00001},
	{Model: "grok-2-mini", InputCostPerToken: 0.0000002, OutputCostPerToken: 0.000001},
	{Model: "grok-beta", InputCostPerToken: 0.000005, OutputCostPerToken: 0.000015},

	// Kimi / Moonshot Models
	{Model: "moonshot-v1-8k", InputCostPerToken: 0.0000002, OutputCostPerToken: 0.000002},
	{Model: "moonshot-v1-32k", InputCostPerToken: 0.000001, OutputCostPerToken: 0.000003},
	{Model: "moonshot-v1-128k", InputCostPerToken: 0.000002, OutputCostPerToken: 0.000005},
	{Model: "kimi-k1.5", InputCostPerToken: 0.0000006, OutputCostPerToken: 0.000003},

	// Qwen & Mistral
	{Model: "qwen-2.5-coder-32b", InputCostPerToken: 0.0000008, OutputCostPerToken: 0.000002},
	{Model: "mistral-large", InputCostPerToken: 0.000002, OutputCostPerToken: 0.000006},
	{Model: "codestral", InputCostPerToken: 0.0000003, OutputCostPerToken: 0.0000009},
}

// NormalizeModelName normalizes only case and surrounding whitespace. Provider,
// version, date and tag components are part of the source key, not aliases.
func NormalizeModelName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// FindModelPricing preserves exact source keys before conservative same-name
// normalization. Unknown versions and provider aliases deliberately stay unpriced.
func FindModelPricing(pricingMap map[string]ModelPricing, modelName string) (ModelPricing, bool) {
	if p, ok := pricingMap[modelName]; ok {
		return p, true
	}
	if normalized := NormalizeModelName(modelName); normalized != "" {
		if p, ok := pricingMap[normalized]; ok {
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
// - independent (Claude): input is already uncached (not subtracting cached), reasoning is a subset of completion (output_tokens already includes thinking), cache creation is 1.25x.
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
	case SemanticsIndependent:
		// Anthropic reports thinking as a subset of output_tokens, so completion
		// already contains the reasoning tokens (CLIProxyAPI host, see
		// internal/runtime/executor/helps/usage_helpers.go: "raw output_tokens
		// already includes thinking"). Adding reasoning here would bill it twice.
		outTokens = completionTokens
	case SemanticsSeparateReasoning:
		// Gemini family: candidatesTokenCount and thoughtsTokenCount are disjoint,
		// so reasoning really is additive to completion.
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

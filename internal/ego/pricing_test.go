package ego

import (
	"testing"
)

func TestPricing_ExactKeysDoNotInventAliases(t *testing.T) {
	prices := map[string]ModelPricing{
		"claude-5":          {Model: "claude-5", InputCostPerToken: 1},
		"gemini-3.8-flash":  {Model: "gemini-3.8-flash", InputCostPerToken: 2},
		"gemini-pro":        {Model: "gemini-pro", InputCostPerToken: 3},
		"vendor/model:paid": {Model: "vendor/model:paid", InputCostPerToken: 4},
		"model":             {Model: "model", InputCostPerToken: 5},
	}
	for _, name := range []string{"claude-5.5", "claude-5-20990101", "gemini-3.8-flash-high", "gemini-pro-agent", "other/model"} {
		t.Run(name, func(t *testing.T) {
			if price, ok := FindModelPricing(prices, name); ok {
				t.Fatalf("unverified model %q resolved to %q", name, price.Model)
			}
		})
	}
	for _, name := range []string{"claude-5", "vendor/model:paid"} {
		t.Run(name, func(t *testing.T) {
			price, ok := FindModelPricing(prices, name)
			if !ok || price.Model != name {
				t.Fatalf("exact source key %q not preserved: %+v, found=%v", name, price, ok)
			}
		})
	}
}

func TestPricing_EmbeddedRatesAndFuzzyMatching(t *testing.T) {
	// Build map from DefaultCuratedPricing
	pricingMap := make(map[string]ModelPricing)
	for _, p := range DefaultCuratedPricing {
		pricingMap[p.Model] = p
	}

	requiredModels := []string{
		"claude-3-7-sonnet",
		"claude-3-5-sonnet",
		"claude-3-5-haiku",
		"claude-3-opus",
		"gpt-4o",
		"gpt-4o-mini",
		"o1",
		"o3-mini",
		"gemini-3.8-flash-high",
		"gemini-2.0-flash",
		"gemini-2.0-pro",
		"gemini-1.5-pro",
		"gemini-1.5-flash",
		"deepseek-chat",
		"deepseek-reasoner",
	}

	for _, model := range requiredModels {
		p, ok := FindModelPricing(pricingMap, model)
		if !ok {
			t.Errorf("expected embedded model %q to be found", model)
		}
		if p.InputCostPerToken <= 0 {
			t.Errorf("model %q has zero or negative input cost: %f", model, p.InputCostPerToken)
		}
		if p.OutputCostPerToken <= 0 {
			t.Errorf("model %q has zero or negative output cost: %f", model, p.OutputCostPerToken)
		}
	}

}

func TestPricing_CalculateCostFormula(t *testing.T) {
	pricing := ModelPricing{
		Model:                        "test-model",
		InputCostPerToken:            0.000003,   // $3 per 1M tokens
		OutputCostPerToken:           0.000015,   // $15 per 1M tokens
		CacheReadInputTokenCost:      0.0000003,  // $0.30 per 1M tokens
		CacheCreationInputTokenCost: 0.00000375, // $3.75 per 1M tokens (1.25x)
	}

	tests := []struct {
		name                string
		provider            string
		promptTokens        int64
		completionTokens    int64
		reasoningTokens     int64
		cacheReadTokens     int64
		cacheCreationTokens int64
		expectedTotal       float64
		expectedPrompt      float64
		expectedOutput      float64
	}{
		{
			name:                "openai subset: pure uncached prompt and completion",
			provider:            "openai",
			promptTokens:        1000,
			completionTokens:    500,
			reasoningTokens:     0,
			cacheReadTokens:     0,
			cacheCreationTokens: 0,
			// prompt: 1000 * 0.000003 = 0.003
			// output: 500 * 0.000015 = 0.0075
			// total: 0.0105
			expectedPrompt: 0.003,
			expectedOutput: 0.0075,
			expectedTotal:  0.0105,
		},
		{
			name:                "openai subset: with cached tokens and reasoning",
			provider:            "openai",
			promptTokens:        1000,
			completionTokens:    500,
			reasoningTokens:     200,
			cacheReadTokens:     400,
			cacheCreationTokens: 0,
			// uncached: 600 * 0.000003 = 0.0018
			// cached:   400 * 0.0000003 = 0.00012
			// promptCost = 0.00192
			// output: 500 * 0.000015 = 0.0075 (completion tokens subsume reasoning tokens in subset)
			// totalCost = 0.00942
			expectedPrompt: 0.00192,
			expectedOutput: 0.0075,
			expectedTotal:  0.00942,
		},
		{
			name:                "claude independent: input is uncached count, cache read and creation are additive",
			provider:            "claude",
			promptTokens:        1000, // already uncached
			completionTokens:    500,  // already includes the 200 thinking tokens
			reasoningTokens:     200,  // subset of completion, not additive
			cacheReadTokens:     50000,
			cacheCreationTokens: 1000,
			// uncached: 1000 * 0.000003 = 0.003
			// cacheRead: 50000 * 0.0000003 = 0.015
			// cacheCreation: 1000 * 0.00000375 = 0.00375
			// promptCost = 0.02175
			// output: 500 * 0.000015 = 0.0075 (thinking is already inside output_tokens)
			// totalCost = 0.02925
			expectedPrompt: 0.02175,
			expectedOutput: 0.0075,
			expectedTotal:  0.02925,
		},
		{
			// Regression for the thinking double-count: payload copied verbatim from the
			// CLIProxyAPI host test TestParseClaudeUsagePreservesThinkingTokensAsReasoningSubset
			// (internal/runtime/executor/helps/usage_helpers_test.go), itself sanitized from
			// real Anthropic logs:
			//   {"input_tokens":2,"cache_creation_input_tokens":831,"cache_read_input_tokens":44225,
			//    "output_tokens":244,"output_tokens_details":{"thinking_tokens":40}}
			// The host asserts OutputTokens=244 / ReasoningTokens=40, i.e. thinking is a subset.
			name:                "claude independent: thinking tokens are not billed twice (host payload)",
			provider:            "claude",
			promptTokens:        2,
			completionTokens:    244,
			reasoningTokens:     40,
			cacheReadTokens:     44225,
			cacheCreationTokens: 831,
			// uncached: 2 * 0.000003 = 0.000006
			// cacheRead: 44225 * 0.0000003 = 0.0132675
			// cacheCreation: 831 * 0.00000375 = 0.00311625
			// promptCost = 0.01638975
			// output: 244 * 0.000015 = 0.00366 (NOT (244 + 40) * 0.000015 = 0.00426)
			// totalCost = 0.02004975
			expectedPrompt: 0.01638975,
			expectedOutput: 0.00366,
			expectedTotal:  0.02004975,
		},
		{
			name:                "gemini separateReasoning: prompt includes cache, reasoning is additive",
			provider:            "gemini",
			promptTokens:        1000, // includes cache
			completionTokens:    500,
			reasoningTokens:     300, // additive to completion
			cacheReadTokens:     400,
			cacheCreationTokens: 0,
			// uncached: (1000 - 400) * 0.000003 = 0.0018
			// cacheRead: 400 * 0.0000003 = 0.00012
			// promptCost = 0.00192
			// output: (500 + 300) * 0.000015 = 0.012
			// totalCost = 0.01392
			expectedPrompt: 0.00192,
			expectedOutput: 0.012,
			expectedTotal:  0.01392,
		},
		{
			name:                "openai-compatible-claude: must resolve to subset semantics even though provider contains claude",
			provider:            "openai-compatible-claude",
			promptTokens:        1000,
			completionTokens:    500,
			reasoningTokens:     200,
			cacheReadTokens:     400,
			cacheCreationTokens: 0,
			// Under subset (OpenAI compat):
			// uncached: 600 * 0.000003 = 0.0018
			// cached:   400 * 0.0000003 = 0.00012
			// promptCost = 0.00192
			// output: 500 * 0.000015 = 0.0075 (completion tokens subsume reasoning tokens)
			// totalCost = 0.00942
			expectedPrompt: 0.00192,
			expectedOutput: 0.0075,
			expectedTotal:  0.00942,
		},
		{
			name:                "cached tokens exceed prompt tokens clamp in subset",
			provider:            "openai",
			promptTokens:        100,
			completionTokens:    50,
			reasoningTokens:     0,
			cacheReadTokens:     150, // exceeds promptTokens, uncached = 0
			cacheCreationTokens: 0,
			// prompt: 0 * 0.000003 + 150 * 0.0000003 = 0.000045
			// output: 50 * 0.000015 = 0.00075
			// total: 0.000795
			expectedPrompt: 0.000045,
			expectedOutput: 0.00075,
			expectedTotal:  0.000795,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tot, inC, outC := CalculateTokenCost(pricing, tt.provider, tt.promptTokens, tt.completionTokens, tt.reasoningTokens, tt.cacheReadTokens, tt.cacheCreationTokens)
			const tolerance = 1e-9
			if diff := tot - tt.expectedTotal; diff > tolerance || diff < -tolerance {
				t.Errorf("expected total %f, got %f (diff: %e)", tt.expectedTotal, tot, diff)
			}
			if diff := inC - tt.expectedPrompt; diff > tolerance || diff < -tolerance {
				t.Errorf("expected prompt cost %f, got %f (diff: %e)", tt.expectedPrompt, inC, diff)
			}
			if diff := outC - tt.expectedOutput; diff > tolerance || diff < -tolerance {
				t.Errorf("expected output cost %f, got %f (diff: %e)", tt.expectedOutput, outC, diff)
			}
		})
	}
}

func TestPricing_ResolveTokenSemantics(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		executorType string
		expected     TokenAccountingSemantics
	}{
		{
			name:     "openai-compatible-claude resolves to subset",
			provider: "openai-compatible-claude",
			expected: SemanticsSubset,
		},
		{
			name:     "openai-compatible-gemini resolves to subset",
			provider: "openai-compatible-gemini",
			expected: SemanticsSubset,
		},
		{
			name:     "openai-compatibility resolves to subset",
			provider: "openai-compatibility",
			expected: SemanticsSubset,
		},
		{
			name:         "openaicompatexecutor with custom provider resolves to subset",
			provider:     "custom-proxy",
			executorType: "openaicompatexecutor",
			expected:     SemanticsSubset,
		},
		{
			name:         "openaicompat executor with claude in name resolves to subset",
			provider:     "my-claude-endpoint",
			executorType: "openai-compat-executor",
			expected:     SemanticsSubset,
		},
		{
			name:     "native claude resolves to independent",
			provider: "claude",
			expected: SemanticsIndependent,
		},
		{
			name:     "anthropic resolves to independent",
			provider: "anthropic",
			expected: SemanticsIndependent,
		},
		{
			name:     "native gemini resolves to separate reasoning",
			provider: "gemini",
			expected: SemanticsSeparateReasoning,
		},
		{
			name:     "vertex resolves to separate reasoning",
			provider: "vertex",
			expected: SemanticsSeparateReasoning,
		},
		{
			name:     "aistudio resolves to separate reasoning",
			provider: "aistudio",
			expected: SemanticsSeparateReasoning,
		},
		{
			name:     "antigravity resolves to separate reasoning",
			provider: "antigravity",
			expected: SemanticsSeparateReasoning,
		},
		{
			name:     "interaction resolves to separate reasoning",
			provider: "interaction",
			expected: SemanticsSeparateReasoning,
		},
		{
			name:     "interactions endpoint resolves to separate reasoning",
			provider: "interactions",
			expected: SemanticsSeparateReasoning,
		},
		{
			name:     "openai standard resolves to subset",
			provider: "openai",
			expected: SemanticsSubset,
		},
		{
			name:     "deepseek standard resolves to subset",
			provider: "deepseek",
			expected: SemanticsSubset,
		},
		{
			name:     "unknown provider resolves to subset default",
			provider: "unknown",
			expected: SemanticsSubset,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got TokenAccountingSemantics
			if tt.executorType != "" {
				got = ResolveTokenSemantics(tt.provider, tt.executorType)
			} else {
				got = ResolveTokenSemantics(tt.provider)
			}
			if got != tt.expected {
				t.Errorf("ResolveTokenSemantics(%q, %q) = %d, want %d", tt.provider, tt.executorType, got, tt.expected)
			}
		})
	}
}

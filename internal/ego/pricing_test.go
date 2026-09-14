package ego

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

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

	// Fuzzy matching test cases
	fuzzyCases := []struct {
		name          string
		inputModel    string
		expectedModel string
	}{
		{
			name:          "dated claude snapshot",
			inputModel:    "claude-3-7-sonnet-20250219",
			expectedModel: "claude-3-7-sonnet",
		},
		{
			name:          "anthropic provider prefix",
			inputModel:    "anthropic/claude-3-7-sonnet",
			expectedModel: "claude-3-7-sonnet",
		},
		{
			name:          "dotted version format",
			inputModel:    "claude-3.7-sonnet",
			expectedModel: "claude-3-7-sonnet",
		},
		{
			name:          "dated 3.5 sonnet",
			inputModel:    "claude-3-5-sonnet-20241022",
			expectedModel: "claude-3-5-sonnet",
		},
		{
			name:          "longest prefix gpt-4o-mini",
			inputModel:    "openai/gpt-4o-mini-2024-07-18",
			expectedModel: "gpt-4o-mini",
		},
		{
			name:          "google models prefix",
			inputModel:    "models/gemini-2.0-flash",
			expectedModel: "gemini-2.0-flash",
		},
		{
			name:          "deepseek prefix",
			inputModel:    "deepseek/deepseek-chat",
			expectedModel: "deepseek-chat",
		},
		{
			name:          "deepseek reasoning alias",
			inputModel:    "deepseek-r1",
			expectedModel: "deepseek-r1",
		},
		{
			name:          "gemini-3.8-flash-high exact",
			inputModel:    "gemini-3.8-flash-high",
			expectedModel: "gemini-3.8-flash-high",
		},
		{
			name:          "gemini flash variant with prefix",
			inputModel:    "antigravity/gemini-3.8-flash-high",
			expectedModel: "gemini-3.8-flash-high",
		},
		{
			name:          "tag suffix with colon",
			inputModel:    "gpt-4o:latest",
			expectedModel: "gpt-4o",
		},
	}

	for _, tc := range fuzzyCases {
		t.Run(tc.name, func(t *testing.T) {
			p, ok := FindModelPricing(pricingMap, tc.inputModel)
			if !ok {
				t.Fatalf("expected fuzzy match for %q, got none", tc.inputModel)
			}
			if p.Model != tc.expectedModel && stringsTrimDash(p.Model) != stringsTrimDash(tc.expectedModel) {
				t.Errorf("for input %q: expected match %q, got %q", tc.inputModel, tc.expectedModel, p.Model)
			}
		})
	}
}

func stringsTrimDash(s string) string {
	res := make([]rune, 0, len(s))
	for _, r := range s {
		if r != '.' && r != '-' {
			res = append(res, r)
		}
	}
	return string(res)
}

func TestPricing_CalculateCostFormula(t *testing.T) {
	// Formula: (prompt - cached) * in_rate + cached * cache_rate + (completion + reasoning) * out_rate
	pricing := ModelPricing{
		Model:                   "test-model",
		InputCostPerToken:       0.000003,  // $3 per 1M tokens
		OutputCostPerToken:      0.000015,  // $15 per 1M tokens
		CacheReadInputTokenCost: 0.0000003, // $0.30 per 1M tokens
	}

	tests := []struct {
		name             string
		promptTokens     int64
		completionTokens int64
		reasoningTokens  int64
		cachedTokens     int64
		expectedTotal    float64
		expectedPrompt   float64
		expectedOutput   float64
	}{
		{
			name:             "pure uncached prompt and completion",
			promptTokens:     1000,
			completionTokens: 500,
			reasoningTokens:  0,
			cachedTokens:     0,
			// prompt: 1000 * 0.000003 = 0.003
			// output: 500 * 0.000015 = 0.0075
			// total: 0.0105
			expectedPrompt: 0.003,
			expectedOutput: 0.0075,
			expectedTotal:  0.0105,
		},
		{
			name:             "with cached tokens and reasoning",
			promptTokens:     1000,
			completionTokens: 500,
			reasoningTokens:  200,
			cachedTokens:     400,
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
			name:             "cached tokens exceed prompt tokens clamp",
			promptTokens:     100,
			completionTokens: 50,
			reasoningTokens:  0,
			cachedTokens:     150, // exceeds promptTokens, uncached = 0
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
			tot, inC, outC := CalculateTokenCost(pricing, tt.promptTokens, tt.completionTokens, tt.reasoningTokens, tt.cachedTokens)
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

func TestPricing_ParseLiteLLMPricing(t *testing.T) {
	mockPayload := []byte(`{
		"sample_spec": {
			"max_tokens": 1000,
			"input_cost_per_token": 0.000001,
			"output_cost_per_token": 0.000002
		},
		"gpt-4o": {
			"input_cost_per_token": 0.0000025,
			"output_cost_per_token": 0.00001,
			"cache_read_input_token_cost": 0.00000125
		},
		"claude-3-5-sonnet-20241022": {
			"input_cost_per_token": 0.000003,
			"output_cost_per_token": 0.000015,
			"cache_read_input_token_cost": 0.0000003
		},
		"free-model": {
			"input_cost_per_token": 0,
			"output_cost_per_token": 0
		}
	}`)

	entries, err := ParseLiteLLMPricing(mockPayload)
	if err != nil {
		t.Fatalf("ParseLiteLLMPricing failed: %v", err)
	}

	entryMap := make(map[string]ModelPricing)
	for _, e := range entries {
		entryMap[e.Model] = e
	}

	// sample_spec and free-model (0 cost) should be skipped
	if _, ok := entryMap["sample_spec"]; ok {
		t.Errorf("expected sample_spec to be skipped")
	}
	if _, ok := entryMap["free-model"]; ok {
		t.Errorf("expected free-model with zero cost to be skipped")
	}

	// gpt-4o
	if gpt, ok := entryMap["gpt-4o"]; !ok {
		t.Errorf("expected gpt-4o to be parsed")
	} else {
		if gpt.InputCostPerToken != 0.0000025 || gpt.OutputCostPerToken != 0.00001 || gpt.CacheReadInputTokenCost != 0.00000125 {
			t.Errorf("unexpected gpt-4o rates: %+v", gpt)
		}
	}

	// claude-3-5-sonnet-20241022
	if claude, ok := entryMap["claude-3-5-sonnet-20241022"]; !ok {
		t.Errorf("expected claude-3-5-sonnet-20241022 to be parsed")
	} else {
		if claude.InputCostPerToken != 0.000003 || claude.OutputCostPerToken != 0.000015 {
			t.Errorf("unexpected claude rates: %+v", claude)
		}
	}
}

func TestPricing_SyncLiteLLMPricingWithServer(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		_, _ = rw.Write([]byte(`{
			"test-remote-model": {
				"input_cost_per_token": 0.000005,
				"output_cost_per_token": 0.00002,
				"cache_read_input_token_cost": 0.000001
			}
		}`))
	}))
	defer mockServer.Close()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_pricing_sync.db")

	storage, err := OpenStorage(dbPath)
	if err != nil {
		t.Fatalf("OpenStorage failed: %v", err)
	}
	defer storage.Close()

	count, err := storage.SyncLiteLLMPricing(mockServer.URL)
	if err != nil {
		t.Fatalf("SyncLiteLLMPricing failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 model synced, got %d", count)
	}

	p := storage.GetPricing("test-remote-model")
	if p.Model != "test-remote-model" || p.InputCostPerToken != 0.000005 {
		t.Errorf("Expected synced model to have input cost 0.000005, got %+v", p)
	}
}

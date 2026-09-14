package ego

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestWorker_IngestionAndFlush(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_worker.db")

	storage, err := OpenStorage(dbPath)
	if err != nil {
		t.Fatalf("OpenStorage failed: %v", err)
	}

	worker := NewWorker(storage, 100, 5, 50*time.Millisecond)
	worker.Start()
	defer worker.Stop()

	// Feed 3 events
	for i := 0; i < 3; i++ {
		payload := RawUsageRecord{
			Provider:    "openai",
			Model:       "gpt-4o",
			AuthID:      "test-account",
			Latency:     int64(250 * time.Millisecond),
			RequestedAt: time.Now(),
			Failed:      false,
			Detail: RawUsageDetail{
				InputTokens:  100,
				OutputTokens: 50,
				TotalTokens:  150,
			},
		}
		raw, _ := json.Marshal(payload)
		worker.Record(raw)
	}

	// Wait for ticker flush
	time.Sleep(120 * time.Millisecond)

	count, err := storage.GetTotalRecords()
	if err != nil {
		t.Fatalf("GetTotalRecords failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 records in DB, got %d", count)
	}

	// Test disabling worker
	worker.SetEnabled(false)
	worker.Record([]byte(`{"Provider":"claude"}`))
	time.Sleep(80 * time.Millisecond)

	count, _ = storage.GetTotalRecords()
	if count != 3 {
		t.Errorf("Expected still 3 records when disabled, got %d", count)
	}
}

func TestHandler_Endpoints(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_handler.db")

	storage, err := OpenStorage(dbPath)
	if err != nil {
		t.Fatalf("OpenStorage failed: %v", err)
	}

	worker := NewWorker(storage, 100, 10, 50*time.Millisecond)
	worker.Start()
	defer worker.Stop()

	handler := NewHandler(worker)

	// Pre-insert an event
	now := time.Now().UnixMilli()
	_ = storage.InsertBatch([]EgoEvent{
		{
			Timestamp:        now,
			Provider:         "gemini",
			Model:            "gemini-2.5-pro",
			Account:          "gem-1",
			PromptTokens:     500,
			CompletionTokens: 200,
			TotalTokens:      700,
			LatencyMs:        300,
			Status:           "success",
		},
	})

	// 1. GET /stats
	req := httptest.NewRequest(http.MethodGet, "/ego/api/stats?range=24h", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rw.Code)
	}
	var summary SummaryStats
	if err := json.Unmarshal(rw.Body.Bytes(), &summary); err != nil {
		t.Fatalf("Failed to parse summary response: %v", err)
	}
	if summary.TotalRequests != 1 || summary.TotalTokens != 700 {
		t.Errorf("Unexpected summary: %+v", summary)
	}

	// 2. GET /settings
	req = httptest.NewRequest(http.MethodGet, "/ego/api/settings", nil)
	rw = httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("Expected 200 for settings, got %d", rw.Code)
	}
	var settings EgoSettings
	if err := json.Unmarshal(rw.Body.Bytes(), &settings); err != nil {
		t.Fatalf("Failed to parse settings: %v", err)
	}
	if !settings.Enabled || settings.TotalRecords != 1 {
		t.Errorf("Unexpected settings: %+v", settings)
	}

	// 3. POST /settings (toggle off)
	body := bytes.NewBufferString(`{"enabled": false}`)
	req = httptest.NewRequest(http.MethodPost, "/ego/api/settings", body)
	rw = httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("Expected 200 for POST settings, got %d", rw.Code)
	}
	if worker.IsEnabled() {
		t.Errorf("Expected worker to be disabled")
	}

	// 4. POST /reset
	req = httptest.NewRequest(http.MethodPost, "/ego/api/reset", nil)
	rw = httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("Expected 200 for reset, got %d", rw.Code)
	}

	count, _ := storage.GetTotalRecords()
	if count != 0 {
		t.Errorf("Expected 0 records after reset, got %d", count)
	}

	// 5. GET /pricing
	req = httptest.NewRequest(http.MethodGet, "/ego/api/pricing", nil)
	rw = httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("Expected 200 for GET /pricing, got %d", rw.Code)
	}
	var pricingResp map[string]any
	if err := json.Unmarshal(rw.Body.Bytes(), &pricingResp); err != nil {
		t.Fatalf("Failed to parse pricing response: %v", err)
	}
	if total, ok := pricingResp["total"].(float64); !ok || total == 0 {
		t.Errorf("Expected positive pricing total, got %v", pricingResp["total"])
	}

	// 6. Method restrictions on endpoints
	// 6a. GET /reset -> 405
	req = httptest.NewRequest(http.MethodGet, "/ego/api/reset", nil)
	rw = httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET /reset, got %d", rw.Code)
	}

	// 6b. GET /prune -> 405
	req = httptest.NewRequest(http.MethodGet, "/ego/api/prune", nil)
	rw = httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET /prune, got %d", rw.Code)
	}

	// 6c. POST /prune -> 200
	req = httptest.NewRequest(http.MethodPost, "/ego/api/prune", bytes.NewBufferString(`{"days": 30}`))
	rw = httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Errorf("Expected 200 for POST /prune, got %d", rw.Code)
	}

	// 6d. DELETE /settings -> 405
	req = httptest.NewRequest(http.MethodDelete, "/ego/api/settings", nil)
	rw = httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for DELETE /settings, got %d", rw.Code)
	}

	// 6e. POST /stats -> 405
	req = httptest.NewRequest(http.MethodPost, "/ego/api/stats", nil)
	rw = httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for POST /stats, got %d", rw.Code)
	}

	// 6f. GET /unknown -> 404
	req = httptest.NewRequest(http.MethodGet, "/ego/api/unknown", nil)
	rw = httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	if rw.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for GET /unknown, got %d", rw.Code)
	}
}

func TestTransformToEgoEvent_CacheTokenAttribution(t *testing.T) {
	// CachedTokens on the stored event is what storage feeds to CalculateTokenCost as
	// cacheReadTokens, so it must carry cache *reads* only: cache creation is billed
	// separately from CacheCreationTokens.
	tests := []struct {
		name                  string
		provider              string
		detail                RawUsageDetail
		expectedCached        int64
		expectedCacheCreation int64
	}{
		{
			name:     "anthropic cold request: host echoes cache creation into CachedTokens",
			provider: "claude",
			detail: RawUsageDetail{
				InputTokens:  1000,
				OutputTokens: 500,
				// CLIProxyAPI fills CachedTokens with CacheReadTokens and then, when there
				// are no cache reads, overwrites it with CacheCreationTokens. Both fields
				// end up carrying the same 50000 tokens, which were written once.
				CachedTokens:        50000,
				CacheReadTokens:     0,
				CacheCreationTokens: 50000,
				TotalTokens:         51500,
			},
			expectedCached:        0, // reads only; the echo must not be billed a second time
			expectedCacheCreation: 50000,
		},
		{
			name:     "anthropic warm request: pure cache read",
			provider: "claude",
			detail: RawUsageDetail{
				InputTokens:         1000,
				OutputTokens:        500,
				CachedTokens:        50000,
				CacheReadTokens:     50000,
				CacheCreationTokens: 0,
				TotalTokens:         51500,
			},
			expectedCached:        50000,
			expectedCacheCreation: 0,
		},
		{
			name:     "anthropic mixed request: cache read plus partial refresh",
			provider: "claude",
			detail: RawUsageDetail{
				InputTokens:         1000,
				OutputTokens:        500,
				CachedTokens:        20000,
				CacheReadTokens:     20000,
				CacheCreationTokens: 5000,
				TotalTokens:         26500,
			},
			expectedCached:        20000,
			expectedCacheCreation: 5000,
		},
		{
			name:     "subset provider reporting only the legacy CachedTokens field",
			provider: "openai",
			detail: RawUsageDetail{
				InputTokens:         40000,
				OutputTokens:        500,
				CachedTokens:        30000,
				CacheReadTokens:     0,
				CacheCreationTokens: 0,
				TotalTokens:         40500,
			},
			expectedCached:        30000, // no cache creation to confuse it with: keep the fallback
			expectedCacheCreation: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := transformToEgoEvent(RawUsageRecord{
				Provider:    tt.provider,
				Model:       "test-model",
				AuthID:      "test-account",
				RequestedAt: time.Now(),
				Detail:      tt.detail,
			})

			if event.CachedTokens != tt.expectedCached {
				t.Errorf("expected cached tokens %d, got %d", tt.expectedCached, event.CachedTokens)
			}
			if event.CacheCreationTokens != tt.expectedCacheCreation {
				t.Errorf("expected cache creation tokens %d, got %d", tt.expectedCacheCreation, event.CacheCreationTokens)
			}
		})
	}
}

func TestTransformToEgoEvent_ColdCacheCostIsNotDoubleCounted(t *testing.T) {
	pricing := ModelPricing{
		Model:                       "claude-3-5-sonnet",
		InputCostPerToken:           0.000003,   // $3 per 1M tokens
		OutputCostPerToken:          0.000015,   // $15 per 1M tokens
		CacheReadInputTokenCost:     0.0000003,  // $0.30 per 1M tokens
		CacheCreationInputTokenCost: 0.00000375, // $3.75 per 1M tokens (1.25x)
	}

	event := transformToEgoEvent(RawUsageRecord{
		Provider:    "claude",
		Model:       "claude-3-5-sonnet",
		AuthID:      "test-account",
		RequestedAt: time.Now(),
		Detail: RawUsageDetail{
			InputTokens:         1000,
			CachedTokens:        50000, // host echo of CacheCreationTokens on a cold request
			CacheReadTokens:     0,
			CacheCreationTokens: 50000,
			TotalTokens:         51000,
		},
	})

	// Storage passes EgoEvent.CachedTokens to CalculateTokenCost as cacheReadTokens.
	_, promptCost, _ := CalculateTokenCost(
		pricing,
		"claude",
		event.PromptTokens,
		event.CompletionTokens,
		event.ReasoningTokens,
		event.CachedTokens,
		event.CacheCreationTokens,
	)

	// uncached:      1000 * 0.000003    = 0.003
	// cacheCreation: 50000 * 0.00000375 = 0.1875
	// promptCost = 0.1905
	// Billing the echo as a cache read as well would add 50000 * 0.0000003 = 0.015 -> 0.2055 (+7.9%).
	const expectedPrompt = 0.1905
	const tolerance = 1e-9
	if diff := promptCost - expectedPrompt; diff > tolerance || diff < -tolerance {
		t.Errorf("expected prompt cost %f, got %f (diff: %e)", expectedPrompt, promptCost, diff)
	}
}

func TestTransformToEgoEvent_TotalTokensFallback(t *testing.T) {
	tests := []struct {
		name          string
		provider      string
		executorType  string
		detail        RawUsageDetail
		expectedTotal int64
	}{
		{
			name:     "subset (openai): completion subsumes reasoning when total is 0",
			provider: "openai",
			detail: RawUsageDetail{
				InputTokens:     1000,
				OutputTokens:    500,
				ReasoningTokens: 200,
				TotalTokens:     0,
			},
			expectedTotal: 1500, // 1000 + 500
		},
		{
			name:     "subset (openai): reasoning exceeds completion when total is 0",
			provider: "openai",
			detail: RawUsageDetail{
				InputTokens:     1000,
				OutputTokens:    100,
				ReasoningTokens: 300,
				TotalTokens:     0,
			},
			expectedTotal: 1300, // 1000 + 300
		},
		{
			name:     "separateReasoning (gemini): reasoning is additive to completion when total is 0",
			provider: "gemini",
			detail: RawUsageDetail{
				InputTokens:     1000,
				OutputTokens:    500,
				ReasoningTokens: 200,
				TotalTokens:     0,
			},
			expectedTotal: 1700, // 1000 + 500 + 200
		},
		{
			name:     "independent (claude): prompt + cached + cacheCreation + completion + reasoning when total is 0",
			provider: "claude",
			detail: RawUsageDetail{
				InputTokens:         1000,
				OutputTokens:        500,
				ReasoningTokens:     200,
				CacheReadTokens:     5000,
				CacheCreationTokens: 2000,
				TotalTokens:         0,
			},
			expectedTotal: 8700, // 1000 + 5000 + 2000 + 500 + 200
		},
		{
			name:         "openai-compatible-claude: uses subset semantics when total is 0",
			provider:     "openai-compatible-claude",
			executorType: "openaicompatexecutor",
			detail: RawUsageDetail{
				InputTokens:     1000,
				OutputTokens:    500,
				ReasoningTokens: 200,
				TotalTokens:     0,
			},
			expectedTotal: 1500, // 1000 + 500 (reasoning not double-counted)
		},
		{
			name:     "explicit total from upstream is preserved as-is",
			provider: "openai",
			detail: RawUsageDetail{
				InputTokens:     1000,
				OutputTokens:    500,
				ReasoningTokens: 200,
				TotalTokens:     9999,
			},
			expectedTotal: 9999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := transformToEgoEvent(RawUsageRecord{
				Provider:     tt.provider,
				ExecutorType: tt.executorType,
				Model:        "test-model",
				AuthID:       "test-account",
				RequestedAt:  time.Now(),
				Detail:       tt.detail,
			})

			if event.TotalTokens != tt.expectedTotal {
				t.Errorf("expected total tokens %d, got %d", tt.expectedTotal, event.TotalTokens)
			}
		})
	}
}

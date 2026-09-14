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
}

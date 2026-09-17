package ego

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStorage_InsertAndQuery(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_ego.db")

	storage, err := OpenStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to open storage: %v", err)
	}
	defer storage.Close()

	now := time.Now().UnixMilli()
	events := []EgoEvent{
		{
			Timestamp:        now - 1000,
			Provider:         "openai",
			Model:            "gpt-4o",
			Account:          "acc-1",
			PromptTokens:     100,
			CompletionTokens: 50,
			ReasoningTokens:  0,
			CachedTokens:     20,
			TotalTokens:      150,
			LatencyMs:        450,
			Status:           "success",
		},
		{
			Timestamp:        now - 500,
			Provider:         "claude",
			Model:            "claude-3-5-sonnet",
			Account:          "acc-2",
			PromptTokens:     200,
			CompletionTokens: 100,
			ReasoningTokens:  30,
			CachedTokens:     50,
			TotalTokens:      300,
			LatencyMs:        800,
			Status:           "success",
		},
		{
			Timestamp:        now,
			Provider:         "openai",
			Model:            "gpt-4o",
			Account:          "acc-1",
			PromptTokens:     50,
			CompletionTokens: 0,
			TotalTokens:      50,
			LatencyMs:        120,
			Status:           "failed",
		},
	}

	if err := storage.InsertBatch(events); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	// 1. Check summary
	summary, err := storage.GetSummary("24h", "all")
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if summary.TotalRequests != 3 {
		t.Errorf("Expected 3 requests, got %d", summary.TotalRequests)
	}
	if summary.TotalSuccess != 2 {
		t.Errorf("Expected 2 successes, got %d", summary.TotalSuccess)
	}
	if summary.TotalFailed != 1 {
		t.Errorf("Expected 1 failed, got %d", summary.TotalFailed)
	}
	if summary.TotalTokens != 500 {
		t.Errorf("Expected 500 total tokens, got %d", summary.TotalTokens)
	}
	if summary.EstimatedCostUSD <= 0 {
		t.Errorf("Expected positive EstimatedCostUSD, got %f", summary.EstimatedCostUSD)
	}
	if summary.InputCostUSD <= 0 {
		t.Errorf("Expected positive InputCostUSD, got %f", summary.InputCostUSD)
	}
	if summary.OutputCostUSD <= 0 {
		t.Errorf("Expected positive OutputCostUSD, got %f", summary.OutputCostUSD)
	}

	// 2. Check summary with provider filter
	claudeSummary, err := storage.GetSummary("24h", "claude")
	if err != nil {
		t.Fatalf("GetSummary for claude failed: %v", err)
	}
	if claudeSummary.TotalRequests != 1 {
		t.Errorf("Expected 1 claude request, got %d", claudeSummary.TotalRequests)
	}
	if claudeSummary.TotalTokens != 300 {
		t.Errorf("Expected 300 claude tokens, got %d", claudeSummary.TotalTokens)
	}
	if claudeSummary.EstimatedCostUSD <= 0 {
		t.Errorf("Expected positive claude EstimatedCostUSD, got %f", claudeSummary.EstimatedCostUSD)
	}

	// Check model rankings with pricing
	modelRanks, err := storage.GetModelRankings("24h", "all")
	if err != nil {
		t.Fatalf("GetModelRankings failed: %v", err)
	}
	if len(modelRanks) != 2 {
		t.Fatalf("Expected 2 model rankings, got %d", len(modelRanks))
	}
	for _, mr := range modelRanks {
		if mr.EstimatedCostUSD <= 0 {
			t.Errorf("Expected model %s to have positive EstimatedCostUSD, got %f", mr.Model, mr.EstimatedCostUSD)
		}
	}

	// 3. Check timeline for 24h and 1h
	timeline, err := storage.GetTimeline("24h", "all")
	if err != nil {
		t.Fatalf("GetTimeline failed: %v", err)
	}
	if len(timeline) == 0 {
		t.Errorf("Expected timeline points, got 0")
	} else {
		if len(timeline[0].TimeBucket) != 16 || timeline[0].TimeBucket[13:] != ":00" {
			t.Errorf("Expected hourly bucket format 'YYYY-MM-DD HH:00', got %s", timeline[0].TimeBucket)
		}
	}

	timeline1h, err := storage.GetTimeline("1h", "all")
	if err != nil {
		t.Fatalf("GetTimeline 1h failed: %v", err)
	}
	if len(timeline1h) == 0 {
		t.Errorf("Expected 1h timeline points, got 0")
	} else {
		if len(timeline1h[0].TimeBucket) != 16 {
			t.Errorf("Expected 5-minute bucket format 'YYYY-MM-DD HH:MM', got %s", timeline1h[0].TimeBucket)
		}
	}

	// 4. Check Provider rankings
	provRank, err := storage.GetProviderRankings("24h")
	if err != nil {
		t.Fatalf("GetProviderRankings failed: %v", err)
	}
	if len(provRank) != 2 {
		t.Fatalf("Expected 2 providers, got %d", len(provRank))
	}
	// claude: 300 tokens, openai: 200 tokens
	if provRank[0].Provider != "claude" {
		t.Errorf("Expected top provider to be claude, got %s", provRank[0].Provider)
	}

	// 5. Check Prune
	deleted, err := storage.PruneOlderThan(1)
	if err != nil {
		t.Fatalf("PruneOlderThan failed: %v", err)
	}
	if deleted != 0 {
		t.Errorf("Expected 0 deleted for 1 day prune, got %d", deleted)
	}

	// 6. Check Reset
	if err := storage.ResetDatabase(); err != nil {
		t.Fatalf("ResetDatabase failed: %v", err)
	}
	count, _ := storage.GetTotalRecords()
	if count != 0 {
		t.Errorf("Expected 0 records after reset, got %d", count)
	}
}

func TestStorage_ConfigGetSet(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_config.db")

	storage, err := OpenStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to open storage: %v", err)
	}
	defer storage.Close()

	if val := storage.GetConfig("enabled", "default_val"); val != "default_val" {
		t.Errorf("Expected default_val, got %s", val)
	}

	if err := storage.SetConfig("enabled", "false"); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	if val := storage.GetConfig("enabled", "default_val"); val != "false" {
		t.Errorf("Expected false, got %s", val)
	}
}

func TestDefaultDatabasePath(t *testing.T) {
	os.Setenv("EGO_DB_PATH", "/custom/ego.db")
	defer os.Unsetenv("EGO_DB_PATH")

	if p := defaultDatabasePath(); p != "/custom/ego.db" {
		t.Errorf("Expected /custom/ego.db, got %s", p)
	}
}

func TestStorage_TimelineBuckets(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_timeline.db")

	storage, err := OpenStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to open storage: %v", err)
	}
	defer storage.Close()

	// Choose reference time within the last 50 minutes, ensuring both 5-min buckets remain in the same hour
	ref := time.Now().Add(-25 * time.Minute)
	min := ref.Minute()
	if min < 15 {
		ref = ref.Add(time.Duration(15-min) * time.Minute)
	} else if min > 45 {
		ref = ref.Add(-time.Duration(min-45) * time.Minute)
	}

	b1Time := ref.Truncate(5 * time.Minute)
	b1Same := b1Time.Add(2 * time.Minute)
	b2Time := b1Time.Add(6 * time.Minute)

	events := []EgoEvent{
		{
			Timestamp:        b1Time.UnixMilli(),
			Provider:         "anthropic",
			Model:            "claude-3-5-sonnet",
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Status:           "success",
		},
		{
			Timestamp:        b1Same.UnixMilli(),
			Provider:         "anthropic",
			Model:            "claude-3-5-sonnet",
			PromptTokens:     50,
			CompletionTokens: 25,
			TotalTokens:      75,
			Status:           "success",
		},
		{
			Timestamp:        b2Time.UnixMilli(),
			Provider:         "anthropic",
			Model:            "claude-3-5-sonnet",
			PromptTokens:     200,
			CompletionTokens: 100,
			TotalTokens:      300,
			Status:           "success",
		},
	}

	if err := storage.InsertBatch(events); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	// 1h range: should group into 2 distinct 5-minute buckets
	tl1h, err := storage.GetTimeline("1h", "all")
	if err != nil {
		t.Fatalf("GetTimeline 1h failed: %v", err)
	}
	if len(tl1h) != 2 {
		t.Fatalf("Expected 2 timeline points for 1h range, got %d", len(tl1h))
	}
	if tl1h[0].TotalTokens != 225 {
		t.Errorf("Expected bucket 1 tokens to be 225, got %d", tl1h[0].TotalTokens)
	}
	if tl1h[1].TotalTokens != 300 {
		t.Errorf("Expected bucket 2 tokens to be 300, got %d", tl1h[1].TotalTokens)
	}

	// 24h range: should aggregate into 1 single hourly bucket
	tl24h, err := storage.GetTimeline("24h", "all")
	if err != nil {
		t.Fatalf("GetTimeline 24h failed: %v", err)
	}
	if len(tl24h) != 1 {
		t.Fatalf("Expected 1 timeline point for 24h range, got %d", len(tl24h))
	}
	if tl24h[0].TotalTokens != 525 {
		t.Errorf("Expected 24h bucket tokens to be 525, got %d", tl24h[0].TotalTokens)
	}
	if tl24h[0].TimeBucket[13:] != ":00" {
		t.Errorf("Expected 24h bucket format '...:00', got %s", tl24h[0].TimeBucket)
	}
}

func TestStorage_GeminiFlashHighCost(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_gemini.db")

	storage, err := OpenStorage(dbPath)
	if err != nil {
		t.Fatalf("OpenStorage failed: %v", err)
	}
	defer storage.Close()

	now := time.Now().UnixMilli()
	event := EgoEvent{
		Timestamp:        now,
		Provider:         "antigravity",
		Model:            "gemini-3.8-flash-high",
		PromptTokens:     1000000,
		CompletionTokens: 50000,
		CachedTokens:     500000,
		TotalTokens:      1050000,
		Status:           "success",
	}

	if err := storage.InsertBatch([]EgoEvent{event}); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	summary, err := storage.GetSummary("24h", "all")
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if summary.TotalTokens != 1050000 {
		t.Errorf("Expected 1050000 tokens, got %d", summary.TotalTokens)
	}
	if summary.EstimatedCostUSD <= 0 {
		t.Errorf("Expected positive EstimatedCostUSD for gemini-3.8-flash-high, got %f", summary.EstimatedCostUSD)
	}
	if summary.InputCostUSD <= 0 {
		t.Errorf("Expected positive InputCostUSD, got %f", summary.InputCostUSD)
	}
	if summary.OutputCostUSD <= 0 {
		t.Errorf("Expected positive OutputCostUSD, got %f", summary.OutputCostUSD)
	}
}

func TestStorage_PermissionsAndEmptySuccessRate(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "sub_plugins")
	dbPath := filepath.Join(subDir, "test_perms.db")

	storage, err := OpenStorage(dbPath)
	if err != nil {
		t.Fatalf("OpenStorage failed: %v", err)
	}
	defer storage.Close()

	dirInfo, err := os.Stat(subDir)
	if err != nil {
		t.Fatalf("Stat subDir failed: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0700 {
		t.Errorf("Expected dir permissions 0700, got %o", perm)
	}

	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Stat dbPath failed: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", perm)
	}

	summary, err := storage.GetSummary("24h", "all")
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if summary.TotalRequests != 0 {
		t.Fatalf("Expected 0 total requests, got %d", summary.TotalRequests)
	}
	if summary.SuccessRate != 0.0 {
		t.Errorf("Expected 0.0 SuccessRate for empty storage, got %f", summary.SuccessRate)
	}
}

func TestStorage_NilGuards(t *testing.T) {
	var s *Storage

	if p := s.Path(); p != "" {
		t.Errorf("expected empty string from nil Storage.Path(), got %s", p)
	}
	if _, err := s.GetDatabaseSize(); err == nil {
		t.Errorf("expected error from nil Storage.GetDatabaseSize()")
	}
	if _, err := s.GetTotalRecords(); err == nil {
		t.Errorf("expected error from nil Storage.GetTotalRecords()")
	}
	if summary, err := s.GetSummary("24h", "all"); err != nil || summary == nil {
		t.Errorf("expected empty summary without error from nil Storage.GetSummary(), got %v, %v", summary, err)
	}
	if timeline, err := s.GetTimeline("24h", "all"); err != nil || len(timeline) != 0 {
		t.Errorf("expected empty timeline from nil Storage.GetTimeline()")
	}
	if rankings, err := s.GetProviderRankings("24h"); err != nil || len(rankings) != 0 {
		t.Errorf("expected empty provider rankings from nil Storage.GetProviderRankings()")
	}
	if rankings, err := s.GetModelRankings("24h", "all"); err != nil || len(rankings) != 0 {
		t.Errorf("expected empty model rankings from nil Storage.GetModelRankings()")
	}
	if rankings, err := s.GetAccountRankings("24h", "all"); err != nil || len(rankings) != 0 {
		t.Errorf("expected empty account rankings from nil Storage.GetAccountRankings()")
	}
	if _, err := s.PruneOlderThan(30); err == nil {
		t.Errorf("expected error from nil Storage.PruneOlderThan()")
	}
	if err := s.ResetDatabase(); err == nil {
		t.Errorf("expected error from nil Storage.ResetDatabase()")
	}
	if err := s.Close(); err != nil {
		t.Errorf("expected nil error from nil Storage.Close(), got %v", err)
	}
	if p := s.GetPricing("gpt-4o"); p.Model != "gpt-4o" {
		t.Errorf("expected default model pricing from nil Storage.GetPricing()")
	}
	if list := s.GetAllPricing(); list != nil {
		t.Errorf("expected nil list from nil Storage.GetAllPricing()")
	}
	if err := s.InsertBatch([]EgoEvent{{}}); err == nil {
		t.Errorf("expected error from nil Storage.InsertBatch()")
	}
	if err := s.UpsertPricingBatch([]ModelPricing{{Model: "test"}}); err == nil {
		t.Errorf("expected error from nil Storage.UpsertPricingBatch()")
	}
}

func TestStorage_EmbeddedPricingCoverage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_coverage.db")

	storage, err := OpenStorage(dbPath)
	if err != nil {
		t.Fatalf("OpenStorage failed: %v", err)
	}
	defer storage.Close()

	allPricing := storage.GetAllPricing()
	if len(allPricing) < 3000 {
		t.Errorf("expected over 3000 embedded pricing entries, got %d", len(allPricing))
	}

	testModels := []string{
		"grok-2",
		"grok-beta",
		"moonshot-v1-8k",
		"kimi-k1.5",
		"qwen-2.5-coder-32b",
		"mistral-large",
		"claude-3-5-sonnet",
		"gpt-4o",
		"gemini-3.8-flash-high",
	}

	for _, m := range testModels {
		p := storage.GetPricing(m)
		if p.InputCostPerToken <= 0 {
			t.Errorf("expected model %s to have positive input pricing, got %f", m, p.InputCostPerToken)
		}
	}
}

func TestStorage_IsPricedFlagInModelRankings(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_is_priced.db")

	storage, err := OpenStorage(dbPath)
	if err != nil {
		t.Fatalf("OpenStorage failed: %v", err)
	}
	defer storage.Close()

	now := time.Now().UnixMilli()
	events := []EgoEvent{
		{
			Timestamp:        now,
			Provider:         "xai",
			Model:            "grok-2",
			PromptTokens:     1000,
			CompletionTokens: 500,
			TotalTokens:      1500,
			Status:           "success",
		},
		{
			Timestamp:        now,
			Provider:         "custom",
			Model:            "my-custom-unlisted-local-model",
			PromptTokens:     1000,
			CompletionTokens: 500,
			TotalTokens:      1500,
			Status:           "success",
		},
	}

	if err := storage.InsertBatch(events); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	rankings, err := storage.GetModelRankings("24h", "all")
	if err != nil {
		t.Fatalf("GetModelRankings failed: %v", err)
	}

	if len(rankings) != 2 {
		t.Fatalf("expected 2 model rankings, got %d", len(rankings))
	}

	for _, r := range rankings {
		if r.Model == "grok-2" {
			if !r.IsPriced {
				t.Errorf("expected grok-2 to have IsPriced = true")
			}
			if r.EstimatedCostUSD <= 0 {
				t.Errorf("expected grok-2 to have positive EstimatedCostUSD, got %f", r.EstimatedCostUSD)
			}
		} else if r.Model == "my-custom-unlisted-local-model" {
			if r.IsPriced {
				t.Errorf("expected unlisted custom model to have IsPriced = false")
			}
			if r.EstimatedCostUSD != 0 {
				t.Errorf("expected unlisted custom model to have EstimatedCostUSD = 0, got %f", r.EstimatedCostUSD)
			}
		}
	}
}

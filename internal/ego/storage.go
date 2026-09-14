package ego

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

// Storage manages persistent SQLite storage for developer Ego analytics.
type Storage struct {
	mu           sync.RWMutex
	db           *sql.DB
	dbPath       string
	pricingCache map[string]ModelPricing
}

var memCounter atomic.Uint64

// OpenStorage initializes or opens an SQLite database with WAL mode and indices.
func OpenStorage(path string) (*Storage, error) {
	if path == "" {
		path = defaultDatabasePath()
	}

	isMemory := path == ":memory:" || path == "file::memory:" || strings.Contains(path, "mode=memory")
	if !isMemory {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	var dsn string
	if isMemory {
		id := memCounter.Add(1)
		dsn = fmt.Sprintf("file:egomemory%d?mode=memory&cache=shared", id)
	} else {
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		dsn = fmt.Sprintf("%s%s_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path, separator)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	if isMemory {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	} else {
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(1 * time.Hour)
	}

	s := &Storage{
		db:           db,
		dbPath:       path,
		pricingCache: make(map[string]ModelPricing),
	}

	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize ego schema: %w", err)
	}

	if !isMemory {
		_ = os.Chmod(path, 0600)
		_ = os.Chmod(path+"-wal", 0600)
		_ = os.Chmod(path+"-shm", 0600)
	}

	if err := s.initPricing(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize ego pricing: %w", err)
	}

	return s, nil
}

func defaultDatabasePath() string {
	if custom := os.Getenv("EGO_DB_PATH"); custom != "" {
		return custom
	}
	// Check if plugins directory exists in current working dir
	if info, err := os.Stat("plugins"); err == nil && info.IsDir() {
		return filepath.Join("plugins", "ego.db")
	}
	return "ego.db"
}

func (s *Storage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS ego_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp INTEGER NOT NULL,
		provider TEXT NOT NULL,
		model TEXT NOT NULL,
		account TEXT NOT NULL,
		prompt_tokens INTEGER NOT NULL DEFAULT 0,
		completion_tokens INTEGER NOT NULL DEFAULT 0,
		reasoning_tokens INTEGER NOT NULL DEFAULT 0,
		cached_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0,
		latency_ms INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_ego_timestamp ON ego_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_ego_provider ON ego_events(provider);
	CREATE INDEX IF NOT EXISTS idx_ego_model ON ego_events(model);
	CREATE INDEX IF NOT EXISTS idx_ego_account ON ego_events(account);

		CREATE TABLE IF NOT EXISTS ego_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS ego_pricing (
			model TEXT PRIMARY KEY,
			input_cost_per_token REAL NOT NULL DEFAULT 0.0,
			output_cost_per_token REAL NOT NULL DEFAULT 0.0,
			cache_read_input_token_cost REAL NOT NULL DEFAULT 0.0,
			updated_at INTEGER NOT NULL DEFAULT 0
		);

		CREATE INDEX IF NOT EXISTS idx_ego_pricing_model ON ego_pricing(model);
		`
	_, err := s.db.Exec(schema)
	return err
}

// Close gracefully closes the database.
func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Path returns the physical path of the database.
func (s *Storage) Path() string {
	return s.dbPath
}

func (s *Storage) initPricing() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stmt, err := s.db.Prepare(`
		INSERT INTO ego_pricing (
			model, input_cost_per_token, output_cost_per_token, cache_read_input_token_cost, updated_at
		) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(model) DO UPDATE SET
			input_cost_per_token = excluded.input_cost_per_token,
			output_cost_per_token = excluded.output_cost_per_token,
			cache_read_input_token_cost = excluded.cache_read_input_token_cost,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare pricing insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UnixMilli()
	for _, p := range DefaultCuratedPricing {
		if _, err := stmt.Exec(p.Model, p.InputCostPerToken, p.OutputCostPerToken, p.CacheReadInputTokenCost, now); err != nil {
			return fmt.Errorf("failed to seed curated pricing: %w", err)
		}
	}

	return s.loadPricingCacheLocked()
}

func (s *Storage) loadPricingCacheLocked() error {
	rows, err := s.db.Query("SELECT model, input_cost_per_token, output_cost_per_token, cache_read_input_token_cost, updated_at FROM ego_pricing")
	if err != nil {
		return fmt.Errorf("failed to query ego pricing: %w", err)
	}
	defer rows.Close()

	cache := make(map[string]ModelPricing)
	for rows.Next() {
		var p ModelPricing
		if err := rows.Scan(&p.Model, &p.InputCostPerToken, &p.OutputCostPerToken, &p.CacheReadInputTokenCost, &p.UpdatedAt); err != nil {
			return fmt.Errorf("failed to scan ego pricing row: %w", err)
		}
		cache[p.Model] = p
	}
	s.pricingCache = cache
	return nil
}

// GetPricing finds the pricing for the specified model.
func (s *Storage) GetPricing(model string) ModelPricing {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, _ := FindModelPricing(s.pricingCache, model)
	return p
}

// GetAllPricing returns a slice of all stored model pricing entries.
func (s *Storage) GetAllPricing() []ModelPricing {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]ModelPricing, 0, len(s.pricingCache))
	for _, p := range s.pricingCache {
		list = append(list, p)
	}
	return list
}

// UpsertPricingBatch inserts or updates model pricing records in SQLite and refreshes the cache.
func (s *Storage) UpsertPricingBatch(pricing []ModelPricing) error {
	if len(pricing) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin pricing transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.Prepare(`
		INSERT INTO ego_pricing (
			model, input_cost_per_token, output_cost_per_token, cache_read_input_token_cost, updated_at
		) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(model) DO UPDATE SET
			input_cost_per_token = excluded.input_cost_per_token,
			output_cost_per_token = excluded.output_cost_per_token,
			cache_read_input_token_cost = excluded.cache_read_input_token_cost,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare upsert pricing statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UnixMilli()
	for _, p := range pricing {
		updated := p.UpdatedAt
		if updated == 0 {
			updated = now
		}
		if _, err := stmt.Exec(p.Model, p.InputCostPerToken, p.OutputCostPerToken, p.CacheReadInputTokenCost, updated); err != nil {
			return fmt.Errorf("failed to exec upsert pricing: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit pricing transaction: %w", err)
	}

	return s.loadPricingCacheLocked()
}

// InsertBatch inserts multiple Ego events in a single transaction.
func (s *Storage) InsertBatch(events []EgoEvent) error {
	if len(events) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin batch insert transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.Prepare(`
		INSERT INTO ego_events (
			timestamp, provider, model, account,
			prompt_tokens, completion_tokens, reasoning_tokens, cached_tokens,
			total_tokens, latency_ms, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch insert: %w", err)
	}
	defer stmt.Close()

	for _, e := range events {
		_, err := stmt.Exec(
			e.Timestamp, e.Provider, e.Model, e.Account,
			e.PromptTokens, e.CompletionTokens, e.ReasoningTokens, e.CachedTokens,
			e.TotalTokens, e.LatencyMs, e.Status,
		)
		if err != nil {
			return fmt.Errorf("failed to exec event insert: %w", err)
		}
	}

	return tx.Commit()
}

func calculateMinTimestamp(timeRange string) int64 {
	now := time.Now().UnixMilli()
	switch timeRange {
	case "1h":
		return now - (1 * time.Hour.Milliseconds())
	case "24h":
		return now - (24 * time.Hour.Milliseconds())
	case "7d":
		return now - (7 * 24 * time.Hour.Milliseconds())
	case "30d":
		return now - (30 * 24 * time.Hour.Milliseconds())
	case "all":
		return 0
	default:
		// Default to 24h
		return now - (24 * time.Hour.Milliseconds())
	}
}

// GetSummary returns overall aggregated stats for the selected window and optional provider filter.
func (s *Storage) GetSummary(timeRange string, provider string) (*SummaryStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	minTime := calculateMinTimestamp(timeRange)
	query := `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(reasoning_tokens), 0),
			COALESCE(SUM(cached_tokens), 0),
			COALESCE(AVG(latency_ms), 0.0),
			COALESCE(MIN(latency_ms), 0),
			COALESCE(MAX(latency_ms), 0),
			COALESCE(MIN(timestamp), 0),
			COALESCE(MAX(timestamp), 0)
		FROM ego_events
		WHERE timestamp >= ?
	`
	args := []any{minTime}
	if provider != "" && provider != "all" {
		query += " AND provider = ?"
		args = append(args, provider)
	}

	var stats SummaryStats
	row := s.db.QueryRow(query, args...)
	err := row.Scan(
		&stats.TotalRequests,
		&stats.TotalSuccess,
		&stats.TotalFailed,
		&stats.TotalTokens,
		&stats.PromptTokens,
		&stats.CompletionTokens,
		&stats.ReasoningTokens,
		&stats.CachedTokens,
		&stats.AvgLatencyMs,
		&stats.MinLatencyMs,
		&stats.MaxLatencyMs,
		&stats.EarliestTime,
		&stats.LatestTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query summary stats: %w", err)
	}

	stats.AvgLatencyMs = math.Round(stats.AvgLatencyMs*10) / 10
	if stats.TotalRequests > 0 {
		stats.SuccessRate = math.Round((float64(stats.TotalSuccess)/float64(stats.TotalRequests))*1000) / 10
	} else {
		stats.SuccessRate = 0.0
	}

	// Calculate estimated retail cost by aggregating token usage per model
	costQuery := `
		SELECT
			model,
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(reasoning_tokens), 0),
			COALESCE(SUM(cached_tokens), 0)
		FROM ego_events
		WHERE timestamp >= ?
	`
	costArgs := []any{minTime}
	if provider != "" && provider != "all" {
		costQuery += " AND provider = ?"
		costArgs = append(costArgs, provider)
	}
	costQuery += " GROUP BY model"

	costRows, err := s.db.Query(costQuery, costArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query model token aggregates for cost: %w", err)
	}
	defer costRows.Close()

	var totalCost, promptCost, outputCost float64
	for costRows.Next() {
		var modelName string
		var pTokens, cTokens, rTokens, cachedTokens int64
		if err := costRows.Scan(&modelName, &pTokens, &cTokens, &rTokens, &cachedTokens); err != nil {
			return nil, fmt.Errorf("failed to scan model cost row: %w", err)
		}

		pricing, _ := FindModelPricing(s.pricingCache, modelName)
		tot, inC, outC := CalculateTokenCost(pricing, pTokens, cTokens, rTokens, cachedTokens)
		totalCost += tot
		promptCost += inC
		outputCost += outC
	}

	stats.EstimatedCostUSD = math.Round(totalCost*10000) / 10000
	stats.InputCostUSD = math.Round(promptCost*10000) / 10000
	stats.OutputCostUSD = math.Round(outputCost*10000) / 10000

	return &stats, nil
}

// GetTimeline returns aggregated data points for time-series charts.
func (s *Storage) GetTimeline(timeRange string, provider string) ([]TimelinePoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	minTime := calculateMinTimestamp(timeRange)
	// Bucket format: 5-minute for 1h, hourly for 24h, daily for > 24h
	var bucketExpr string
	if timeRange == "1h" {
		bucketExpr = "strftime('%Y-%m-%d %H:', timestamp / 1000, 'unixepoch', 'localtime') || printf('%02d', (CAST(strftime('%M', timestamp / 1000, 'unixepoch', 'localtime') AS INTEGER) / 5) * 5)"
	} else if timeRange == "24h" {
		bucketExpr = "strftime('%Y-%m-%d %H:00', timestamp / 1000, 'unixepoch', 'localtime')"
	} else {
		bucketExpr = "strftime('%Y-%m-%d', timestamp / 1000, 'unixepoch', 'localtime')"
	}

	query := fmt.Sprintf(`
		SELECT
			%s as bucket,
			MIN(timestamp) as min_ts,
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(CAST(AVG(latency_ms) AS INTEGER), 0)
		FROM ego_events
		WHERE timestamp >= ?
	`, bucketExpr)

	args := []any{minTime}
	if provider != "" && provider != "all" {
		query += " AND provider = ?"
		args = append(args, provider)
	}
	query += " GROUP BY bucket ORDER BY bucket ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}
	defer rows.Close()

	points := make([]TimelinePoint, 0)
	for rows.Next() {
		var p TimelinePoint
		if err := rows.Scan(
			&p.TimeBucket,
			&p.Timestamp,
			&p.TotalRequests,
			&p.SuccessRequests,
			&p.FailedRequests,
			&p.TotalTokens,
			&p.PromptTokens,
			&p.CompletionTokens,
			&p.AvgLatencyMs,
		); err != nil {
			return nil, fmt.Errorf("failed to scan timeline row: %w", err)
		}
		points = append(points, p)
	}

	return points, nil
}

// GetProviderRankings ranks providers by total tokens burned and shows latency metrics.
func (s *Storage) GetProviderRankings(timeRange string) ([]ProviderRanking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	minTime := calculateMinTimestamp(timeRange)
	query := `
		SELECT
			provider,
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(AVG(latency_ms), 0.0)
		FROM ego_events
		WHERE timestamp >= ?
		GROUP BY provider
		ORDER BY SUM(total_tokens) DESC, COUNT(*) DESC
	`
	rows, err := s.db.Query(query, minTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query provider rankings: %w", err)
	}
	defer rows.Close()

	rankings := make([]ProviderRanking, 0)
	for rows.Next() {
		var r ProviderRanking
		var successCount int64
		if err := rows.Scan(
			&r.Provider,
			&r.TotalRequests,
			&successCount,
			&r.TotalTokens,
			&r.PromptTokens,
			&r.CompletionTokens,
			&r.AvgLatencyMs,
		); err != nil {
			return nil, fmt.Errorf("failed to scan provider ranking: %w", err)
		}
		r.AvgLatencyMs = math.Round(r.AvgLatencyMs*10) / 10
		if r.TotalRequests > 0 {
			r.SuccessRate = math.Round((float64(successCount)/float64(r.TotalRequests))*1000) / 10
		}
		rankings = append(rankings, r)
	}

	return rankings, nil
}

// GetModelRankings ranks models by total tokens burned.
func (s *Storage) GetModelRankings(timeRange string, provider string) ([]ModelRanking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	minTime := calculateMinTimestamp(timeRange)
	query := `
		SELECT
			model,
			provider,
			COUNT(*),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(SUM(reasoning_tokens), 0),
			COALESCE(SUM(cached_tokens), 0),
			COALESCE(AVG(latency_ms), 0.0)
		FROM ego_events
		WHERE timestamp >= ?
	`
	args := []any{minTime}
	if provider != "" && provider != "all" {
		query += " AND provider = ?"
		args = append(args, provider)
	}
	query += " GROUP BY model, provider ORDER BY SUM(total_tokens) DESC, COUNT(*) DESC LIMIT 50"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query model rankings: %w", err)
	}
	defer rows.Close()

	rankings := make([]ModelRanking, 0)
	for rows.Next() {
		var r ModelRanking
		var reasoningTokens, cachedTokens int64
		if err := rows.Scan(
			&r.Model,
			&r.Provider,
			&r.TotalRequests,
			&r.TotalTokens,
			&r.PromptTokens,
			&r.CompletionTokens,
			&reasoningTokens,
			&cachedTokens,
			&r.AvgLatencyMs,
		); err != nil {
			return nil, fmt.Errorf("failed to scan model ranking: %w", err)
		}
		r.AvgLatencyMs = math.Round(r.AvgLatencyMs*10) / 10

		pricing, _ := FindModelPricing(s.pricingCache, r.Model)
		tot, _, _ := CalculateTokenCost(pricing, r.PromptTokens, r.CompletionTokens, reasoningTokens, cachedTokens)
		r.EstimatedCostUSD = math.Round(tot*10000) / 10000

		rankings = append(rankings, r)
	}

	return rankings, nil
}

// GetAccountRankings ranks accounts by tokens burned.
func (s *Storage) GetAccountRankings(timeRange string, provider string) ([]AccountRanking, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	minTime := calculateMinTimestamp(timeRange)
	query := `
		SELECT
			account,
			provider,
			COUNT(*),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(prompt_tokens), 0),
			COALESCE(SUM(completion_tokens), 0),
			COALESCE(AVG(latency_ms), 0.0)
		FROM ego_events
		WHERE timestamp >= ? AND account != ''
	`
	args := []any{minTime}
	if provider != "" && provider != "all" {
		query += " AND provider = ?"
		args = append(args, provider)
	}
	query += " GROUP BY account, provider ORDER BY SUM(total_tokens) DESC LIMIT 50"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query account rankings: %w", err)
	}
	defer rows.Close()

	rankings := make([]AccountRanking, 0)
	for rows.Next() {
		var r AccountRanking
		if err := rows.Scan(
			&r.Account,
			&r.Provider,
			&r.TotalRequests,
			&r.TotalTokens,
			&r.PromptTokens,
			&r.CompletionTokens,
			&r.AvgLatencyMs,
		); err != nil {
			return nil, fmt.Errorf("failed to scan account ranking: %w", err)
		}
		r.AvgLatencyMs = math.Round(r.AvgLatencyMs*10) / 10
		rankings = append(rankings, r)
	}

	return rankings, nil
}

// PruneOlderThan deletes events older than the specified number of days.
func (s *Storage) PruneOlderThan(days int) (int64, error) {
	if days <= 0 {
		return 0, fmt.Errorf("days must be greater than zero")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -days).UnixMilli()
	res, err := s.db.Exec("DELETE FROM ego_events WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to prune old ego events: %w", err)
	}
	return res.RowsAffected()
}

// ResetDatabase truncates the ego_events table.
func (s *Storage) ResetDatabase() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.db.Exec("DELETE FROM ego_events;"); err != nil {
		return fmt.Errorf("failed to delete ego events: %w", err)
	}
	// Best-effort vacuum to reclaim disk space
	_, _ = s.db.Exec("VACUUM;")
	return nil
}

// GetDatabaseSize returns the current database file size in bytes.
func (s *Storage) GetDatabaseSize() (int64, error) {
	info, err := os.Stat(s.dbPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// GetTotalRecords returns the total row count in ego_events.
func (s *Storage) GetTotalRecords() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM ego_events").Scan(&count)
	return count, err
}

// GetConfig reads a configuration key from ego_config.
func (s *Storage) GetConfig(key string, defaultVal string) string {
	if s == nil {
		return defaultVal
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.db == nil {
		return defaultVal
	}

	var val string
	err := s.db.QueryRow("SELECT value FROM ego_config WHERE key = ?", key).Scan(&val)
	if err != nil {
		return defaultVal
	}
	return val
}

// SetConfig writes a configuration key to ego_config.
func (s *Storage) SetConfig(key string, val string) error {
	if s == nil {
		return fmt.Errorf("storage is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return fmt.Errorf("database is closed")
	}

	_, err := s.db.Exec("INSERT INTO ego_config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, val)
	return err
}

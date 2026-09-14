package ego

import "time"

// RawUsageDetail holds the fine-grained token breakdown sent by CLIProxyAPI.
type RawUsageDetail struct {
	InputTokens         int64 `json:"InputTokens"`
	OutputTokens        int64 `json:"OutputTokens"`
	ReasoningTokens     int64 `json:"ReasoningTokens"`
	CachedTokens        int64 `json:"CachedTokens"`
	CacheReadTokens     int64 `json:"CacheReadTokens"`
	CacheCreationTokens int64 `json:"CacheCreationTokens"`
	TotalTokens         int64 `json:"TotalTokens"`
}

// RawUsageRecord matches the payload dispatched by CLIProxyAPI on usage.handle.
type RawUsageRecord struct {
	Provider        string         `json:"Provider"`
	ExecutorType    string         `json:"ExecutorType"`
	Model           string         `json:"Model"`
	Alias           string         `json:"Alias"`
	APIKey          string         `json:"APIKey"`
	SessionID       string         `json:"SessionID"`
	ParentSessionID string         `json:"ParentSessionID"`
	AuthID          string         `json:"AuthID"`
	AuthIndex       string         `json:"AuthIndex"`
	AuthType        string         `json:"AuthType"`
	Source          string         `json:"Source"`
	ReasoningEffort string         `json:"ReasoningEffort"`
	ServiceTier     string         `json:"ServiceTier"`
	Generate        bool           `json:"Generate"`
	RequestedAt     time.Time      `json:"RequestedAt"`
	Latency         int64          `json:"Latency"` // Nanoseconds
	TTFT            int64          `json:"TTFT"`    // Nanoseconds
	Failed          bool           `json:"Failed"`
	Detail          RawUsageDetail `json:"Detail"`
}

// EgoEvent represents a persistent record stored in the local SQLite database.
type EgoEvent struct {
	ID                  int64  `json:"id"`
	Timestamp           int64  `json:"timestamp"` // Unix timestamp in milliseconds
	Provider            string `json:"provider"`
	Model               string `json:"model"`
	Account             string `json:"account"`
	PromptTokens        int64  `json:"prompt_tokens"`
	CompletionTokens    int64  `json:"completion_tokens"`
	ReasoningTokens     int64  `json:"reasoning_tokens"`
	CachedTokens        int64  `json:"cached_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
	TotalTokens         int64  `json:"total_tokens"`
	LatencyMs           int64  `json:"latency_ms"`
	Status              string `json:"status"` // "success" or "failed"
}

// SummaryStats contains aggregated metrics for a specified time window.
type SummaryStats struct {
	TotalRequests       int64   `json:"total_requests"`
	TotalSuccess        int64   `json:"total_success"`
	TotalFailed         int64   `json:"total_failed"`
	SuccessRate         float64 `json:"success_rate"`
	TotalTokens         int64   `json:"total_tokens"`
	PromptTokens        int64   `json:"prompt_tokens"`
	CompletionTokens    int64   `json:"completion_tokens"`
	ReasoningTokens     int64   `json:"reasoning_tokens"`
	CachedTokens        int64   `json:"cached_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	EstimatedCostUSD    float64 `json:"estimated_cost_usd"`
	InputCostUSD        float64 `json:"input_cost_usd"`
	OutputCostUSD       float64 `json:"output_cost_usd"`
	AvgLatencyMs        float64 `json:"avg_latency_ms"`
	MinLatencyMs        int64   `json:"min_latency_ms"`
	MaxLatencyMs        int64   `json:"max_latency_ms"`
	EarliestTime        int64   `json:"earliest_time"`
	LatestTime          int64   `json:"latest_time"`
}

// TimelinePoint represents a single data point in a time series chart.
type TimelinePoint struct {
	TimeBucket       string `json:"time_bucket"`
	Timestamp        int64  `json:"timestamp"`
	TotalRequests    int64  `json:"total_requests"`
	SuccessRequests  int64  `json:"success_requests"`
	FailedRequests   int64  `json:"failed_requests"`
	TotalTokens      int64  `json:"total_tokens"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	AvgLatencyMs     int64  `json:"avg_latency_ms"`
}

// ProviderRanking aggregates token and request metrics per provider.
type ProviderRanking struct {
	Provider         string  `json:"provider"`
	TotalRequests    int64   `json:"total_requests"`
	TotalTokens      int64   `json:"total_tokens"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	SuccessRate      float64 `json:"success_rate"`
}

// ModelRanking aggregates token metrics per model.
type ModelRanking struct {
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	TotalRequests    int64   `json:"total_requests"`
	TotalTokens      int64   `json:"total_tokens"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
}

// ModelPricing holds the retail pricing rates per token in USD for a model.
type ModelPricing struct {
	Model                        string  `json:"model"`
	InputCostPerToken            float64 `json:"input_cost_per_token"`
	OutputCostPerToken           float64 `json:"output_cost_per_token"`
	CacheReadInputTokenCost      float64 `json:"cache_read_input_token_cost"`
	CacheCreationInputTokenCost float64 `json:"cache_creation_input_token_cost,omitempty"`
	UpdatedAt                    int64   `json:"updated_at,omitempty"`
}

// AccountRanking aggregates metrics per account/credential.
type AccountRanking struct {
	Account          string  `json:"account"`
	Provider         string  `json:"provider"`
	TotalRequests    int64   `json:"total_requests"`
	TotalTokens      int64   `json:"total_tokens"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
}

// EgoSettings describes configuration and database health.
type EgoSettings struct {
	Enabled       bool   `json:"enabled"`
	DatabasePath  string `json:"database_path"`
	DatabaseBytes int64  `json:"database_bytes"`
	TotalRecords  int64  `json:"total_records"`
}

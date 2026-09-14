package ego

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	globalWorker *Worker
	workerOnce   sync.Once
)

// Worker provides non-blocking, asynchronous batch ingestion of usage records into SQLite.
type Worker struct {
	storage    *Storage
	queue      chan EgoEvent
	stopCh     chan struct{}
	wg         sync.WaitGroup
	enabled    atomic.Bool
	flushEvery time.Duration
	batchSize  int
}

// GetWorker returns or lazily initializes the singleton Ego Worker.
func GetWorker() *Worker {
	workerOnce.Do(func() {
		storage, err := OpenStorage("")
		if err != nil {
			// Fallback to in-memory if disk opening fails
			storage, _ = OpenStorage(":memory:")
		}

		w := NewWorker(storage, 2048, 50, 250*time.Millisecond)
		// Load enabled state from config (default: true)
		if storage != nil {
			val := storage.GetConfig("enabled", "true")
			w.SetEnabled(val != "false")
		}
		w.Start()
		globalWorker = w
	})
	return globalWorker
}

// NewWorker creates an instance of Worker with the specified queue and batch parameters.
func NewWorker(storage *Storage, queueCapacity int, batchSize int, flushEvery time.Duration) *Worker {
	w := &Worker{
		storage:    storage,
		queue:      make(chan EgoEvent, queueCapacity),
		stopCh:     make(chan struct{}),
		batchSize:  batchSize,
		flushEvery: flushEvery,
	}
	w.enabled.Store(true)
	return w
}

// Start begins the background batch insertion worker.
func (w *Worker) Start() {
	w.wg.Add(1)
	go w.flushLoop()
}

// Stop signals the worker to finish flushing queued events and close storage.
func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	if w.storage != nil {
		_ = w.storage.Close()
	}
}

// IsEnabled returns true if metrics collection is active.
func (w *Worker) IsEnabled() bool {
	return w.enabled.Load()
}

// SetEnabled toggles metrics collection on or off.
func (w *Worker) SetEnabled(enabled bool) {
	w.enabled.Store(enabled)
	if w.storage != nil {
		val := "true"
		if !enabled {
			val = "false"
		}
		_ = w.storage.SetConfig("enabled", val)
	}
}

// Storage returns the underlying SQLite storage instance.
func (w *Worker) Storage() *Storage {
	return w.storage
}

// Record parses raw JSON from CLIProxyAPI usage.handle and enqueues the event.
// This function is guaranteed never to block the calling goroutine.
func (w *Worker) Record(rawBytes []byte) {
	if !w.IsEnabled() || len(rawBytes) == 0 {
		return
	}

	var rec RawUsageRecord
	if err := json.Unmarshal(rawBytes, &rec); err != nil {
		return
	}

	event := transformToEgoEvent(rec)

	// Non-blocking send: if queue is saturated, drop event rather than blocking proxy traffic
	select {
	case w.queue <- event:
	default:
		// Queue full: dropped to preserve proxy throughput
	}
}

func transformToEgoEvent(rec RawUsageRecord) EgoEvent {
	ts := rec.RequestedAt.UnixMilli()
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}

	// Determine account name
	account := strings.TrimSpace(rec.AuthID)
	if account == "" {
		account = strings.TrimSpace(rec.AuthIndex)
	}
	if account == "" && rec.APIKey != "" {
		// Mask API Key to avoid exposing secrets
		key := strings.TrimSpace(rec.APIKey)
		if len(key) > 8 {
			account = key[:4] + "..." + key[len(key)-4:]
		} else {
			account = "api_key"
		}
	}
	if account == "" {
		account = "default"
	}

	// Latency in milliseconds (rec.Latency is in nanoseconds)
	latencyMs := rec.Latency / 1_000_000
	if latencyMs <= 0 && rec.TTFT > 0 {
		latencyMs = rec.TTFT / 1_000_000
	}

	prompt := rec.Detail.InputTokens
	completion := rec.Detail.OutputTokens
	reasoning := rec.Detail.ReasoningTokens
	// cached must hold cache *reads* only: it is billed as a separate term from
	// CacheCreationTokens. CLIProxyAPI echoes cache creation into CachedTokens when a
	// request has no cache reads, so falling back to CachedTokens blindly would bill the
	// cache-creation tokens twice on cold requests. Providers that only fill the legacy
	// CachedTokens field still fall back to it.
	cached := rec.Detail.CacheReadTokens
	if cached == 0 && rec.Detail.CachedTokens != rec.Detail.CacheCreationTokens {
		cached = rec.Detail.CachedTokens
	}
	cacheCreation := rec.Detail.CacheCreationTokens

	provider := strings.ToLower(strings.TrimSpace(rec.Provider))
	if provider == "" {
		provider = "unknown"
	}

	total := rec.Detail.TotalTokens
	if total == 0 {
		semantics := ResolveTokenSemantics(provider, rec.ExecutorType)
		switch semantics {
		case SemanticsIndependent:
			total = prompt + cached + cacheCreation + completion + reasoning
		case SemanticsSeparateReasoning:
			total = prompt + completion + reasoning
		default: // SemanticsSubset
			outTokens := completion
			if outTokens < reasoning {
				outTokens = reasoning
			}
			total = prompt + outTokens
		}
	}

	status := "success"
	if rec.Failed {
		status = "failed"
	}

	model := strings.TrimSpace(rec.Model)
	if model == "" {
		model = strings.TrimSpace(rec.Alias)
	}
	if model == "" {
		model = "unknown"
	}

	return EgoEvent{
		Timestamp:           ts,
		Provider:            provider,
		Model:               model,
		Account:             account,
		PromptTokens:        prompt,
		CompletionTokens:    completion,
		ReasoningTokens:     reasoning,
		CachedTokens:        cached,
		CacheCreationTokens: cacheCreation,
		TotalTokens:         total,
		LatencyMs:           latencyMs,
		Status:              status,
	}
}

func (w *Worker) flushLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.flushEvery)
	defer ticker.Stop()

	batch := make([]EgoEvent, 0, w.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if w.storage != nil {
			if err := w.storage.InsertBatch(batch); err != nil {
				log.Printf("ego worker: failed to insert batch of %d events: %v", len(batch), err)
			}
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-w.stopCh:
			// Drain remaining items in queue before exiting
			for {
				select {
				case e := <-w.queue:
					batch = append(batch, e)
					if len(batch) >= w.batchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}

		case e := <-w.queue:
			batch = append(batch, e)
			if len(batch) >= w.batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

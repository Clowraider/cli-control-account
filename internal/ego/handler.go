package ego

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler serves JSON API endpoints for Ego analytics and maintenance.
type Handler struct {
	worker *Worker
}

// NewHandler creates an Ego API handler.
func NewHandler(w *Worker) *Handler {
	if w == nil {
		w = GetWorker()
	}
	return &Handler{worker: w}
}

// ServeHTTP routes incoming requests under /ego/api.
func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	rw.Header().Set("X-Content-Type-Options", "nosniff")

	path := req.URL.Path
	// Match subpath after /ego/api
	switch {
	case endsWith(path, "/stats"):
		if req.Method != http.MethodGet {
			h.writeError(rw, http.StatusMethodNotAllowed, "method not allowed, use GET")
			return
		}
		h.handleStats(rw, req)
	case endsWith(path, "/timeline"):
		if req.Method != http.MethodGet {
			h.writeError(rw, http.StatusMethodNotAllowed, "method not allowed, use GET")
			return
		}
		h.handleTimeline(rw, req)
	case endsWith(path, "/providers"):
		if req.Method != http.MethodGet {
			h.writeError(rw, http.StatusMethodNotAllowed, "method not allowed, use GET")
			return
		}
		h.handleProviders(rw, req)
	case endsWith(path, "/models"):
		if req.Method != http.MethodGet {
			h.writeError(rw, http.StatusMethodNotAllowed, "method not allowed, use GET")
			return
		}
		h.handleModels(rw, req)
	case endsWith(path, "/accounts"):
		if req.Method != http.MethodGet {
			h.writeError(rw, http.StatusMethodNotAllowed, "method not allowed, use GET")
			return
		}
		h.handleAccounts(rw, req)
	case endsWith(path, "/settings"):
		switch req.Method {
		case http.MethodGet:
			h.handleGetSettings(rw, req)
		case http.MethodPost:
			h.handlePostSettings(rw, req)
		default:
			h.writeError(rw, http.StatusMethodNotAllowed, "method not allowed")
		}
	case endsWith(path, "/prune"):
		if req.Method != http.MethodPost {
			h.writeError(rw, http.StatusMethodNotAllowed, "method not allowed, use POST")
			return
		}
		h.handlePrune(rw, req)
	case endsWith(path, "/reset"):
		if req.Method != http.MethodPost {
			h.writeError(rw, http.StatusMethodNotAllowed, "method not allowed, use POST")
			return
		}
		h.handleReset(rw, req)
	case endsWith(path, "/pricing"):
		if req.Method != http.MethodGet {
			h.writeError(rw, http.StatusMethodNotAllowed, "method not allowed, use GET")
			return
		}
		h.handleGetPricing(rw, req)
	default:
		h.writeError(rw, http.StatusNotFound, "endpoint not found")
	}
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func (h *Handler) handleStats(rw http.ResponseWriter, req *http.Request) {
	timeRange := req.URL.Query().Get("range")
	provider := req.URL.Query().Get("provider")

	stats, err := h.worker.Storage().GetSummary(timeRange, provider)
	if err != nil {
		h.writeError(rw, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(rw, http.StatusOK, stats)
}

func (h *Handler) handleTimeline(rw http.ResponseWriter, req *http.Request) {
	timeRange := req.URL.Query().Get("range")
	provider := req.URL.Query().Get("provider")

	timeline, err := h.worker.Storage().GetTimeline(timeRange, provider)
	if err != nil {
		h.writeError(rw, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(rw, http.StatusOK, timeline)
}

func (h *Handler) handleProviders(rw http.ResponseWriter, req *http.Request) {
	timeRange := req.URL.Query().Get("range")

	rankings, err := h.worker.Storage().GetProviderRankings(timeRange)
	if err != nil {
		h.writeError(rw, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(rw, http.StatusOK, rankings)
}

func (h *Handler) handleModels(rw http.ResponseWriter, req *http.Request) {
	timeRange := req.URL.Query().Get("range")
	provider := req.URL.Query().Get("provider")

	rankings, err := h.worker.Storage().GetModelRankings(timeRange, provider)
	if err != nil {
		h.writeError(rw, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(rw, http.StatusOK, rankings)
}

func (h *Handler) handleAccounts(rw http.ResponseWriter, req *http.Request) {
	timeRange := req.URL.Query().Get("range")
	provider := req.URL.Query().Get("provider")

	rankings, err := h.worker.Storage().GetAccountRankings(timeRange, provider)
	if err != nil {
		h.writeError(rw, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(rw, http.StatusOK, rankings)
}

func (h *Handler) handleGetSettings(rw http.ResponseWriter, req *http.Request) {
	storage := h.worker.Storage()
	size, _ := storage.GetDatabaseSize()
	total, _ := storage.GetTotalRecords()

	settings := EgoSettings{
		Enabled:       h.worker.IsEnabled(),
		DatabasePath:  storage.Path(),
		DatabaseBytes: size,
		TotalRecords:  total,
	}
	h.writeJSON(rw, http.StatusOK, settings)
}

type postSettingsPayload struct {
	Enabled *bool `json:"enabled"`
}

func (h *Handler) handlePostSettings(rw http.ResponseWriter, req *http.Request) {
	if qEnabled := req.URL.Query().Get("enabled"); qEnabled != "" {
		enabled := qEnabled == "true" || qEnabled == "1"
		h.worker.SetEnabled(enabled)
		h.handleGetSettings(rw, req)
		return
	}

	var payload postSettingsPayload
	if req.Body != nil && req.ContentLength > 0 {
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			h.writeError(rw, http.StatusBadRequest, "invalid request body")
			return
		}
		if payload.Enabled != nil {
			h.worker.SetEnabled(*payload.Enabled)
		}
	}

	h.handleGetSettings(rw, req)
}

type prunePayload struct {
	Days int `json:"days"`
}

func (h *Handler) handlePrune(rw http.ResponseWriter, req *http.Request) {
	var payload prunePayload
	_ = json.NewDecoder(req.Body).Decode(&payload)

	days := payload.Days
	if days <= 0 {
		if qDays := req.URL.Query().Get("days"); qDays != "" {
			days, _ = strconv.Atoi(qDays)
		}
	}
	if days <= 0 {
		days = 30
	}

	pruned, err := h.worker.Storage().PruneOlderThan(days)
	if err != nil {
		h.writeError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(rw, http.StatusOK, map[string]any{
		"ok":           true,
		"rows_deleted": pruned,
		"days":         days,
	})
}

func (h *Handler) handleReset(rw http.ResponseWriter, req *http.Request) {
	if err := h.worker.Storage().ResetDatabase(); err != nil {
		h.writeError(rw, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(rw, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "database truncated and vacuumed successfully",
	})
}

func (h *Handler) handleGetPricing(rw http.ResponseWriter, req *http.Request) {
	pricing := h.worker.Storage().GetAllPricing()
	h.writeJSON(rw, http.StatusOK, map[string]any{
		"total":   len(pricing),
		"pricing": pricing,
	})
}

func (h *Handler) writeJSON(rw http.ResponseWriter, status int, data any) {
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(data)
}

func (h *Handler) writeError(rw http.ResponseWriter, status int, msg string) {
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(map[string]any{
		"error":   true,
		"message": msg,
	})
}

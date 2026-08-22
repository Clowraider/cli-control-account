package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"strings"

	"control-account/internal/web"
)

// DefaultQuotaBasePath defines the standard management resource path for Quota UI.
const DefaultQuotaBasePath = "/v0/resource/plugins/control-account/quota"

// DefaultSecurityHeaders returns standard HTTP security and cache headers.
func DefaultSecurityHeaders(contentType string) map[string][]string {
	return map[string][]string{
		"Content-Type":           {contentType},
		"Cache-Control":          {"public, max-age=3600"},
		"X-Content-Type-Options": {"nosniff"},
	}
}

// ResourceHandler serves embedded web assets with path sanitization and security headers.
type ResourceHandler struct {
	basePath string
}

// NewResourceHandler creates a new ResourceHandler with the default quota base path.
func NewResourceHandler() *ResourceHandler {
	return &ResourceHandler{
		basePath: DefaultQuotaBasePath,
	}
}

// NewResourceHandlerWithPath creates a ResourceHandler with a custom base path.
func NewResourceHandlerWithPath(basePath string) *ResourceHandler {
	return &ResourceHandler{
		basePath: basePath,
	}
}

// ErrorResponse represents the structured JSON error payload.
type ErrorResponse struct {
	Error  string `json:"error"`
	Path   string `json:"path,omitempty"`
	Status int    `json:"status"`
}

// ServeHTTP handles incoming HTTP requests for static assets.
func (h *ResourceHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		h.writeJSONError(rw, "method not allowed", http.StatusMethodNotAllowed, req.URL.Path)
		return
	}

	requestPath := req.URL.Path
	if rawPath := req.URL.RawPath; rawPath != "" {
		if unescaped, err := url.PathUnescape(rawPath); err == nil {
			requestPath = unescaped
		}
	}

	// Security: Guard against directory traversal attempts in path
	if strings.Contains(req.URL.Path, "..") || strings.Contains(req.URL.RawPath, "..") || strings.Contains(requestPath, "..") {
		h.writeJSONError(rw, "invalid or unsafe path", http.StatusNotFound, req.URL.Path)
		return
	}

	// Verify path begins with the registered basePath
	if !strings.HasPrefix(requestPath, h.basePath) {
		h.writeJSONError(rw, "resource not found", http.StatusNotFound, req.URL.Path)
		return
	}

	// Extract subpath relative to base path
	relPath := strings.TrimPrefix(requestPath, h.basePath)
	relPath = strings.TrimPrefix(relPath, "/")

	var assetName string
	switch relPath {
	case "", "index.html", "/":
		assetName = "index.html"
	case "styles.css":
		assetName = "styles.css"
	case "app.js":
		assetName = "app.js"
	default:
		// Clean and check arbitrary asset subpath
		clean := path.Clean(relPath)
		if clean == "." || strings.HasPrefix(clean, "..") {
			h.writeJSONError(rw, "resource not found", http.StatusNotFound, req.URL.Path)
			return
		}
		assetName = clean
	}

	data, mimeType, err := web.GetAsset(assetName)
	if err != nil {
		h.writeJSONError(rw, "resource not found", http.StatusNotFound, req.URL.Path)
		return
	}

	for k, v := range DefaultSecurityHeaders(mimeType) {
		for _, val := range v {
			rw.Header().Add(k, val)
		}
	}
	rw.WriteHeader(http.StatusOK)

	if req.Method != http.MethodHead {
		_, _ = rw.Write(data)
	}
}

// writeJSONError formats and sends a structured JSON error response.
func (h *ResourceHandler) writeJSONError(rw http.ResponseWriter, message string, status int, reqPath string) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.Header().Set("X-Content-Type-Options", "nosniff")
	rw.WriteHeader(status)

	resp := ErrorResponse{
		Error:  message,
		Path:   reqPath,
		Status: status,
	}

	_ = json.NewEncoder(rw).Encode(resp)
}

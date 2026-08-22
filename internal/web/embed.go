package web

import (
	"embed"
	"errors"
	"io/fs"
	"mime"
	"path"
	"strings"
)

//go:embed assets/*
var AssetsFS embed.FS

// ErrAssetNotFound is returned when the requested asset does not exist.
var ErrAssetNotFound = errors.New("asset not found")

// ErrInvalidPath is returned when the path is malformed or attempts directory traversal.
var ErrInvalidPath = errors.New("invalid or unsafe asset path")

// GetAsset reads a file from embedded assets by name (e.g., "index.html", "styles.css", "app.js").
// Returns file content, resolved MIME content-type, and error if not found or invalid.
func GetAsset(assetName string) ([]byte, string, error) {
	cleanName := path.Clean(strings.TrimSpace(assetName))
	cleanName = strings.TrimPrefix(cleanName, "/")

	// Check for path traversal or empty names
	if cleanName == "." || cleanName == "" || strings.HasPrefix(cleanName, "..") || strings.Contains(cleanName, "/../") {
		return nil, "", ErrInvalidPath
	}

	// Assets are embedded under assets/
	fullPath := path.Join("assets", cleanName)
	data, err := fs.ReadFile(AssetsFS, fullPath)
	if err != nil {
		return nil, "", ErrAssetNotFound
	}

	mimeType := ResolveMIMEType(cleanName)
	return data, mimeType, nil
}

// ResolveMIMEType determines the Content-Type header value based on file extension.
func ResolveMIMEType(filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".ico":
		return "image/x-icon"
	default:
		detected := mime.TypeByExtension(ext)
		if detected != "" {
			return detected
		}
		return "application/octet-stream"
	}
}

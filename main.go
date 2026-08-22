package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	cliproxy_host_call_fn call;
	cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);

static const cliproxy_host_api* stored_host;

static void store_host_api(const cliproxy_host_api* host) {
	stored_host = host;
}
*/
import "C"

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"unsafe"

	"control-account/internal/handlers"
	"control-account/internal/lifecycle"
	"control-account/internal/web"
)

const abiVersion uint32 = 1

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type managementRequestPayload struct {
	Method  string              `json:"Method"`
	Path    string              `json:"Path"`
	Headers map[string][]string `json:"Headers"`
	Query   map[string][]string `json:"Query"`
	Body    []byte              `json:"Body"`
}

type managementResponsePayload struct {
	StatusCode int                 `json:"StatusCode"`
	Headers    map[string][]string `json:"Headers"`
	Body       string              `json:"Body"` // Base64 encoded body
}

var (
	globalDispatcher *lifecycle.Dispatcher
	initOnce         sync.Once
)

// GetDispatcher retrieves or lazily initializes the singleton lifecycle dispatcher.
func GetDispatcher() *lifecycle.Dispatcher {
	initOnce.Do(func() {
		globalDispatcher = lifecycle.NewDispatcher()
	})
	return globalDispatcher
}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
	C.store_host_api(host)
	plugin.abi_version = C.uint32_t(abiVersion)
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeResponse(response, errorEnvelope("invalid_method", "method is required"))
		return 1
	}

	methodStr := C.GoString(method)
	var reqBytes []byte
	if request != nil && requestLen > 0 {
		reqBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}

	raw, errHandle := handlePluginMethod(methodStr, reqBytes)
	if errHandle != nil {
		writeResponse(response, errorEnvelope("plugin_error", errHandle.Error()))
		return 1
	}

	writeResponse(response, raw)
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, len C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
	_ = len
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {}

func handlePluginMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case "plugin.register", "plugin.reconfigure":
		registration := map[string]any{
			"schema_version": 1,
			"metadata": map[string]any{
				"Name":             "control-account",
				"Version":          "0.1.0",
				"Author":           "Clowraider",
				"Description":      "Quota management dashboard with account prefix support",
				"GitHubRepository": "https://github.com/Clowraider/cli-control-account",
				"Logo":             "",
				"ConfigFields":     []any{},
			},
			"capabilities": map[string]any{
				"management_api": true,
			},
		}
		raw, err := json.Marshal(registration)
		if err != nil {
			return nil, err
		}
		return okEnvelope(raw), nil

	case "management.register":
		regResponse := map[string]any{
			"resources": []map[string]any{
				{
					"Path":        "/quota",
					"Menu":        "Control Account",
					"Description": "Quota management dashboard with account prefix display",
				},
			},
		}
		raw, err := json.Marshal(regResponse)
		if err != nil {
			return nil, err
		}
		return okEnvelope(raw), nil

	case "management.handle":
		return handleManagementHTTP(request)

	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func handleManagementHTTP(request []byte) ([]byte, error) {
	var req managementRequestPayload
	if len(request) > 0 {
		_ = json.Unmarshal(request, &req)
	}

	// Resolve asset path from request path
	subpath := req.Path
	if idx := strings.Index(subpath, "/quota"); idx != -1 {
		subpath = subpath[idx+len("/quota"):]
	}
	subpath = strings.TrimPrefix(subpath, "/")
	if subpath == "" {
		subpath = "index.html"
	}

	assetBytes, contentType, err := web.GetAsset(subpath)
	if err != nil {
		resp := managementResponsePayload{
			StatusCode: http.StatusNotFound,
			Headers: map[string][]string{
				"Content-Type": {"application/json; charset=utf-8"},
			},
			Body: base64.StdEncoding.EncodeToString([]byte(`{"error":"not_found","message":"resource not found"}`)),
		}
		raw, _ := json.Marshal(resp)
		return okEnvelope(raw), nil
	}

	headers := handlers.DefaultSecurityHeaders(contentType)
	resp := managementResponsePayload{
		StatusCode: http.StatusOK,
		Headers:    headers,
		Body:       base64.StdEncoding.EncodeToString(assetBytes),
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return okEnvelope(raw), nil
}

func okEnvelope(result []byte) []byte {
	raw, _ := json.Marshal(envelope{OK: true, Result: json.RawMessage(result)})
	return raw
}

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	ptr := C.CBytes(raw)
	if ptr == nil {
		return
	}
	response.ptr = ptr
	response.len = C.size_t(len(raw))
}

func main() {
	// main is intentionally empty as required for c-shared library builds
}

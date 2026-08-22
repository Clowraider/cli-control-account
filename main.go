package main

/*
#include <stdlib.h>
#include <stdint.h>

// PluginHostAPI represents host function pointers provided during initialization.
typedef struct {
    const char* api_version;
    int (*register_hook)(const char* event_name, void* hook_func);
} PluginHostAPI;
*/
import "C"
import (
	"context"
	"sync"
	"unsafe"

	"control-account/internal/lifecycle"
)

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
func cliproxy_plugin_init(api *C.PluginHostAPI) C.int {
	if api == nil {
		return -1
	}

	d := GetDispatcher()
	// Register the core plugin metadata
	_ = d.OnPluginRegister(context.Background(), nil)

	return 0
}

//export cliproxy_plugin_dispatch
func cliproxy_plugin_dispatch(event *C.char, payload *C.char) *C.char {
	if event == nil {
		return nil
	}

	eventStr := C.GoString(event)
	var payloadStr string
	if payload != nil {
		payloadStr = C.GoString(payload)
	}

	d := GetDispatcher()
	res, err := d.DispatchEvent(context.Background(), eventStr, payloadStr)
	if err != nil {
		return C.CString(err.Error())
	}
	_ = res
	return C.CString("ok")
}

//export cliproxy_free_string
func cliproxy_free_string(ptr *C.char) {
	if ptr != nil {
		C.free(unsafe.Pointer(ptr))
	}
}

func main() {
	// main is intentionally empty as required for c-shared library builds
}

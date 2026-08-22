package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"control-account/internal/handlers"
)

// Standard lifecycle event names.
const (
	EventPluginRegister     = "plugin.register"
	EventPluginReconfigure  = "plugin.reconfigure"
	EventManagementRegister = "management.register"
	EventManagementHandle   = "management.handle"
)

// Management endpoint paths.
const (
	DefaultManagementResourcePath = "/v0/resource/plugins/control-account/quota"
)

// PluginConfigField represents a configurable field for the plugin.
type PluginConfigField struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	EnumValues  []string `json:"enum_values,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Default     any      `json:"default,omitempty"`
}

// PluginMenu represents UI navigation entries contributed by the plugin.
type PluginMenu struct {
	Path        string `json:"path"`
	Menu        string `json:"menu"`
	Description string `json:"description,omitempty"`
}

// PluginRegistration contains metadata and definitions provided during registration.
type PluginRegistration struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Version       string              `json:"version"`
	Author        string              `json:"author"`
	Description   string              `json:"description"`
	Logo          string              `json:"logo,omitempty"`
	ConfigFields  []PluginConfigField `json:"config_fields,omitempty"`
	Menus         []PluginMenu        `json:"menus,omitempty"`
	SupportsOAuth bool                `json:"supports_oauth,omitempty"`
	OAuthProvider string              `json:"oauth_provider,omitempty"`
}

// ManagementRouter abstracts host endpoint registration.
type ManagementRouter interface {
	RegisterResource(path string, handler http.Handler) error
}

// MapManagementRouter is a simple in-memory ManagementRouter implementation.
type MapManagementRouter struct {
	mu     sync.RWMutex
	routes map[string]http.Handler
}

// NewMapManagementRouter creates a new MapManagementRouter.
func NewMapManagementRouter() *MapManagementRouter {
	return &MapManagementRouter{
		routes: make(map[string]http.Handler),
	}
}

// RegisterResource binds a resource path to a handler.
func (m *MapManagementRouter) RegisterResource(path string, handler http.Handler) error {
	if path == "" {
		return errors.New("empty path not allowed")
	}
	if handler == nil {
		return errors.New("nil handler not allowed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routes[path] = handler
	return nil
}

// GetHandler retrieves the handler for a given path.
func (m *MapManagementRouter) GetHandler(path string) (http.Handler, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.routes[path]
	return h, ok
}

// LifecycleHandler defines the lifecycle hook interface.
type LifecycleHandler interface {
	OnPluginRegister(ctx context.Context, reg *PluginRegistration) error
	OnPluginReconfigure(ctx context.Context, config map[string]any) error
	OnManagementRegister(ctx context.Context, router ManagementRouter) error
	OnManagementHandle(ctx context.Context, req *http.Request, rw http.ResponseWriter) bool
}

// Dispatcher manages lifecycle registrations, handlers, and configurations.
type Dispatcher struct {
	mu           sync.RWMutex
	metadata     PluginRegistration
	config       map[string]any
	routes       map[string]http.Handler
	eventHooks   map[string][]func(ctx context.Context, payload any) (any, error)
	customRouter ManagementRouter
}

// NewDispatcher creates an initialized Lifecycle Dispatcher.
func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		routes:     make(map[string]http.Handler),
		config:     make(map[string]any),
		eventHooks: make(map[string][]func(ctx context.Context, payload any) (any, error)),
		metadata: PluginRegistration{
			ID:          "control-account",
			Name:        "Control Account Quota Plugin",
			Version:     "1.0.0",
			Author:      "CLIProxyAPI Team",
			Description: "Account quota monitoring and management center dashboard plugin",
			Menus: []PluginMenu{
				{
					Path:        DefaultManagementResourcePath,
					Menu:        "Quota Management",
					Description: "View and filter real-time account quota windows and token metrics",
				},
			},
		},
	}
	d.RegisterRoute(DefaultManagementResourcePath, handlers.NewResourceHandler())
	return d
}

// RegisterRoute binds an HTTP handler to an absolute resource route.
func (d *Dispatcher) RegisterRoute(path string, handler http.Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.routes[path] = handler
}

// GetRoute returns the handler registered for a route.
func (d *Dispatcher) GetRoute(path string) (http.Handler, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	h, ok := d.routes[path]
	return h, ok
}

// OnPluginRegister populates or overrides the plugin registration metadata.
func (d *Dispatcher) OnPluginRegister(ctx context.Context, reg *PluginRegistration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if reg != nil {
		if reg.ID != "" {
			d.metadata.ID = reg.ID
		}
		if reg.Name != "" {
			d.metadata.Name = reg.Name
		}
		if reg.Version != "" {
			d.metadata.Version = reg.Version
		}
		if reg.Author != "" {
			d.metadata.Author = reg.Author
		}
		if reg.Description != "" {
			d.metadata.Description = reg.Description
		}
		if len(reg.ConfigFields) > 0 {
			d.metadata.ConfigFields = reg.ConfigFields
		}
		if len(reg.Menus) > 0 {
			d.metadata.Menus = reg.Menus
		}
		d.metadata.SupportsOAuth = reg.SupportsOAuth
		d.metadata.OAuthProvider = reg.OAuthProvider
	}
	return nil
}

// GetMetadata returns a copy of current plugin registration metadata.
func (d *Dispatcher) GetMetadata() PluginRegistration {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.metadata
}

// OnPluginReconfigure updates plugin configuration in a thread-safe manner.
func (d *Dispatcher) OnPluginReconfigure(ctx context.Context, config map[string]any) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if config == nil {
		d.config = make(map[string]any)
		return nil
	}

	newConfig := make(map[string]any, len(config))
	for k, v := range config {
		newConfig[k] = v
	}
	d.config = newConfig
	return nil
}

// GetConfigValue returns a configuration value by key.
func (d *Dispatcher) GetConfigValue(key string) (any, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	val, ok := d.config[key]
	return val, ok
}

// OnManagementRegister binds routes to the host router.
func (d *Dispatcher) OnManagementRegister(ctx context.Context, router ManagementRouter) error {
	if router == nil {
		return errors.New("management router cannot be nil")
	}

	d.mu.Lock()
	d.customRouter = router
	routes := make(map[string]http.Handler, len(d.routes))
	for p, h := range d.routes {
		routes[p] = h
	}
	d.mu.Unlock()

	for p, h := range routes {
		if err := router.RegisterResource(p, h); err != nil {
			return fmt.Errorf("failed to register route %s: %w", p, err)
		}
	}
	return nil
}

// OnManagementHandle handles incoming HTTP requests matching management resources.
// Returns true if the request was routed, false otherwise.
func (d *Dispatcher) OnManagementHandle(ctx context.Context, req *http.Request, rw http.ResponseWriter) bool {
	if req == nil || rw == nil {
		return false
	}

	path := req.URL.Path
	handler, ok := d.GetRoute(path)
	if !ok {
		return false
	}

	handler.ServeHTTP(rw, req)
	return true
}

// DispatchEvent routes arbitrary lifecycle events to registered hooks or builtin handlers.
func (d *Dispatcher) DispatchEvent(ctx context.Context, event string, payload any) (any, error) {
	switch event {
	case EventPluginRegister:
		var reg *PluginRegistration
		if payload != nil {
			switch p := payload.(type) {
			case *PluginRegistration:
				reg = p
			case PluginRegistration:
				reg = &p
			case []byte:
				var parsed PluginRegistration
				if err := json.Unmarshal(p, &parsed); err != nil {
					return nil, fmt.Errorf("invalid plugin.register payload: %w", err)
				}
				reg = &parsed
			case string:
				var parsed PluginRegistration
				if err := json.Unmarshal([]byte(p), &parsed); err != nil {
					return nil, fmt.Errorf("invalid plugin.register payload: %w", err)
				}
				reg = &parsed
			}
		}
		err := d.OnPluginRegister(ctx, reg)
		return d.GetMetadata(), err

	case EventPluginReconfigure:
		var configMap map[string]any
		if payload != nil {
			switch p := payload.(type) {
			case map[string]any:
				configMap = p
			case []byte:
				if err := json.Unmarshal(p, &configMap); err != nil {
					return nil, fmt.Errorf("invalid plugin.reconfigure payload: %w", err)
				}
			case string:
				if err := json.Unmarshal([]byte(p), &configMap); err != nil {
					return nil, fmt.Errorf("invalid plugin.reconfigure payload: %w", err)
				}
			}
		}
		err := d.OnPluginReconfigure(ctx, configMap)
		return true, err

	case EventManagementRegister:
		router, ok := payload.(ManagementRouter)
		if !ok || router == nil {
			return false, errors.New("payload for management.register must implement ManagementRouter")
		}
		err := d.OnManagementRegister(ctx, router)
		return err == nil, err

	default:
		d.mu.RLock()
		hooks, exists := d.eventHooks[event]
		d.mu.RUnlock()
		if !exists || len(hooks) == 0 {
			return nil, fmt.Errorf("unsupported or unhandled lifecycle event: %s", event)
		}

		var lastRes any
		for _, hook := range hooks {
			res, err := hook(ctx, payload)
			if err != nil {
				return nil, err
			}
			lastRes = res
		}
		return lastRes, nil
	}
}

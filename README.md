# CLIProxyAPI Control Account Plugin (`v0.1`)

[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A standard C-ABI dynamic library plugin in Go for **[CLIProxyAPI](https://github.com/router-for-all/CLIProxyAPI)** that provides an embedded Quota Management Single-Page Application (SPA) dashboard with dark theme, real-time quota calculations, provider tab filtering, and explicit **Account Prefix** visibility.

---

## Features

- **Standard C-ABI Dynamic Plugin**: Exports `cliproxy_plugin_init` and integrates seamlessly with CLIProxyAPI host lifecycle hooks (`plugin.register`, `plugin.reconfigure`, `management.register`, `management.handle`).
- **Embedded Web SPA**: Zero-dependency static asset delivery bundled via Go `//go:embed` at `/v0/resource/plugins/control-account/quota`.
- **Dark Theme Dashboard**: Modern dark mode UI matching the CLI Proxy API Management Center aesthetics.
- **Provider Filtering**: Filter accounts across supported providers: `All`, `Antigravity`, `Claude`, `Codex`, `Kimi`, and `xAI`.
- **Real-Time Timers & Progress Gauges**: Live countdown timers for quota reset windows and visual consumption meters.
- **Account Prefix Visibility**: Renders the account prefix immediately beneath the account ID with a fixed-height placeholder (`-`) ensuring uniform card heights and flawless grid alignment.
- **Security Hardened**: Strict path traversal defense, `X-Content-Type-Options: nosniff`, and structured JSON 404 responses.

---

## Directory Structure

```text
cli-control-account/
├── Makefile                          # Build & test automation
├── go.mod                            # Go module definition
├── main.go                           # C-ABI entry point (cliproxy_plugin_init)
├── main_test.go                      # Unit tests for C-ABI entry point
├── internal/
│   ├── handlers/                     # HTTP resource handler & router
│   │   ├── resource.go
│   │   └── resource_test.go
│   ├── lifecycle/                    # Host lifecycle event dispatcher
│   │   ├── events.go
│   │   └── events_test.go
│   ├── models/                       # Quota domain models & prefix formatting
│   │   ├── quota.go
│   │   └── quota_test.go
│   └── web/                          # Embedded static web assets
│       ├── embed.go
│       ├── embed_test.go
│       └── assets/
│           ├── app.js
│           ├── index.html
│           └── styles.css
└── README.md
```

---

## Building & Testing

### Prerequisites

- Go 1.22+ with Cgo enabled (`CGO_ENABLED=1`)
- GCC / Clang C compiler

### Build Shared Library

```bash
make build
# Or directly:
go build -buildmode=c-shared -o control-account.so main.go
```

This generates `control-account.so` (and `control-account.h`).

### Run Test Suite

```bash
make test
# Or with race detector:
go test -v -race ./...
```

---

## Installation in CLIProxyAPI

1. Compile the plugin for your target OS / architecture:
   ```bash
   make build
   ```
2. Copy `control-account.so` (or `control-account.dylib` on macOS) to your CLIProxyAPI `plugins/` directory.
3. Configure the plugin in CLIProxyAPI `config.yaml`:
   ```yaml
   plugins:
     - name: control-account
       path: plugins/control-account.so
   ```
4. Start / restart CLIProxyAPI and access the dashboard at:
   ```text
   http://localhost:8000/v0/resource/plugins/control-account/quota
   ```

---

## License

MIT License. See [LICENSE](LICENSE) for details.

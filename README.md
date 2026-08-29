# CLIProxyAPI Control Account Plugin (`v0.3.1`)

[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A standard C-ABI dynamic library plugin in Go for **[CLIProxyAPI](https://github.com/router-for-all/CLIProxyAPI)** that provides an embedded Quota Management Single-Page Application (SPA) dashboard with dark theme, real-time quota calculations, provider tab filtering, interactive profile/prefix modification, live throughput metrics, per-account activity sparklines, enabled/disabled account toggles, and default **Prefix (A-Z)** sorting.

---

## 📦 Option 1: Quick Install with Precompiled Binaries (No compilation needed)

No Go toolchain or C compiler is required. Download the pre-built `.so` file for your platform directly from [GitHub Releases](https://github.com/Clowraider/cli-control-account/releases):

| Platform / Architecture | Download Binary |
|---|---|
| **Linux amd64** (Ubuntu / Debian / Docker Standard) | `control-account-linux-amd64.so` |
| **Linux arm64** (Apple Silicon Docker / Raspberry / AWS Graviton) | `control-account-linux-arm64.so` |

### 1. Place the binary in your plugins folder
```bash
mkdir -p plugins
# Copy downloaded binary directly to your plugins folder
cp /path/to/control-account-linux-amd64.so plugins/control-account-linux-amd64.so
```

### 2. Configure CLIProxyAPI `config.yaml`
```yaml
plugins:
  enabled: true
  dir: /app/plugins
  configs:
    control-account-linux-amd64:
      enabled: true
```

### 3. Docker & Docker Compose Setup
Mount the `./plugins` folder into your container:

```yaml
services:
  cliproxy:
    image: router-for-all/cli-proxy-api:latest
    ports:
      - "8000:8000"
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./plugins:/app/plugins
```

Open the Quota Dashboard in your browser:
```text
http://localhost:8000/v0/resource/plugins/control-account-linux-amd64/quota
```

---

## 🛠️ Option 2: Local Compilation & Testing for Developers

If you want to modify the plugin and test it locally on your Ubuntu machine with Docker before pushing new versions:

### Method A: Compile Directly on Ubuntu Host
If you have Go 1.22+ installed on your Ubuntu host:
```bash
# 1. Compile the dynamic library (builds control-account-linux-amd64.so)
make build
# Or manually:
go build -buildmode=c-shared -o control-account-linux-amd64.so main.go

# 2. Copy the resulting .so directly to your Docker plugins directory
cp control-account-linux-amd64.so /ruta/a/tu/docker/plugins/control-account-linux-amd64.so
```

### Method B: Compile inside Docker (Zero host dependencies)
If you prefer not to install Go or GCC on your host, compile inside an ephemeral container that exactly matches Linux Docker ABI:
```bash
docker run --rm -v "$(pwd)":/src -w /src golang:1.22 \
  go build -buildmode=c-shared -o control-account-linux-amd64.so main.go
```

### Run Test Suite Locally
```bash
make test
# Or with race detector:
go test -v -race ./...
```

---

## 📁 Repository Structure

```text
cli-control-account/
├── .github/workflows/release.yml     # Automated Linux amd64 / arm64 CI/CD builds
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

## License

MIT License. See [LICENSE](LICENSE) for details.

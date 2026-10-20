# syntax=docker/dockerfile:1
FROM golang:1.22-bookworm AS builder

WORKDIR /src

# Pre-fetch dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

ARG VERSION=0.6.8
ENV CGO_ENABLED=1

# Run Go test suite
RUN go test ./...

# Compile dynamic shared library (.so)
RUN go build -buildmode=c-shared \
    -ldflags="-s -w -X control-account/internal/version.Version=${VERSION}" \
    -o control-account-linux-amd64.so main.go && \
    cp control-account-linux-amd64.so control-account.so

# Export stage: allows extracting binaries directly via BuildKit
# Example: docker build --target export --output . .
FROM scratch AS export
COPY --from=builder /src/control-account-linux-amd64.so /
COPY --from=builder /src/control-account-linux-amd64.h /
COPY --from=builder /src/control-account.so /

# Final lightweight distribution image
FROM debian:bookworm-slim
WORKDIR /dist
COPY --from=builder /src/control-account* /dist/
CMD ["ls", "-la", "/dist"]

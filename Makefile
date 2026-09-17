.PHONY: all build test clean lint

BINARY_NAME=control-account-linux-amd64.so
HEADER_NAME=control-account-linux-amd64.h
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.6.0-dev")
LDFLAGS ?= -s -w -X control-account/internal/version.Version=$(VERSION)

all: test build

build:
	go build -buildmode=c-shared -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) main.go

test:
	go test -v -race ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY_NAME) $(HEADER_NAME) control-account.so control-account.h

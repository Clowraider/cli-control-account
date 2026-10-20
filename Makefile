.PHONY: all build test clean lint docker-build

BINARY_NAME=control-account-linux-amd64.so
HEADER_NAME=control-account-linux-amd64.h
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo "0.6.8")
LDFLAGS ?= -s -w -X control-account/internal/version.Version=$(VERSION)

all: test build

build:
	go build -buildmode=c-shared -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) main.go

test:
	go test -v -race ./...

lint:
	go vet ./...

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t cli-control-account:$(VERSION) .
	docker create --name ccase-extract cli-control-account:$(VERSION)
	docker cp ccase-extract:/dist/$(BINARY_NAME) ./$(BINARY_NAME)
	docker cp ccase-extract:/dist/$(HEADER_NAME) ./$(HEADER_NAME)
	docker cp ccase-extract:/dist/control-account.so ./control-account.so
	docker rm ccase-extract

clean:
	rm -f $(BINARY_NAME) $(HEADER_NAME) control-account.so control-account.h

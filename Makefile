.PHONY: all build test clean lint

BINARY_NAME=control-account-linux-amd64.so
HEADER_NAME=control-account-linux-amd64.h

all: test build

build:
	go build -buildmode=c-shared -o $(BINARY_NAME) main.go

test:
	go test -v -race ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY_NAME) $(HEADER_NAME) control-account.so control-account.h

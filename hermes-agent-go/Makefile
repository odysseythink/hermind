# Makefile
.PHONY: build test lint clean

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/hermes ./cmd/hermes

test:
	go test -race -cover ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

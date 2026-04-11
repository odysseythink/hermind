# Makefile
.PHONY: build test lint clean

VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDDATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILDDATE)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/hermes ./cmd/hermes

test:
	go test -race -cover ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

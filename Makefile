.PHONY: build build-frontend build-server build-extension build-all dev test lint

FRONTEND_DIST := frontend/dist
GOFLAGS := -tags="fts5"

# Platform detection for LanceDB workaround on macOS
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
	# macOS: LanceDB pre-built library version mismatch (v0.1.2 lib vs v0.1.3 Go code).
	# Use nolancedb stub for local development builds. Production builds on Linux
	# should have matching libraries or build from source.
	GOFLAGS := -tags="fts5 nolancedb"
endif

build-frontend:
	cd frontend && yarn install && yarn build

build-extension:
	cd browser-extension && yarn install && yarn build

build-all: build-extension build-server

build-server: build-frontend
	cd backend && \
	rm -rf ./cmd/server/frontend/dist && \
	cp -r ../frontend/dist ./cmd/server/frontend/dist && \
	mv ./cmd/server/frontend/dist/_index.html ./cmd/server/frontend/dist/index.html && \
	go build $(GOFLAGS) -o ./bin/hermind ./cmd/server/

dev:
	go run $(GOFLAGS) ./cmd/server/ -logtostderr

test:
	go test -v ./...

lint:
	golangci-lint run ./...

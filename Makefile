# Makefile
.PHONY: build test lint clean release-snapshot

VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDDATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILDDATE)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/hermind ./cmd/hermind

test:
	go test -race -cover ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

release-snapshot:
	@command -v goreleaser >/dev/null 2>&1 || { \
	  echo "error: goreleaser not found. Install with one of:"; \
	  echo "  brew install goreleaser"; \
	  echo "  go install github.com/goreleaser/goreleaser/v2@latest"; \
	  exit 1; \
	}
	goreleaser release --snapshot --skip=publish --clean

.PHONY: web web-install web-dev web-clean web-test web-lint web-check

web-install:
	@command -v pnpm >/dev/null 2>&1 || { \
	  echo "error: pnpm not found. With Node 20+: corepack enable && corepack prepare pnpm@9 --activate"; \
	  exit 1; \
	}
	cd web && (pnpm install --frozen-lockfile || pnpm install)

web: web-install
	cd web && pnpm build
	find api/webroot -mindepth 1 -delete
	cp -R web/dist/. api/webroot/

web-dev: web-install
	@test -f web/.env.local || cp web/.env.example web/.env.local
	cd web && pnpm dev

web-clean:
	rm -rf web/node_modules web/dist

web-test: web-install
	cd web && pnpm test

web-lint: web-install
	cd web && pnpm lint

# web-check is the composite gate CI runs: install deps, type-check,
# unit tests, lint, build + sync, assert api/webroot/ is up to date.
# Run locally before pushing to avoid the CI round-trip.
web-check: web-install
	cd web && pnpm type-check
	cd web && pnpm test
	cd web && pnpm lint
	$(MAKE) web
	@if ! git diff --quiet api/webroot/; then \
	  echo "error: api/webroot/ is out of date. Run 'make web' and commit the result."; \
	  git diff --stat api/webroot/; \
	  exit 1; \
	fi

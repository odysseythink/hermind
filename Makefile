# Makefile
.PHONY: build test lint clean release-snapshot all

VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDDATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILDDATE)

build:
	go build -buildvcs=false -ldflags "$(LDFLAGS)" -o bin/hermind ./cmd/hermind

all: web build

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

# ─── web frontend ────────────────────────────────────────────────────────────
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

# ─── desktop-electron ────────────────────────────────────────────────────────
.PHONY: electron-install electron-dev electron-build electron-build-go electron-dist electron-dist-dir electron-clean

electron-install:
	@command -v npm >/dev/null 2>&1 || { \
	  echo "error: npm not found. Install Node.js first."; \
	  exit 1; \
	}
	cd desktop-electron && npm install

electron-build-go:
	cd desktop-electron && go build -ldflags="-s -w" -o resources/hermind-desktop.exe ../cmd/hermind

electron-build: electron-install
	cd desktop-electron && npm run build

electron-dev: electron-install
	cd desktop-electron && npm run dev

electron-preview: electron-install
	cd desktop-electron && npm run preview

electron-dist: electron-install
	cd desktop-electron && npm run dist

electron-dist-dir: electron-install
	cd desktop-electron && npm run dist:dir

electron-clean:
	rm -rf desktop-electron/node_modules desktop-electron/dist desktop-electron/out

# ─── desktop backend (Qt) ────────────────────────────────────────────────────
.PHONY: build-desktop-backend-macos build-desktop-backend-windows

build-desktop-backend-macos:
	go build -o desktop/resources/hermind-desktop-backend ./cmd/hermind

build-desktop-backend-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "-H=windowsgui" -o desktop/resources/hermind-desktop-backend.exe ./cmd/hermind

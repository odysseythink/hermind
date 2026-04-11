# Plan 9: Release / goreleaser / CI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Produce a reproducible cross-platform release pipeline for `hermes` using goreleaser, plus `version` / `--version` flag wiring, a VERSION file, a multi-arch GitHub Actions release workflow, and a simple install script.

**Architecture:**
- `.goreleaser.yml` at the repo root builds `hermes-agent-go/cmd/hermes` for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64. ldflags inject `main.Version`, `main.Commit`, `main.BuildDate`.
- `main.go` gains `Commit` and `BuildDate` variables in addition to `Version`.
- `cli.NewRootCmd` learns a `--version` flag, and `hermes version` prints all three fields.
- GitHub Actions workflow `.github/workflows/release.yml` triggers on tag push (`v*`), runs tests, then `goreleaser release`.
- `scripts/install.sh` at the repo root downloads the matching release archive from GitHub and extracts `hermes` into `/usr/local/bin`.
- `VERSION` file at the repo root carries the current version string (CI injects from the tag; local builds fall back to `git describe`).

**Tech Stack:** goreleaser v2, GitHub Actions, bash, existing Go toolchain. No new Go deps.

**Deliverable at end of plan:**
```
$ hermes version
hermes-agent v0.1.0
  commit:     abc1234
  built:      2026-04-11T12:00:00Z
  go:         go1.25.0
```

```
$ curl -sSL https://raw.githubusercontent.com/nousresearch/hermes-agent-go/main/scripts/install.sh | bash
Downloading hermes v0.1.0 (linux_amd64)...
Installed /usr/local/bin/hermes
```

**Non-goals for this plan (deferred):**
- Homebrew tap — later
- Debian / RPM packages — later
- Docker images — later (optional add-on)
- Signed releases (cosign / GPG) — later
- Auto-update of the client — deferred

**Plans 1-8 dependencies this plan touches:**
- `hermes-agent-go/cmd/hermes/main.go` — add Commit / BuildDate vars
- `hermes-agent-go/cli/root.go` — add --version flag + richer version output
- `hermes-agent-go/Makefile` — inject all three ldflags
- `.goreleaser.yml` — NEW, at repo root
- `.github/workflows/release.yml` — NEW
- `scripts/install.sh` — NEW
- `VERSION` — NEW

---

## File Structure

```
hermes-agent-rewrite/
├── .goreleaser.yml                      # NEW
├── VERSION                              # NEW
├── scripts/
│   └── install.sh                       # NEW
├── .github/workflows/
│   ├── test.yml                         # (existing)
│   └── release.yml                      # NEW
└── hermes-agent-go/
    ├── Makefile                         # MODIFIED
    ├── cmd/hermes/main.go               # MODIFIED
    └── cli/
        └── root.go                      # MODIFIED
```

---

## Task 1: Version metadata wiring

- [ ] **Step 1:** In `hermes-agent-go/cmd/hermes/main.go`, replace the single Version var with:

```go
// Injected at build time via ldflags.
var (
    Version   = "dev"
    Commit    = ""
    BuildDate = ""
)
```

And pass them through to `cli`:

```go
cli.Version = Version
cli.Commit = Commit
cli.BuildDate = BuildDate
```

- [ ] **Step 2:** In `hermes-agent-go/cli/root.go`, add the matching vars and update the `version` subcommand:

```go
var (
    Version   = "dev"
    Commit    = ""
    BuildDate = ""
)

func newVersionCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "version",
        Short: "Print version info",
        RunE: func(cmd *cobra.Command, args []string) error {
            fmt.Fprintf(cmd.OutOrStdout(),
                "hermes-agent %s\n  commit:     %s\n  built:      %s\n  go:         %s\n",
                Version, coalesce(Commit, "unknown"), coalesce(BuildDate, "unknown"), runtime.Version())
            return nil
        },
    }
}

func coalesce(a, b string) string {
    if a == "" {
        return b
    }
    return a
}
```

Also wire a root-level `--version` flag that prints the short form:
```go
root.Version = Version
root.SetVersionTemplate("hermes-agent {{.Version}}\n")
```

- [ ] **Step 3:** In `hermes-agent-go/Makefile`, update ldflags:

```makefile
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDDATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILDDATE)
```

- [ ] **Step 4:** Build and smoke-test:

```
make build
./bin/hermes version
```

Expected: version, commit, built, go version all printed.

- [ ] **Step 5:** Commit `feat(cli): expose commit and build date in version output`.

---

## Task 2: .goreleaser.yml

- [ ] **Step 1:** Create `.goreleaser.yml` at the **repo root** (not inside `hermes-agent-go/`):

```yaml
version: 2

project_name: hermes

before:
  hooks:
    - go -C hermes-agent-go mod tidy
    - go -C hermes-agent-go test ./...

builds:
  - id: hermes
    main: ./cmd/hermes
    binary: hermes
    dir: hermes-agent-go
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w
      - -X main.Version={{.Version}}
      - -X main.Commit={{.ShortCommit}}
      - -X main.BuildDate={{.Date}}

archives:
  - id: hermes
    builds: [hermes]
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- if eq .Arch "amd64" }}x86_64{{ else }}{{ .Arch }}{{ end }}
    files:
      - LICENSE*
      - README*
      - CHANGELOG*

checksum:
  name_template: 'checksums.txt'

snapshot:
  version_template: "{{ .Tag }}-SNAPSHOT-{{ .ShortCommit }}"

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^chore:'

release:
  draft: true
  prerelease: auto
```

- [ ] **Step 2:** Verify with a snapshot build:

```
goreleaser release --snapshot --clean
ls dist/hermes_*/hermes
dist/hermes_linux_amd64_v1/hermes version
```

Expected: a `hermes` binary in each per-arch directory under `dist/`.

- [ ] **Step 3:** Commit `feat(release): add .goreleaser.yml for multi-arch release builds`.

---

## Task 3: GitHub Actions release workflow

- [ ] **Step 1:** Create `.github/workflows/release.yml`:

```yaml
name: release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'
          cache: true

      - name: Run tests
        working-directory: hermes-agent-go
        run: go test -race -cover ./...

      - name: Run goreleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2:** Commit `ci: add release workflow wired to goreleaser on tag push`.

---

## Task 4: VERSION file + install.sh

- [ ] **Step 1:** Create `VERSION` at the repo root containing a single line:

```
0.1.0
```

(No leading `v` — tags will prepend it.)

- [ ] **Step 2:** Create `scripts/install.sh`:

```bash
#!/usr/bin/env bash
# install.sh — fetch the latest hermes release from GitHub and install
# it into /usr/local/bin. Override PREFIX or VERSION via env vars.
set -euo pipefail

REPO=${REPO:-nousresearch/hermes-agent-go}
PREFIX=${PREFIX:-/usr/local/bin}
VERSION=${VERSION:-$(curl -sSfL https://api.github.com/repos/$REPO/releases/latest | grep -oP '"tag_name": "\K[^"]+')}

if [[ -z "${VERSION:-}" ]]; then
  echo "error: could not determine latest release tag" >&2
  exit 1
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="x86_64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac

# Strip leading 'v' from tag for archive name.
VER_NO_V="${VERSION#v}"
ARCHIVE="hermes_${VER_NO_V}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Downloading hermes ${VERSION} (${OS}_${ARCH})..."
curl -sSfL "$URL" -o "$TMP/hermes.tar.gz"
tar -xzf "$TMP/hermes.tar.gz" -C "$TMP"

if [[ ! -x "$TMP/hermes" ]]; then
  echo "error: archive did not contain hermes binary" >&2
  exit 1
fi

install -m 0755 "$TMP/hermes" "$PREFIX/hermes"
echo "Installed $PREFIX/hermes"
"$PREFIX/hermes" version
```

- [ ] **Step 3:** `chmod +x scripts/install.sh`.
- [ ] **Step 4:** Commit `feat(release): add VERSION file and install.sh`.

---

## Verification Checklist

- [ ] `make build && ./bin/hermes version` prints version + commit + build date
- [ ] `./bin/hermes --version` short form prints correctly
- [ ] `goreleaser release --snapshot --clean` succeeds locally (requires goreleaser installed)
- [ ] `.github/workflows/release.yml` parses cleanly (`yamllint` or similar)
- [ ] `scripts/install.sh --help` equivalent runs without setting VERSION (no download; just parse)

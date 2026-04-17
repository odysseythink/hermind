# Design: `make release-snapshot` cross-platform build target

**Status:** Approved
**Date:** 2026-04-17

## Goal

Provide a single `make release-snapshot` command that produces cross-platform
hermind binaries — Linux, macOS, Windows × x86 (386), x86_64 (amd64), arm64 —
plus archives and checksums, without cutting a GitHub release or publishing
to the Homebrew tap. Enables local QA of release artifacts and ad-hoc
distribution to users on any supported OS/arch.

## Non-goals

- Publishing to GitHub releases, Homebrew tap, or any remote.
- FreeBSD, Linux/armv7, Linux/ppc64le, or other exotic targets. YAGNI for now.
- Replacing the existing release workflow (`.github/workflows/release.yml`).
- Updating `scripts/install.sh` to recognize `386` archives. (Out of scope —
  install.sh serves tagged releases; 386 support in install.sh is a follow-up
  if real user demand emerges.)
- Docker-based goreleaser invocation. `brew install goreleaser` is one line.

## Architecture

Reuse the existing `.goreleaser.yml` by invoking `goreleaser release --snapshot
--skip=publish`. This runs the full pipeline — build, archive, checksum —
but skips every remote side-effect (GitHub release creation, Homebrew tap PR,
announcements). The only new surface is a Makefile target with a preflight
check for the `goreleaser` binary.

No new shell scripts. No duplication of the build matrix in Make.

## Changes

### 1. `.goreleaser.yml` — expand matrix

Current matrix: linux/darwin/windows × amd64/arm64, skipping windows/arm64.
Expanded matrix: add `386` for linux/windows, unskip windows/arm64, exclude
darwin/386 (Apple dropped 32-bit long ago).

```yaml
builds:
  - id: hermind
    main: ./cmd/hermind
    binary: hermind
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
      - "386"
    ignore:
      - goos: darwin
        goarch: "386"
    ldflags:
      - -s -w
      - -X main.Version={{.Version}}
      - -X main.Commit={{.ShortCommit}}
      - -X main.BuildDate={{.Date}}
```

Result: 8 binaries (linux/{386,amd64,arm64}, darwin/{amd64,arm64},
windows/{386,amd64,arm64}).

### 2. `.goreleaser.yml` — archive naming

Current `name_template` maps `amd64 → x86_64`. Extend to map `386 → x86` so
the user-facing archive names match the "x86 / x86_64 / arm64" mental model:

```yaml
archives:
  - id: hermind
    ids: [hermind]
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_
      {{- if eq .Arch "amd64" }}x86_64
      {{- else if eq .Arch "386" }}x86
      {{- else }}{{ .Arch }}{{ end }}
    files:
      - LICENSE*
      - README*
      - CHANGELOG*
```

Result: `hermind_<version>_linux_x86.tar.gz`, `hermind_<version>_darwin_arm64.tar.gz`,
`hermind_<version>_windows_x86_64.zip`, etc.

### 3. `Makefile` — new `release-snapshot` target

Update the existing `.PHONY` declaration to include the new target, then append
the rule. Final Makefile head:

```makefile
.PHONY: build test lint clean release-snapshot

# ... existing VERSION/COMMIT/BUILDDATE/LDFLAGS block unchanged ...

release-snapshot:
	@command -v goreleaser >/dev/null 2>&1 || { \
	  echo "error: goreleaser not found. Install with one of:"; \
	  echo "  brew install goreleaser"; \
	  echo "  go install github.com/goreleaser/goreleaser/v2@latest"; \
	  exit 1; \
	}
	goreleaser release --snapshot --skip=publish --clean
```

### 4. `.gitignore` — ignore `dist/`

Add `dist/` to `.gitignore` so goreleaser snapshot artifacts never get
committed accidentally. Current `.gitignore` has `bin/` but not `dist/`.

## Output layout

```
dist/
  hermind_<version>_linux_x86.tar.gz
  hermind_<version>_linux_x86_64.tar.gz
  hermind_<version>_linux_arm64.tar.gz
  hermind_<version>_darwin_x86_64.tar.gz
  hermind_<version>_darwin_arm64.tar.gz
  hermind_<version>_windows_x86.zip
  hermind_<version>_windows_x86_64.zip
  hermind_<version>_windows_arm64.zip
  checksums.txt
  artifacts.json
  metadata.json
  config.yaml
  hermind_linux_386/hermind
  hermind_linux_amd64_v1/hermind
  ... (raw binary directories per target)
```

Snapshot `<version>` is `{{ .Tag }}-SNAPSHOT-{{ .ShortCommit }}` per the
existing `snapshot.version_template`. When no tag exists, goreleaser falls
back to `0.0.0-next-SNAPSHOT-<sha>`.

## Testing

- **Smoke**: `make release-snapshot` produces 8 archives + `checksums.txt` in
  `dist/`. Verify with `ls dist/*.tar.gz dist/*.zip dist/checksums.txt`.
- **Arch sanity**: `tar -xOzf dist/hermind_*_linux_arm64.tar.gz hermind | file -`
  reports `ELF 64-bit LSB ... ARM aarch64`.
- **Build validation is free**: goreleaser's `before.hooks` already runs
  `go mod tidy` + `go test ./...` before any build — so any cross-compile
  regression surfaces immediately.
- **CI**: no new CI work. Existing `test.yml` validates the default build;
  `release.yml` validates the full matrix on tag push.

## Error handling

| Scenario | Behavior |
|----------|----------|
| `goreleaser` not on PATH | Makefile prints two install hints and exits 1 |
| Any GOOS/GOARCH fails to compile | goreleaser fails fast, names the target |
| `go test ./...` fails in before-hook | goreleaser aborts before any build |
| Previous `dist/` exists | `--clean` wipes it first |

## Rollout

Single commit. No migration, no flag rollout. Any contributor with
`goreleaser` installed runs `make release-snapshot` and gets the full matrix.

## Open questions / future work

- Should `install.sh` learn to pick `x86` archives for 32-bit hosts? Defer
  until a user asks.
- Should snapshot archives be uploaded somewhere (S3, a private bucket) for
  QA distribution? Defer until need is concrete.
- Should we add `make release-snapshot-fast` that skips `before.hooks`
  (`go test ./...`) for quick iteration? Defer — current test suite is under
  10 seconds.
